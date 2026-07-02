/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package claudellm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ claudellmapi.Client
)

// parseRetryAfterSeconds converts an HTTP Retry-After delta-seconds header into a duration string (e.g. "5s"),
// or "" when the header is absent or not a non-negative integer.
func parseRetryAfterSeconds(h string) string {
	if h == "" {
		return ""
	}
	n, err := strconv.Atoi(h)
	if err != nil || n < 0 {
		return ""
	}
	return (time.Duration(n) * time.Second).String()
}

// errorDetailAttrs returns the raw upstream response as error attributes for caller introspection: the full headers
// (minus the egress proxy's own Microbus-* additions) and the body truncated to 16KB.
func errorDetailAttrs(h http.Header, body []byte) []any {
	headers := map[string]string{}
	for k, v := range h {
		if !strings.HasPrefix(k, "Microbus-") {
			headers[k] = strings.Join(v, ",")
		}
	}
	if len(body) > 16*1024 {
		body = body[:16*1024]
	}
	return []any{"headers", headers, "body", string(body)}
}

/*
Service implements the claude.llm.core microservice.

The Claude LLM provider microservice implements the Turn endpoint for the Anthropic Claude API.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	rateMu       sync.Mutex
	blockedUntil map[string]time.Time // model -> when its rate-limit window clears; preempts calls until then
	modelAliases map[string]string    // tier/family alias -> concrete model; nil falls back to claudeDefaultAliases. Phase 2 repopulates it from the models-list API.
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// rateLimitWait returns the time remaining until model's rate-limit window clears, or 0 if not currently limited.
func (svc *Service) rateLimitWait(model string) time.Duration {
	svc.rateMu.Lock()
	defer svc.rateMu.Unlock()
	wait := time.Until(svc.blockedUntil[model])
	if wait <= 0 {
		delete(svc.blockedUntil, model)
		return 0
	}
	return wait
}

// blockModel records that model is rate-limited for the next d, so subsequent calls are preempted until it clears.
func (svc *Service) blockModel(model string, d time.Duration) {
	svc.rateMu.Lock()
	defer svc.rateMu.Unlock()
	if svc.blockedUntil == nil {
		svc.blockedUntil = map[string]time.Time{}
	}
	svc.blockedUntil[model] = time.Now().Add(d)
}

// claudeDefaultAliases is the shipped alias table (capability tiers and Claude family names -> concrete models). The
// Anthropic API has no evergreen pointer - dateless IDs like claude-opus-4-8 are pinned snapshots, not floating
// aliases (the opus/sonnet aliases that auto-migrate are a Claude Code CLI convenience, not API model IDs) - so these
// are pinned to the current release and refreshed by the Phase 2 models-list lookup when svc.modelAliases is populated.
var claudeDefaultAliases = map[string]string{
	llmapi.ModelFast:    "claude-haiku-4-5",
	llmapi.ModelDefault: "claude-sonnet-5",
	llmapi.ModelSmart:   "claude-opus-4-8",
	"haiku":             "claude-haiku-4-5",
	"sonnet":            "claude-sonnet-5",
	"opus":              "claude-opus-4-8",
	"fable":             "claude-fable-5",
}

// resolveModel maps a tier/family alias to a concrete Claude model via svc.modelAliases (falling back to the shipped
// defaults while that member is nil), passes through any claude- prefixed name so a newly released model works before
// it is listed, and returns "" for anything it does not recognize.
func (svc *Service) resolveModel(model string) string {
	aliases := svc.modelAliases
	if aliases == nil {
		aliases = claudeDefaultAliases
	}
	if concrete, ok := aliases[model]; ok {
		return concrete
	}
	if strings.HasPrefix(model, "claude-") {
		return model
	}
	return ""
}

/*
OnResolveProvider is fired by llm.core to resolve which provider serves a given model alias or name. This provider answers ok=true when it holds an API key and its catalog recognizes the model.
*/
func (svc *Service) OnResolveProvider(ctx context.Context, model string) (ok bool, err error) { // MARKER: OnResolveProvider
	return svc.APIKey() != "" && svc.resolveModel(model) != "", nil
}

/*
Turn executes a single LLM turn using the Claude provider.
*/
func (svc *Service) Turn(ctx context.Context, model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (outItems []llmapi.Item, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}
	// Resolve a tier/family alias to a concrete model; an unrecognized string passes through unchanged so an
	// explicit-provider call to a brand-new model still reaches the API.
	if resolved := svc.resolveModel(model); resolved != "" {
		model = resolved
	}
	// Preempt while this model's account is in a known rate-limit window, without calling the provider.
	if wait := svc.rateLimitWait(model); wait > 0 {
		return nil, "", llmapi.Usage{}, errors.New("rate limited (preempted)", http.StatusTooManyRequests, "retryAfter", wait.String())
	}
	maxTokens := 4096
	if options != nil && options.MaxTokens > 0 {
		maxTokens = options.MaxTokens
	}

	// Convert the item log to Claude's two-level shape. A system message folds into systemMsg; an
	// assistant text item plus the tool_call items that follow it coalesce into one assistant message
	// (text then tool_use blocks); tool_result items coalesce into a single user message, as Anthropic
	// requires all tool_results for a turn to share one user message. Content is always a block array so
	// cache_control can attach to the last block uniformly. Reasoning items are dropped: extended
	// thinking is not enabled, so no thinking blocks are produced or replayed.
	var systemMsg string
	claudeMsgs := make([]claudeMessage, 0, len(items))
	for _, it := range items {
		switch it.Type() {
		case llmapi.ItemMessage:
			if it.Message == nil {
				continue
			}
			switch it.Message.Role {
			case "system":
				if systemMsg != "" {
					systemMsg += "\n\n"
				}
				systemMsg += it.Message.Content
			case "assistant":
				if it.Message.Content == "" {
					continue // an empty assistant text; following tool_call items open the message
				}
				claudeMsgs = append(claudeMsgs, claudeMessage{
					Role:    "assistant",
					Content: []claudeContentBlock{{Type: "text", Text: it.Message.Content}},
				})
			default:
				claudeMsgs = append(claudeMsgs, claudeMessage{
					Role:    it.Message.Role,
					Content: []claudeContentBlock{{Type: "text", Text: it.Message.Content}},
				})
			}
		case llmapi.ItemToolCall:
			if it.ToolCall == nil {
				continue
			}
			block := claudeContentBlock{
				Type:  "tool_use",
				ID:    it.ToolCall.ID,
				Name:  it.ToolCall.Name,
				Input: it.ToolCall.Arguments,
			}
			if n := len(claudeMsgs); n > 0 && claudeMsgs[n-1].Role == "assistant" {
				claudeMsgs[n-1].Content = append(claudeMsgs[n-1].Content, block)
			} else {
				claudeMsgs = append(claudeMsgs, claudeMessage{Role: "assistant", Content: []claudeContentBlock{block}})
			}
		case llmapi.ItemToolResult:
			if it.ToolResult == nil {
				continue
			}
			block := claudeContentBlock{
				Type:      "tool_result",
				ToolUseID: it.ToolResult.CallID,
				Content:   it.ToolResult.Output,
			}
			if n := len(claudeMsgs); n > 0 && claudeMsgs[n-1].Role == "user" {
				claudeMsgs[n-1].Content = append(claudeMsgs[n-1].Content, block)
			} else {
				claudeMsgs = append(claudeMsgs, claudeMessage{Role: "user", Content: []claudeContentBlock{block}})
			}
		}
	}

	// Sort tools by name to insulate the cache key from caller-side ordering variance.
	// Anthropic's prompt caching matches on byte-exact prefix, so a non-deterministic
	// tool array order would defeat the cache. Sorting here is local and cheap.
	sortedTools := make([]llmapi.Tool, len(tools))
	copy(sortedTools, tools)
	sort.Slice(sortedTools, func(i, j int) bool { return sortedTools[i].Name < sortedTools[j].Name })

	// Convert tools to Claude format.
	claudeTools := make([]claudeTool, 0, len(sortedTools))
	for _, t := range sortedTools {
		claudeTools = append(claudeTools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	// Build the system content block array (when non-empty).
	var systemBlocks []claudeContentBlock
	if systemMsg != "" {
		systemBlocks = []claudeContentBlock{{Type: "text", Text: systemMsg}}
	}

	// Apply prompt-caching breakpoints. Anthropic's cache is prefix-based and silently
	// declines to cache content below the per-model size threshold (~1024 tokens for
	// Sonnet/Opus, ~2048 for Haiku), so unconditionally setting these markers is a no-op
	// for small requests and a free win for large ones.
	//
	// Breakpoint 1: cache the stable preamble.
	//   - Last tool when tools are present  -> caches "system + tools"
	//   - Else last system block            -> caches "system"
	// Breakpoint 2: cache the conversation history.
	//   - Last content block of last message -> caches "system + tools + history"
	//
	// This uses 2 of the 4 breakpoints Anthropic allows per request, leaving headroom
	// for future per-call hints (e.g. an explicit caller-supplied breakpoint).
	ephemeral := &claudeCacheControl{Type: "ephemeral"}
	switch {
	case len(claudeTools) > 0:
		claudeTools[len(claudeTools)-1].CacheControl = ephemeral
	case len(systemBlocks) > 0:
		systemBlocks[len(systemBlocks)-1].CacheControl = ephemeral
	}
	if n := len(claudeMsgs); n > 0 {
		lastMsg := &claudeMsgs[n-1]
		if m := len(lastMsg.Content); m > 0 {
			lastMsg.Content[m-1].CacheControl = ephemeral
		}
	}

	// Build the request
	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  claudeMsgs,
		Tools:     claudeTools,
		System:    systemBlocks,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	// Build the HTTP request to the Claude API
	apiURL := svc.MessagesURL()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", svc.APIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send via HTTP egress proxy
	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		svc.LogWarn(ctx, "Claude API error response",
			"status", httpResp.StatusCode,
			"headers", httpResp.Header,
			"body", string(respBody),
		)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBody, &apiErr)
		message := apiErr.Error.Message
		if message == "" {
			message = "Claude API error"
		}
		props := []any{httpResp.StatusCode}
		// retryAfter is the retry signal to ChatLoop; Anthropic sends Retry-After only on retryable errors.
		if ra := parseRetryAfterSeconds(httpResp.Header.Get("Retry-After")); ra != "" {
			props = append(props, "retryAfter", ra)
			if d, perr := time.ParseDuration(ra); perr == nil {
				svc.blockModel(model, d)
			}
		}
		props = append(props, errorDetailAttrs(httpResp.Header, respBody)...)
		return nil, "", llmapi.Usage{}, errors.New(message, props...)
	}

	// Parse the response
	var claudeResp claudeResponse
	err = json.NewDecoder(httpResp.Body).Decode(&claudeResp)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	// Emit the assistant text (if any) as a single message item, then a tool_call item per tool_use
	// block. On replay these coalesce back into one assistant message (text then tool_use blocks).
	var text string
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	if text != "" {
		outItems = append(outItems, llmapi.NewMessage("assistant", text).AsItem())
	}
	for _, block := range claudeResp.Content {
		if block.Type == "tool_use" {
			outItems = append(outItems, llmapi.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			}.AsItem())
		}
	}

	usage = llmapi.Usage{
		InputTokens:      claudeResp.Usage.InputTokens,
		OutputTokens:     claudeResp.Usage.OutputTokens,
		CacheReadTokens:  claudeResp.Usage.CacheReadInputTokens,
		CacheWriteTokens: claudeResp.Usage.CacheCreationInputTokens,
		Model:            claudeResp.Model,
		Turns:            1,
	}

	stopReason = mapStopReason(claudeResp.StopReason)
	return outItems, stopReason, usage, nil
}

// mapStopReason translates Anthropic's stop_reason field to the normalized llmapi value.
// The Anthropic Messages API documents end_turn, tool_use, max_tokens, stop_sequence,
// refusal, and pause_turn. An empty or unrecognized value is reported as Unknown so callers
// surface it instead of treating it as a completion.
func mapStopReason(s string) string {
	switch s {
	case "end_turn":
		return llmapi.StopReasonEndTurn
	case "tool_use":
		return llmapi.StopReasonToolUse
	case "max_tokens":
		return llmapi.StopReasonMaxTokens
	case "stop_sequence":
		return llmapi.StopReasonStopSequence
	case "refusal":
		return llmapi.StopReasonRefusal
	case "pause_turn":
		return llmapi.StopReasonPauseTurn
	default:
		return llmapi.StopReasonUnknown
	}
}

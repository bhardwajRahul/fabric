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

/*
Service implements the claude.llm.core microservice.

The Claude LLM provider microservice implements the Turn endpoint for the Anthropic Claude API.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
Turn executes a single LLM turn using the Claude provider.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return "", nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}
	maxTokens := 4096
	if options != nil && options.MaxTokens > 0 {
		maxTokens = options.MaxTokens
	}

	// Extract system message
	var systemMsg string
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
			break
		}
	}

	// Convert messages to Claude format. Content is always emitted as a content-block array
	// (even for plain text), so cache_control can be attached to the last block uniformly.
	claudeMsgs := make([]claudeMessage, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			continue
		case "assistant":
			blocks := make([]claudeContentBlock, 0, 2)
			if msg.Content != "" {
				blocks = append(blocks, claudeContentBlock{Type: "text", Text: msg.Content})
			}
			if msg.ToolCalls != "" {
				var tcs []llmapi.ToolCall
				json.Unmarshal([]byte(msg.ToolCalls), &tcs)
				for _, tc := range tcs {
					blocks = append(blocks, claudeContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Arguments,
					})
				}
			}
			claudeMsgs = append(claudeMsgs, claudeMessage{Role: "assistant", Content: blocks})
		case "tool":
			// Anthropic requires ALL tool_result blocks paired with a single assistant turn's
			// tool_use blocks to live in ONE user message. Coalesce consecutive `tool` role
			// llmapi.Messages into the trailing user message we just emitted (if any), instead
			// of emitting a separate user message per tool_result.
			block := claudeContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}
			if n := len(claudeMsgs); n > 0 && claudeMsgs[n-1].Role == "user" {
				claudeMsgs[n-1].Content = append(claudeMsgs[n-1].Content, block)
			} else {
				claudeMsgs = append(claudeMsgs, claudeMessage{
					Role:    "user",
					Content: []claudeContentBlock{block},
				})
			}
		default:
			claudeMsgs = append(claudeMsgs, claudeMessage{
				Role:    msg.Role,
				Content: []claudeContentBlock{{Type: "text", Text: msg.Content}},
			})
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
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	// Build the HTTP request to the Claude API
	apiURL := svc.CompletionURL()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", svc.APIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send via HTTP egress proxy
	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
			RequestID string `json:"request_id"`
		}
		respBody, _ := io.ReadAll(httpResp.Body)
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return "", nil, "", llmapi.Usage{}, errors.New(
				apiErr.Error.Message,
				httpResp.StatusCode,
				"type", apiErr.Error.Type,
				"requestId", apiErr.RequestID,
			)
		}
		return "", nil, "", llmapi.Usage{}, errors.New("Claude API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse the response
	var claudeResp claudeResponse
	err = json.NewDecoder(httpResp.Body).Decode(&claudeResp)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, llmapi.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
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
	return content, toolCalls, stopReason, usage, nil
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

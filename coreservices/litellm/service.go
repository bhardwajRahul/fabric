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

package litellm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/litellm/litellmapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ litellmapi.Client
)

// litellmRetryAfter derives the wait to observe after a LiteLLM 429. It prefers an explicit Retry-After (delta-
// seconds), then LiteLLM's forwarded upstream header, then the OpenAI-style reset durations, and finally a 60s
// default so a throttle is always retryable.
func litellmRetryAfter(h http.Header) string {
	for _, k := range []string{"Retry-After", "llm_provider-retry-after"} {
		if n, err := strconv.Atoi(h.Get(k)); err == nil && n >= 0 {
			return (time.Duration(n) * time.Second).String()
		}
	}
	best := time.Duration(0)
	for _, k := range []string{"X-Ratelimit-Reset-Tokens", "X-Ratelimit-Reset-Requests"} {
		if d, err := time.ParseDuration(h.Get(k)); err == nil && d > best {
			best = d
		}
	}
	if best > 0 {
		return best.String()
	}
	return "60s"
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
Service implements the lite.llm.core microservice.

The LiteLLM provider microservice implements the Turn endpoint for a LiteLLM proxy, which exposes the OpenAI Responses wire format.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	rateMu        sync.Mutex
	blockedUntil  map[string]time.Time // model -> when its rate-limit window clears; preempts calls until then
	reasonMu      sync.Mutex
	reasoningSeen map[string]bool // model -> observed to reason (billed reasoning tokens); enables encrypted-reasoning replay
	aliasMu       sync.RWMutex
	fetchMu       sync.Mutex      // serializes the lazy models-list fetch so concurrent resolves make one call
	modelNames    map[string]bool // set of model_name entries the proxy exposes; populated lazily from the models-list API
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	// Warm the model_name set in the background so the first resolve rarely pays the fetch; startup never blocks.
	svc.Go(ctx, svc.RefreshModels)
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

/*
OnResolveProvider is fired by llm.core to resolve which provider serves a given model alias or name. The LiteLLM proxy fronts an operator-defined model_list, so this provider answers ok=true for any model_name the proxy exposes (fetched from its models-list API), including tiers like smart when the operator names an entry so.
*/
func (svc *Service) OnResolveProvider(ctx context.Context, model string) (ok bool, err error) { // MARKER: OnResolveProvider
	if svc.APIKey() == "" {
		return false, nil
	}
	resolved, err := svc.resolveModel(ctx, model)
	if err != nil {
		return false, errors.Trace(err)
	}
	return resolved != "", nil
}

// resolveModel returns the model as-is when the proxy exposes a model_name matching it, else "". LiteLLM's model_name
// is the alias - the proxy maps it to a real backend model - so there is no vendor prefix or family to interpret;
// membership in the lazily-fetched model_name set is the whole test. Turn does not call this: it passes the model
// straight to the proxy (the proxy is the authority on its model_list), so an explicit-provider call never depends on
// the fetch.
func (svc *Service) resolveModel(ctx context.Context, model string) (string, error) {
	if model == "" {
		return "", nil
	}
	err := svc.ensureAliases(ctx)
	if err != nil {
		return "", errors.Trace(err)
	}
	svc.aliasMu.RLock()
	held := svc.modelNames[model]
	svc.aliasMu.RUnlock()
	if held {
		return model, nil
	}
	return "", nil
}

// ensureAliases lazily populates the model_name set on first use, serialized by fetchMu; a failed fetch retries next call.
func (svc *Service) ensureAliases(ctx context.Context) error {
	svc.aliasMu.RLock()
	populated := svc.modelNames != nil
	svc.aliasMu.RUnlock()
	if populated {
		return nil
	}
	if svc.APIKey() == "" {
		return nil
	}
	svc.fetchMu.Lock()
	defer svc.fetchMu.Unlock()
	svc.aliasMu.RLock()
	populated = svc.modelNames != nil
	svc.aliasMu.RUnlock()
	if populated {
		return nil
	}
	return svc.RefreshModels(ctx)
}

/*
RefreshModels periodically repopulates the model_name set from the LiteLLM proxy's models-list API.
*/
func (svc *Service) RefreshModels(ctx context.Context) (err error) { // MARKER: RefreshModels
	if svc.APIKey() == "" {
		return nil
	}
	names, err := svc.listModelNames(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	svc.aliasMu.Lock()
	svc.modelNames = set
	svc.aliasMu.Unlock()
	return nil
}

// listModelNames fetches the operator's model_name entries from the LiteLLM proxy's OpenAI-compatible models-list API.
func (svc *Service) listModelNames(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequest("GET", svc.ModelsURL(), nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if svc.APIKey() != "" {
		httpReq.Header.Set("Authorization", "Bearer "+svc.APIKey())
	}
	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, errors.New("LiteLLM models list returned status %d", httpResp.StatusCode)
	}
	var listResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &listResp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	names := make([]string, 0, len(listResp.Data))
	for _, m := range listResp.Data {
		names = append(names, m.ID)
	}
	return names, nil
}

/*
Turn executes a single LLM turn through the LiteLLM proxy.
*/
func (svc *Service) Turn(ctx context.Context, model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (outItems []llmapi.Item, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}
	// Preempt while this model's account is in a known rate-limit window, without calling the proxy.
	if wait := svc.rateLimitWait(model); wait > 0 {
		return nil, "", llmapi.Usage{}, errors.New("rate limited (preempted)", http.StatusTooManyRequests, "retryAfter", wait.String())
	}

	// Convert the item log to Responses input items. System messages fold into the top-level
	// instructions; every other item maps one-to-one and stays in order, so a reasoning item keeps its
	// position immediately before the function_call it belongs to.
	var instructions string
	oaiInput := make([]openaiItem, 0, len(items))
	for _, it := range items {
		switch it.Type() {
		case llmapi.ItemMessage:
			if it.Message == nil {
				continue
			}
			if it.Message.Role == "system" {
				if instructions != "" {
					instructions += "\n\n"
				}
				instructions += it.Message.Content
				continue
			}
			partType := "input_text"
			if it.Message.Role == "assistant" {
				partType = "output_text"
			}
			oaiInput = append(oaiInput, openaiItem{
				Type:    "message",
				Role:    it.Message.Role,
				Content: []openaiContent{{Type: partType, Text: it.Message.Content}},
			})
		case llmapi.ItemToolCall:
			if it.ToolCall == nil {
				continue
			}
			oaiInput = append(oaiInput, openaiItem{
				Type:   "function_call",
				CallID: it.ToolCall.ID,
				Name:   it.ToolCall.Name,
				Args:   string(it.ToolCall.Arguments),
			})
		case llmapi.ItemToolResult:
			if it.ToolResult == nil {
				continue
			}
			oaiInput = append(oaiInput, openaiItem{
				Type:   "function_call_output",
				CallID: it.ToolResult.CallID,
				Output: it.ToolResult.Output,
			})
		case llmapi.ItemReasoning:
			if it.Reasoning == nil || it.Reasoning.EncryptedContent == "" {
				continue // only reasoning items carrying the encrypted payload can be replayed
			}
			// summary must be present on an echoed reasoning item (empty array is accepted; omitting
			// it is a 400), so initialize it non-nil.
			ri := openaiItem{
				Type:             "reasoning",
				ID:               it.Reasoning.ID,
				EncryptedContent: it.Reasoning.EncryptedContent,
				Summary:          []openaiSummary{},
			}
			for _, s := range it.Reasoning.Summary {
				ri.Summary = append(ri.Summary, openaiSummary{Type: "summary_text", Text: s})
			}
			oaiInput = append(oaiInput, ri)
		}
	}

	// Convert tools. Responses uses a flat function shape (no {type,function:{...}} wrapper).
	var oaiTools []openaiTool
	for _, t := range tools {
		oaiTools = append(oaiTools, openaiTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}

	reqBody := openaiRequest{
		Model:        model,
		Input:        oaiInput,
		Instructions: instructions,
		Tools:        oaiTools,
	}
	// Ask for the encrypted reasoning payload only once this model is observed to reason (a prior turn
	// billed reasoning tokens); a non-reasoning model rejects the request with a 400. This detection is
	// runtime, not a name list - important behind LiteLLM where the model string is operator-defined.
	if svc.knownReasoning(model) {
		reqBody.Include = []string{"reasoning.encrypted_content"}
	}
	if options != nil {
		reqBody.MaxOutputTokens = options.MaxTokens
		reqBody.Temperature = options.Temperature
		// Reasoning effort passes through verbatim; the summary surfaces reasoning for display. Only a
		// reasoning model accepts the field, so a non-reasoning model never receives it.
		if options.Effort != "" && svc.knownReasoning(model) {
			reqBody.Reasoning = &openaiReasoning{Effort: options.Effort, Summary: "auto"}
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	apiURL := svc.ResponsesURL()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+svc.APIKey())

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		svc.LogWarn(ctx, "LiteLLM API error response",
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
			message = "LiteLLM API error"
		}
		props := []any{httpResp.StatusCode}
		// retryAfter is the retry signal to ChatLoop. LiteLLM's retry decision is status-code-only and cannot detect
		// the poison case, so we treat any 429 as retryable and rely on CallLLM's bounded retry cap plus metrics to
		// contain a permanently-too-large request (the litellm exception - no per-upstream poison heuristic here).
		if httpResp.StatusCode == http.StatusTooManyRequests {
			ra := litellmRetryAfter(httpResp.Header)
			props = append(props, "retryAfter", ra)
			if d, perr := time.ParseDuration(ra); perr == nil {
				svc.blockModel(model, d)
			}
		}
		props = append(props, errorDetailAttrs(httpResp.Header, respBody)...)
		return nil, "", llmapi.Usage{}, errors.New(message, props...)
	}

	var oaiResp openaiResponse
	err = json.NewDecoder(httpResp.Body).Decode(&oaiResp)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	// Walk the output items in order, preserving reasoning/message/function_call sequencing so a later
	// turn can echo reasoning items back adjacent to their tool call.
	hasToolCalls := false
	for _, item := range oaiResp.Output {
		switch item.Type {
		case "reasoning":
			r := llmapi.Reasoning{ID: item.ID, EncryptedContent: item.EncryptedContent}
			for _, s := range item.Summary {
				r.Summary = append(r.Summary, s.Text)
			}
			outItems = append(outItems, r.AsItem())
		case "message":
			var text string
			for _, part := range item.Content {
				if part.Type == "output_text" {
					text += part.Text
				}
			}
			outItems = append(outItems, llmapi.NewMessage("assistant", text).AsItem())
		case "function_call":
			outItems = append(outItems, llmapi.ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: json.RawMessage(item.Args),
			}.AsItem())
			hasToolCalls = true
		}
	}
	stopReason = mapStopReason(oaiResp.Status, oaiResp.IncompleteDetails.Reason, hasToolCalls)

	cachedTokens := oaiResp.Usage.InputTokensDetails.CachedTokens
	usage = llmapi.Usage{
		InputTokens:     oaiResp.Usage.InputTokens - cachedTokens,
		OutputTokens:    oaiResp.Usage.OutputTokens,
		ReasoningTokens: oaiResp.Usage.OutputTokensDetails.ReasoningTokens,
		CacheReadTokens: cachedTokens,
		Model:           oaiResp.Model,
		Turns:           1,
	}
	if usage.InputTokens < 0 {
		usage.InputTokens = oaiResp.Usage.InputTokens
		usage.CacheReadTokens = 0
	}

	// Billed reasoning tokens prove this model reasons, so future turns request the encrypted reasoning
	// payload for replay. Runtime detection replaces a hardcoded reasoning-model name list.
	if usage.ReasoningTokens > 0 {
		svc.noteReasoning(model)
	}

	return outItems, stopReason, usage, nil
}

// knownReasoning reports whether this model has been observed to reason (a prior turn billed reasoning
// tokens). Only then does Turn request the encrypted reasoning payload, since a non-reasoning model
// rejects it. Per-replica; warms on the first response from a reasoning model. Runtime detection is
// important behind LiteLLM, where the operator-defined model string can't be pattern-matched.
func (svc *Service) knownReasoning(model string) bool {
	svc.reasonMu.Lock()
	defer svc.reasonMu.Unlock()
	return svc.reasoningSeen[model]
}

// noteReasoning records that a model reasons, learned at runtime from billed reasoning tokens.
func (svc *Service) noteReasoning(model string) {
	svc.reasonMu.Lock()
	defer svc.reasonMu.Unlock()
	if svc.reasoningSeen == nil {
		svc.reasoningSeen = map[string]bool{}
	}
	svc.reasoningSeen[model] = true
}

// mapStopReason derives the normalized llmapi stop reason from the Responses API result. The
// Responses API has no single finish_reason: a completed turn that emitted function_call items is a
// tool-use turn, an otherwise-completed turn is an end-turn, and an incomplete turn carries its cause
// in incomplete_details.reason (max_output_tokens, content_filter). Anything else is reported as
// Unknown so callers surface it instead of treating it as a completion.
func mapStopReason(status, incompleteReason string, hasToolCalls bool) string {
	switch status {
	case "completed":
		if hasToolCalls {
			return llmapi.StopReasonToolUse
		}
		return llmapi.StopReasonEndTurn
	case "incomplete":
		switch incompleteReason {
		case "max_output_tokens":
			return llmapi.StopReasonMaxTokens
		case "content_filter":
			return llmapi.StopReasonRefusal
		default:
			return llmapi.StopReasonUnknown
		}
	default:
		return llmapi.StopReasonUnknown
	}
}

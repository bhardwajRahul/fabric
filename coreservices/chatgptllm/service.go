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

package chatgptllm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/chatgptllm/chatgptllmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// openaiTPMRE extracts the "Limit N, Requested M" pair OpenAI reports on a token-rate error.
var openaiTPMRE = regexp.MustCompile(`Limit (\d+), Requested (\d+)`)

// openaiResetWait returns the longest positive X-Ratelimit-Reset-* duration (tokens or requests) as a duration
// string, or "" when neither is a positive duration. It is the wait a genuine throttle should observe.
func openaiResetWait(h http.Header) string {
	best := time.Duration(0)
	for _, k := range []string{"X-Ratelimit-Reset-Tokens", "X-Ratelimit-Reset-Requests"} {
		if d, err := time.ParseDuration(h.Get(k)); err == nil && d > best {
			best = d
		}
	}
	if best <= 0 {
		return ""
	}
	return best.String()
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

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ chatgptllmapi.Client
)

/*
Service implements the chatgpt.llm.core microservice.

The ChatGPT LLM provider microservice implements the Turn endpoint for the OpenAI Responses API.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	rateMu       sync.Mutex
	blockedUntil map[string]time.Time // model -> when its rate-limit window clears; preempts calls until then
	aliasMu      sync.RWMutex
	fetchMu      sync.Mutex        // serializes the lazy models-list fetch so concurrent resolves make one call
	modelAliases map[string]string // tier/family alias -> concrete model; populated lazily from the models-list API (no shipped defaults)
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	// Warm the alias table in the background so the first alias resolve rarely pays the fetch; startup never blocks.
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

// resolveModel resolves a tier/family/gpt-*-latest alias to a concrete OpenAI model, or "" if unknown. Concrete names
// pass through without the table; aliases consult the lazily-populated table.
func (svc *Service) resolveModel(ctx context.Context, model string) (string, error) {
	if model == "" {
		return "", nil
	}
	if isConcreteOpenAIModel(model) {
		return model, nil
	}
	err := svc.ensureAliases(ctx)
	if err != nil {
		return "", errors.Trace(err)
	}
	svc.aliasMu.RLock()
	concrete := svc.modelAliases[model]
	svc.aliasMu.RUnlock()
	return concrete, nil
}

// isConcreteOpenAIModel reports whether model is a concrete id (gpt- + a version digit, or an o-series o + a digit)
// rather than an alias; the digit distinguishes gpt-5.5 from the synthesized gpt-latest alias, and matches the o-series
// the same way isReasoningModel does so a bare o3/o1 is recognized.
func isConcreteOpenAIModel(model string) bool {
	if strings.HasPrefix(model, "gpt-") && len(model) > 4 && model[4] >= '0' && model[4] <= '9' {
		return true
	}
	return len(model) >= 2 && model[0] == 'o' && model[1] >= '1' && model[1] <= '9'
}

// ensureAliases lazily populates the alias table on first use, serialized by fetchMu; a failed fetch retries next call.
func (svc *Service) ensureAliases(ctx context.Context) error {
	svc.aliasMu.RLock()
	populated := svc.modelAliases != nil
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
	populated = svc.modelAliases != nil
	svc.aliasMu.RUnlock()
	if populated {
		return nil
	}
	return svc.RefreshModels(ctx)
}

/*
OnResolveProvider is fired by llm.core to resolve which provider serves a given model alias or name. This provider answers ok=true when it holds an API key and its catalog recognizes the model.
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

/*
Turn executes a single LLM turn using the ChatGPT provider.
*/
func (svc *Service) Turn(ctx context.Context, model string, items []llmapi.Item, tools []llmapi.Tool, options *llmapi.TurnOptions) (outItems []llmapi.Item, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}
	// Resolve a tier/family alias to a concrete model; an unrecognized string passes through unchanged so an
	// explicit-provider call to a brand-new model still reaches the API.
	resolved, err := svc.resolveModel(ctx, model)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	if resolved != "" {
		model = resolved
	}
	// Preempt while this model's account is in a known rate-limit window, without calling the provider.
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
	// Request the encrypted reasoning payload only for reasoning models; a non-reasoning model rejects it with a 400.
	if isReasoningModel(model) {
		reqBody.Include = []string{"reasoning.encrypted_content"}
	}
	if options != nil {
		reqBody.MaxOutputTokens = options.MaxTokens
		reqBody.Temperature = options.Temperature
		// Reasoning effort passes through verbatim; the summary surfaces reasoning for display. Only a
		// reasoning model accepts the field, so a non-reasoning model never receives it.
		if options.Effort != "" && isReasoningModel(model) {
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
		svc.LogWarn(ctx, "OpenAI API error response",
			"status", httpResp.StatusCode,
			"headers", httpResp.Header,
			"body", string(respBody),
		)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(respBody, &apiErr)
		message := apiErr.Error.Message
		if message == "" {
			message = "OpenAI API error"
		}
		props := []any{httpResp.StatusCode}
		// retryAfter is the retry signal to ChatLoop; only a genuine throttle (429 rate_limit_exceeded whose request
		// fits the limit) gets one. Poison (Requested > Limit) and insufficient_quota get none.
		if httpResp.StatusCode == http.StatusTooManyRequests && apiErr.Error.Code == "rate_limit_exceeded" {
			poison := false
			if m := openaiTPMRE.FindStringSubmatch(apiErr.Error.Message); m != nil {
				limit, _ := strconv.Atoi(m[1])
				requested, _ := strconv.Atoi(m[2])
				poison = requested > limit
			}
			if !poison {
				ra := openaiResetWait(httpResp.Header)
				if ra == "" {
					ra = "60s"
				}
				props = append(props, "retryAfter", ra)
				if d, perr := time.ParseDuration(ra); perr == nil {
					svc.blockModel(model, d)
				}
			}
		}
		props = append(props, errorDetailAttrs(httpResp.Header, respBody)...)
		return nil, "", llmapi.Usage{}, errors.New(message, props...)
	}

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	var oaiResp openaiResponse
	err = json.Unmarshal(rawBody, &oaiResp)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	// Walk the output items in order, preserving reasoning/message/function_call sequencing so a later
	// turn can echo reasoning items back adjacent to their tool call. Reasoning items carry the
	// encrypted payload (and any summary) needed for replay.
	hasContent := false
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
			if text != "" {
				hasContent = true
			}
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

	// No content and no tool calls is the smoking-gun shape behind downstream "LLM returned no
	// final assistant content" errors - typically an incomplete response from a content-filter
	// block, which otherwise surfaces only as an opaque unknown-stop-reason 502. Logged at
	// debug with the raw body so the cause is visible under MICROBUS_LOG_DEBUG=1.
	if !hasContent && !hasToolCalls {
		svc.LogDebug(ctx, "OpenAI returned no content and no tool calls",
			"model", model,
			"status", oaiResp.Status,
			"incompleteReason", oaiResp.IncompleteDetails.Reason,
			"stopReason", stopReason,
			"rawBody", string(rawBody),
		)
	}

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

	return outItems, stopReason, usage, nil
}

// openaiReasoningRE matches the gpt-<version> prefix, capturing the numeric version for the reasoning cutoff.
var openaiReasoningRE = regexp.MustCompile(`^gpt-([0-9]+(?:\.[0-9]+)?)`)

// isReasoningModel infers reasoning from the model name (the models-list API has no reasoning flag): the o-series
// reasons, and a gpt reasons unless its version is below 5, so a future/unversioned gpt defaults to reasoning. The
// gpt-*-chat variants are the exception - non-reasoning chat models even at version >= 5.
func isReasoningModel(model string) bool {
	if len(model) >= 2 && model[0] == 'o' && model[1] >= '1' && model[1] <= '9' {
		return true
	}
	if !strings.HasPrefix(model, "gpt-") {
		return false
	}
	if strings.Contains(model, "-chat") {
		return false
	}
	if m := openaiReasoningRE.FindStringSubmatch(model); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v < 5 {
			return false
		}
	}
	return true
}

// listedModel is the subset of a models-list entry the alias refresher needs.
type listedModel struct {
	id      string
	created int64 // unix seconds; the newest per variant wins
}

/*
RefreshModels periodically repopulates the model alias table from the OpenAI models-list API.
*/
func (svc *Service) RefreshModels(ctx context.Context) (err error) { // MARKER: RefreshModels
	if svc.APIKey() == "" {
		return nil
	}
	models, err := svc.listModels(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	aliases := buildChatgptAliases(models)
	svc.aliasMu.Lock()
	svc.modelAliases = aliases
	svc.aliasMu.Unlock()
	return nil
}

// listModels fetches the current model catalog from the OpenAI models-list API.
func (svc *Service) listModels(ctx context.Context) ([]listedModel, error) {
	httpReq, err := http.NewRequest("GET", svc.ModelsURL(), nil)
	if err != nil {
		return nil, errors.Trace(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+svc.APIKey())
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
		return nil, errors.New("OpenAI models list returned status %d", httpResp.StatusCode)
	}
	var listResp struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &listResp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	models := make([]listedModel, 0, len(listResp.Data))
	for _, m := range listResp.Data {
		models = append(models, listedModel{id: m.ID, created: m.Created})
	}
	return models, nil
}

// openaiVariantRE parses a gpt-<version>[-<variant>] id. The single all-letters suffix requirement excludes
// preview/beta/dated/multi-segment names, so a "latest" alias tracks stable, dateless flagships only.
var openaiVariantRE = regexp.MustCompile(`^gpt-([0-9]+(?:\.[0-9]+)?)(?:-([a-z]+))?$`)

// gptVariant is the running best model of a variant, ranked by version then recency.
type gptVariant struct {
	id      string
	version float64
	created int64
}

// buildChatgptAliases builds the alias table from the live list: each tier and its synthesized gpt-*-latest alias ->
// latest gpt-5+ model of the variant (highest version, tie-broken by created). base->default, pro->smart, mini->fast,
// nano->nano. Pre-5/chat/preview/non-tier models are excluded.
func buildChatgptAliases(models []listedModel) map[string]string {
	aliases := map[string]string{}
	best := map[string]gptVariant{} // variant ("", "pro", "mini", "nano") -> latest gpt-5+ model
	for _, m := range models {
		match := openaiVariantRE.FindStringSubmatch(m.id)
		if match == nil {
			continue
		}
		version, perr := strconv.ParseFloat(match[1], 64)
		if perr != nil || version < 5 {
			continue
		}
		variant := match[2]
		switch variant {
		case "", "pro", "mini", "nano":
		default:
			continue
		}
		cur, ok := best[variant]
		if !ok || version > cur.version || (version == cur.version && m.created > cur.created) {
			best[variant] = gptVariant{id: m.id, version: version, created: m.created}
		}
	}
	if m, ok := best[""]; ok {
		aliases[llmapi.ModelDefault] = m.id
		aliases["gpt-latest"] = m.id
	}
	if m, ok := best["pro"]; ok {
		aliases[llmapi.ModelSmart] = m.id
		aliases["gpt-pro-latest"] = m.id
	}
	if m, ok := best["mini"]; ok {
		aliases[llmapi.ModelFast] = m.id
		aliases["gpt-mini-latest"] = m.id
	}
	if m, ok := best["nano"]; ok {
		aliases["gpt-nano-latest"] = m.id
	}
	return aliases
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

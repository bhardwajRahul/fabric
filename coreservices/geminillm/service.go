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

package geminillm

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

	"github.com/microbus-io/fabric/coreservices/geminillm/geminillmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

// countInputTokens asks Gemini's countTokens endpoint for the exact input token count of a request, without
// generating. It runs on a separate, free quota, so it works even while generation is rate-limited.
func (svc *Service) countInputTokens(ctx context.Context, model string, reqBody geminiRequest) (int, error) {
	ctReq := geminiCountTokensReq{
		GenerateContentRequest: geminiCountInner{
			Model:             "models/" + model,
			Contents:          reqBody.Contents,
			Tools:             reqBody.Tools,
			SystemInstruction: reqBody.SystemInstruction,
		},
	}
	body, err := json.Marshal(ctReq)
	if err != nil {
		return 0, errors.Trace(err)
	}
	apiURL := svc.ModelsURL() + "/" + model + ":countTokens?key=" + svc.APIKey()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return 0, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return 0, errors.New("countTokens %d: %s", httpResp.StatusCode, string(respBody))
	}
	var ct geminiCountTokensResp
	err = json.Unmarshal(respBody, &ct)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return ct.TotalTokens, nil
}

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ geminillmapi.Client
)

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
Service implements the gemini.llm.core microservice.

The Gemini LLM provider microservice implements the Turn endpoint for the Google Gemini API.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	rateMu       sync.Mutex
	blockedUntil map[string]time.Time // model -> when its rate-limit window clears; preempts calls until then
	modelAliases map[string]string    // tier/family alias -> concrete model; nil falls back to geminiDefaultAliases. Phase 2 repopulates it from the models-list API.
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

// geminiDefaultAliases is the shipped alias table (capability tiers and Gemini family names -> concrete models).
// Gemini exposes a floating "-latest" pointer for every family, so all tiers self-update to the current model with
// no code change. Phase 2's models-list lookup repopulates svc.modelAliases if these ever need overriding.
var geminiDefaultAliases = map[string]string{
	llmapi.ModelFast:    "gemini-flash-lite-latest",
	llmapi.ModelDefault: "gemini-flash-latest",
	llmapi.ModelSmart:   "gemini-pro-latest",
	"flash":             "gemini-flash-latest",
	"pro":               "gemini-pro-latest",
	"flash-lite":        "gemini-flash-lite-latest",
}

// resolveModel maps a tier/family alias to a concrete Gemini model via svc.modelAliases (falling back to the shipped
// defaults while that member is nil), passes through any gemini- prefixed name so a newly released model works before
// it is listed, and returns "" for anything it does not recognize.
func (svc *Service) resolveModel(model string) string {
	aliases := svc.modelAliases
	if aliases == nil {
		aliases = geminiDefaultAliases
	}
	if concrete, ok := aliases[model]; ok {
		return concrete
	}
	if strings.HasPrefix(model, "gemini-") {
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
Turn executes a single LLM turn using the Gemini provider.
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

	// Convert the item log to Gemini's two-level shape. System messages hoist into the dedicated
	// systemInstruction field (blank-line joined). Reasoning items carry a thought signature that binds
	// to the item that follows them: pendingSig holds it until the next text/functionCall part consumes
	// it, re-gluing the signature Gemini requires on that exact part. An assistant text item and the
	// tool_call items after it coalesce into one model content; tool_result items coalesce into one user
	// content, matching Gemini's call/response pairing.
	var systemInstruction *geminiContent
	var systemParts []string
	contents := make([]geminiContent, 0, len(items))
	var pendingSig string
	for _, it := range items {
		switch it.Type() {
		case llmapi.ItemReasoning:
			if it.Reasoning != nil && it.Reasoning.Signature != "" {
				pendingSig = it.Reasoning.Signature
			}
		case llmapi.ItemMessage:
			if it.Message == nil {
				continue
			}
			switch it.Message.Role {
			case "system":
				if it.Message.Content != "" {
					systemParts = append(systemParts, it.Message.Content)
				}
			case "assistant":
				parts := make([]geminiPart, 0, 1+len(it.Message.Attachments))
				if it.Message.Content != "" {
					parts = append(parts, geminiPart{Text: it.Message.Content, ThoughtSignature: pendingSig})
					pendingSig = ""
				}
				parts = append(parts, attachmentsToParts(it.Message.Attachments)...)
				if len(parts) == 0 {
					continue
				}
				contents = append(contents, geminiContent{Role: "model", Parts: parts})
			default:
				parts := make([]geminiPart, 0, 1+len(it.Message.Attachments))
				if it.Message.Content != "" {
					parts = append(parts, geminiPart{Text: it.Message.Content})
				}
				parts = append(parts, attachmentsToParts(it.Message.Attachments)...)
				if len(parts) == 0 {
					// Skip silently rather than emit a content with no parts -- Gemini rejects
					// empty-parts contents with an INVALID_ARGUMENT error.
					continue
				}
				contents = append(contents, geminiContent{Role: it.Message.Role, Parts: parts})
			}
		case llmapi.ItemToolCall:
			if it.ToolCall == nil {
				continue
			}
			var args map[string]any
			json.Unmarshal(it.ToolCall.Arguments, &args)
			part := geminiPart{
				FunctionCall:     &geminiFuncCall{Name: it.ToolCall.Name, Args: args},
				ThoughtSignature: pendingSig,
			}
			pendingSig = ""
			if n := len(contents); n > 0 && contents[n-1].Role == "model" {
				contents[n-1].Parts = append(contents[n-1].Parts, part)
			} else {
				contents = append(contents, geminiContent{Role: "model", Parts: []geminiPart{part}})
			}
		case llmapi.ItemToolResult:
			if it.ToolResult == nil {
				continue
			}
			var resultMap map[string]any
			json.Unmarshal([]byte(it.ToolResult.Output), &resultMap)
			if resultMap == nil {
				resultMap = map[string]any{"result": it.ToolResult.Output}
			}
			// Gemini keys a functionResponse by name (functionCall has no id), so ToolResult.CallID,
			// which mirrors the ToolCall.ID we set to the function name, is the name here.
			part := geminiPart{
				FunctionResponse: &geminiFuncResp{
					Name:     it.ToolResult.CallID,
					Response: resultMap,
				},
			}
			if n := len(contents); n > 0 && contents[n-1].Role == "user" {
				contents[n-1].Parts = append(contents[n-1].Parts, part)
			} else {
				contents = append(contents, geminiContent{Role: "user", Parts: []geminiPart{part}})
			}
		}
	}
	if len(systemParts) > 0 {
		systemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}

	// Convert tools
	var gemTools []geminiToolDec
	if len(tools) > 0 {
		funcs := make([]geminiFunc, 0, len(tools))
		for _, t := range tools {
			funcs = append(funcs, geminiFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		gemTools = []geminiToolDec{{FunctionDeclarations: funcs}}
	}

	reqBody := geminiRequest{
		Contents:          contents,
		Tools:             gemTools,
		SystemInstruction: systemInstruction,
	}
	if options != nil && (options.MaxTokens > 0 || options.Temperature != 0) {
		reqBody.GenerationConfig = &geminiGenConfig{
			MaxOutputTokens: options.MaxTokens,
			Temperature:     options.Temperature,
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	apiURL := svc.ModelsURL() + "/" + model + ":generateContent?key=" + svc.APIKey()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		svc.LogWarn(ctx, "Gemini API error response",
			"status", httpResp.StatusCode,
			"headers", httpResp.Header,
			"body", string(rawBody),
		)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Details []struct {
					Type       string `json:"@type"`
					RetryDelay string `json:"retryDelay"` // google.rpc.RetryInfo
					Violations []struct {
						QuotaID    string `json:"quotaId"`
						QuotaValue string `json:"quotaValue"`
					} `json:"violations"` // google.rpc.QuotaFailure
				} `json:"details"`
			} `json:"error"`
		}
		json.Unmarshal(rawBody, &apiErr)
		message := apiErr.Error.Message
		if message == "" {
			message = "Gemini API error"
		}
		// Parse only what the retry decision needs; the full body and headers are attached raw below.
		var retryDelay, quotaID string
		var quotaValue int64
		for _, d := range apiErr.Error.Details {
			switch {
			case strings.HasSuffix(d.Type, "RetryInfo"):
				retryDelay = d.RetryDelay
			case strings.HasSuffix(d.Type, "QuotaFailure"):
				for _, v := range d.Violations {
					quotaID = v.QuotaID
					if qv, convErr := strconv.ParseInt(v.QuotaValue, 10, 64); convErr == nil {
						quotaValue = qv
					}
				}
			}
		}
		props := []any{httpResp.StatusCode}
		// retryAfter is the retry signal to ChatLoop; attached only for a retryable per-minute overflow (the request
		// fits the quota, not a PerDay quota), with a 1s margin for retryDelay's truncation. A token spans at least
		// one byte, so the marshaled request length is a hard ceiling on its token count: when even that ceiling fits
		// the quota the request cannot be poison and we skip the countTokens round-trip. Otherwise ask countTokens for
		// the exact count; if that call fails we cannot prove the request fits, so treat it as poison (fail closed).
		poison := false
		if quotaValue > 0 && int64(len(body)) >= quotaValue {
			tokens, ctErr := svc.countInputTokens(ctx, model, reqBody)
			if ctErr != nil {
				svc.LogWarn(ctx, "Gemini countTokens failed; treating as non-retryable", "error", ctErr)
				poison = true
			} else {
				poison = int64(tokens) > quotaValue
			}
		}
		perDay := strings.Contains(quotaID, "PerDay")
		if httpResp.StatusCode == http.StatusTooManyRequests && retryDelay != "" && !poison && !perDay {
			retryAfter := retryDelay
			if d, parseErr := time.ParseDuration(retryDelay); parseErr == nil {
				wait := d + time.Second // 1s margin for retryDelay's truncation
				retryAfter = wait.String()
				svc.blockModel(model, wait)
			}
			props = append(props, "retryAfter", retryAfter)
		}
		// 503 UNAVAILABLE is a transient overload (the model is momentarily swamped), not a permanent
		// failure. Gemini attaches no retryDelay to it, so we retry with a modest default wait, keeping
		// the provider symmetric with Anthropic's retryable 529 overload.
		if httpResp.StatusCode == http.StatusServiceUnavailable {
			wait := 5 * time.Second
			props = append(props, "retryAfter", wait.String())
			svc.blockModel(model, wait)
		}
		props = append(props, errorDetailAttrs(httpResp.Header, rawBody)...)
		return nil, "", llmapi.Usage{}, errors.New(message, props...)
	}

	var gemResp geminiResponse
	err = json.Unmarshal(rawBody, &gemResp)
	if err != nil {
		return nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	var mediaPartCount int
	hasContent := false
	hasToolCalls := false
	if len(gemResp.Candidates) > 0 {
		// Gemini can split a single assistant answer across several text parts. Accumulate consecutive
		// text into one assistant message (matching the other providers) so LastAssistantMessage sees
		// the whole answer and the replayed conversation stays role-alternating. The buffer is flushed
		// before any non-text item (tool call, reasoning signature, thought) and at the end.
		var text strings.Builder
		flushText := func() {
			if text.Len() > 0 {
				outItems = append(outItems, llmapi.NewMessage("assistant", text.String()).AsItem())
				hasContent = true
				text.Reset()
			}
		}
		for _, part := range gemResp.Candidates[0].Content.Parts {
			// Thought parts are the model's internal reasoning (Gemini 2.5 thinking models). Emit them
			// as reasoning items carrying the thinking text and signature, and keep them out of the
			// visible assistant content.
			if part.Thought {
				flushText()
				r := llmapi.Reasoning{Signature: part.ThoughtSignature}
				if part.Text != "" {
					r.Summary = []string{part.Text}
				}
				outItems = append(outItems, r.AsItem())
				continue
			}
			// A signature on a non-thought part binds to that part; emit it as a reasoning item
			// immediately before the item it belongs to so a later turn can re-glue it by position.
			if part.ThoughtSignature != "" {
				flushText()
				outItems = append(outItems, llmapi.Reasoning{Signature: part.ThoughtSignature}.AsItem())
			}
			if part.FunctionCall != nil {
				flushText()
				args, _ := json.Marshal(part.FunctionCall.Args)
				outItems = append(outItems, llmapi.ToolCall{
					ID:        part.FunctionCall.Name,
					Name:      part.FunctionCall.Name,
					Arguments: args,
				}.AsItem())
				hasToolCalls = true
			} else if part.Text != "" {
				text.WriteString(part.Text)
			}
			if part.InlineData != nil || part.FileData != nil {
				mediaPartCount++
			}
		}
		flushText()
		stopReason = mapFinishReason(gemResp.Candidates[0].FinishReason, hasToolCalls)
	}
	// Image-generation models (e.g. gemini-2.5-flash-image-preview) return inlineData/fileData
	// parts carrying produced media. Surfacing them through Turn would require widening the
	// cross-provider Turn signature, which is out of scope here. Log so the gap is visible
	// rather than swallow the bytes silently.
	if mediaPartCount > 0 {
		svc.LogDebug(ctx, "Gemini response contains multimodal parts not surfaced by Turn",
			"model", model,
			"mediaPartCount", mediaPartCount,
		)
	}

	// Diagnostic for "LLM returned no final assistant content" downstream errors: the loop
	// terminates at end_turn with empty content because the model genuinely produced no parts
	// (often a 2.5-Flash multi-turn quirk, or a safety/policy clip that doesn't surface as a
	// refusal finishReason). Logged at debug so it's available with MICROBUS_LOG_DEBUG=1 when
	// you need it, without pestering the normal log stream.
	if !hasContent && !hasToolCalls {
		finishReason := ""
		if len(gemResp.Candidates) > 0 {
			finishReason = gemResp.Candidates[0].FinishReason
		}
		svc.LogDebug(ctx, "Gemini returned no content and no tool calls",
			"model", model,
			"finishReason", finishReason,
			"stopReason", stopReason,
			"rawBody", string(rawBody),
		)
	}

	cachedTokens := gemResp.UsageMetadata.CachedContentTokenCount
	thoughtsTokens := gemResp.UsageMetadata.ThoughtsTokenCount
	usage = llmapi.Usage{
		InputTokens:     gemResp.UsageMetadata.PromptTokenCount - cachedTokens,
		OutputTokens:    gemResp.UsageMetadata.CandidatesTokenCount + thoughtsTokens,
		ThinkingTokens:  thoughtsTokens,
		CacheReadTokens: cachedTokens,
		Model:           gemResp.ModelVersion,
		Turns:           1,
	}
	if usage.InputTokens < 0 {
		usage.InputTokens = gemResp.UsageMetadata.PromptTokenCount
		usage.CacheReadTokens = 0
	}
	if usage.Model == "" {
		usage.Model = model
	}

	return outItems, stopReason, usage, nil
}

// attachmentsToParts converts llmapi.Attachment values into Gemini parts. An attachment
// with Data set produces an inlineData part (raw bytes; encoding/json handles base64);
// an attachment with URI set produces a fileData part (Gemini File API URI or public HTTPS).
// Attachments missing both are silently skipped - they carry no transportable payload.
func attachmentsToParts(atts []llmapi.Attachment) []geminiPart {
	if len(atts) == 0 {
		return nil
	}
	out := make([]geminiPart, 0, len(atts))
	for _, a := range atts {
		switch {
		case len(a.Data) > 0:
			out = append(out, geminiPart{
				InlineData: &geminiInlineData{MimeType: a.MediaType, Data: a.Data},
			})
		case a.URI != "":
			out = append(out, geminiPart{
				FileData: &geminiFileData{MimeType: a.MediaType, FileURI: a.URI},
			})
		}
	}
	return out
}

// mapFinishReason translates Gemini's finishReason field to the normalized llmapi value.
// Gemini's API documents STOP, MAX_TOKENS, SAFETY, RECITATION, BLOCKLIST, PROHIBITED_CONTENT,
// SPII, LANGUAGE, MALFORMED_FUNCTION_CALL, FINISH_REASON_UNSPECIFIED, and OTHER. STOP can
// represent either a natural end or a tool_call turn (Gemini doesn't carry a separate
// tool_use finish_reason); the hasToolCalls hint lets us disambiguate. SAFETY, RECITATION,
// BLOCKLIST, PROHIBITED_CONTENT, and SPII all map to refusal. Anything unrecognized maps to
// Unknown so callers surface it.
func mapFinishReason(s string, hasToolCalls bool) string {
	switch s {
	case "STOP":
		if hasToolCalls {
			return llmapi.StopReasonToolUse
		}
		return llmapi.StopReasonEndTurn
	case "MAX_TOKENS":
		return llmapi.StopReasonMaxTokens
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return llmapi.StopReasonRefusal
	default:
		return llmapi.StopReasonUnknown
	}
}

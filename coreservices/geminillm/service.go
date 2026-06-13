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
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/geminillm/geminillmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ geminillmapi.Client
)

/*
Service implements the gemini.llm.core microservice.

The Gemini LLM provider microservice implements the Turn endpoint for the Google Gemini API.
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
Turn executes a single LLM turn using the Gemini provider.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return "", nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}

	// Gemini has a dedicated `systemInstruction` request field; system role messages
	// belong there, not in the `contents` array (where mapping them to `user` would
	// create user/user adjacency at the start and isn't the contract Gemini documents).
	// Collect every system message in arrival order and concatenate them with blank lines.
	var systemInstruction *geminiContent
	var systemParts []string
	for _, msg := range messages {
		if msg.Role == "system" && msg.Content != "" {
			systemParts = append(systemParts, msg.Content)
		}
	}
	if len(systemParts) > 0 {
		systemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}

	// Convert messages
	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// Hoisted into systemInstruction above.
			continue
		case "assistant":
			if msg.ToolCalls != "" {
				var tcs []llmapi.ToolCall
				json.Unmarshal([]byte(msg.ToolCalls), &tcs)
				parts := make([]geminiPart, 0, len(tcs)+1+len(msg.Attachments))
				if msg.Content != "" {
					// The assistant's text part carries its own thoughtSignature (stored on
					// the parent Message). Echo it back so the model can resume its thinking.
					parts = append(parts, geminiPart{
						Text:             msg.Content,
						ThoughtSignature: msg.ThoughtSignature,
					})
				}
				parts = append(parts, attachmentsToParts(msg.Attachments)...)
				for _, tc := range tcs {
					var args map[string]any
					json.Unmarshal(tc.Arguments, &args)
					parts = append(parts, geminiPart{
						FunctionCall:     &geminiFuncCall{Name: tc.Name, Args: args},
						ThoughtSignature: tc.ThoughtSignature,
					})
				}
				contents = append(contents, geminiContent{Role: "model", Parts: parts})
			} else {
				parts := make([]geminiPart, 0, 1+len(msg.Attachments))
				if msg.Content != "" {
					parts = append(parts, geminiPart{
						Text:             msg.Content,
						ThoughtSignature: msg.ThoughtSignature,
					})
				}
				parts = append(parts, attachmentsToParts(msg.Attachments)...)
				contents = append(contents, geminiContent{Role: "model", Parts: parts})
			}
		case "tool":
			var resultMap map[string]any
			json.Unmarshal([]byte(msg.Content), &resultMap)
			if resultMap == nil {
				resultMap = map[string]any{"result": msg.Content}
			}
			// Gemini expects ALL functionResponses paired with a single model turn's
			// functionCalls to live in ONE user content turn. Coalesce consecutive `tool`
			// role llmapi.Messages into the trailing user content (if any) instead of
			// emitting a separate user content per result. Without this, a model turn that
			// requested N parallel function calls produces a follow-up sequence of N
			// separate user contents, breaking Gemini's call/response pairing.
			part := geminiPart{
				FunctionResponse: &geminiFuncResp{
					Name:     msg.ToolCallID,
					Response: resultMap,
				},
			}
			if n := len(contents); n > 0 && contents[n-1].Role == "user" {
				contents[n-1].Parts = append(contents[n-1].Parts, part)
			} else {
				contents = append(contents, geminiContent{
					Role:  "user",
					Parts: []geminiPart{part},
				})
			}
		default:
			parts := make([]geminiPart, 0, 1+len(msg.Attachments))
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			parts = append(parts, attachmentsToParts(msg.Attachments)...)
			if len(parts) == 0 {
				// Skip silently rather than emit a content with no parts -- Gemini rejects
				// empty-parts contents with an INVALID_ARGUMENT error.
				continue
			}
			contents = append(contents, geminiContent{
				Role:  msg.Role,
				Parts: parts,
			})
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
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	apiURL := svc.CompletionURL() + "/" + model + ":generateContent?key=" + svc.APIKey()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return "", nil, "", llmapi.Usage{}, errors.New("Gemini API error %d: %s", httpResp.StatusCode, string(rawBody))
	}

	var gemResp geminiResponse
	err = json.Unmarshal(rawBody, &gemResp)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	var mediaPartCount int
	if len(gemResp.Candidates) > 0 {
		for _, part := range gemResp.Candidates[0].Content.Parts {
			// Thought parts are the model's internal reasoning (Gemini 2.5 thinking models).
			// Skip them from the visible content; their thoughtSignature is still attached to
			// the corresponding non-thought parts further down the parts list.
			if part.Thought {
				continue
			}
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, llmapi.ToolCall{
					ID:               part.FunctionCall.Name,
					Name:             part.FunctionCall.Name,
					Arguments:        args,
					ThoughtSignature: part.ThoughtSignature,
				})
			}
			if part.InlineData != nil || part.FileData != nil {
				mediaPartCount++
			}
		}
		stopReason = mapFinishReason(gemResp.Candidates[0].FinishReason, len(toolCalls) > 0)
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
	if content == "" && len(toolCalls) == 0 {
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

	return content, toolCalls, stopReason, usage, nil
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

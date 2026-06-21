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

The ChatGPT LLM provider microservice implements the Turn endpoint for the OpenAI Chat Completions API.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
	rateMu       sync.Mutex
	blockedUntil map[string]time.Time // model -> when its rate-limit window clears; preempts calls until then
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

/*
Turn executes a single LLM turn using the ChatGPT provider.
*/
func (svc *Service) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	if model == "" {
		return "", nil, "", llmapi.Usage{}, errors.New("model is required", http.StatusBadRequest)
	}
	// Preempt while this model's account is in a known rate-limit window, without calling the provider.
	if wait := svc.rateLimitWait(model); wait > 0 {
		return "", nil, "", llmapi.Usage{}, errors.New("rate limited (preempted)", http.StatusTooManyRequests, "retryAfter", wait.String())
	}

	// Convert messages
	oaiMsgs := make([]openaiMessage, 0, len(messages))
	for _, msg := range messages {
		oaiMsg := openaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "assistant" && msg.ToolCalls != "" {
			var tcs []llmapi.ToolCall
			json.Unmarshal([]byte(msg.ToolCalls), &tcs)
			for _, tc := range tcs {
				oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openaiCallFunc{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				})
			}
		}
		if msg.Role == "tool" {
			oaiMsg.ToolCallID = msg.ToolCallID
		}
		oaiMsgs = append(oaiMsgs, oaiMsg)
	}

	// Convert tools
	var oaiTools []openaiTool
	for _, t := range tools {
		oaiTools = append(oaiTools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	reqBody := openaiRequest{
		Model:    model,
		Messages: oaiMsgs,
		Tools:    oaiTools,
	}
	if options != nil {
		reqBody.MaxTokens = options.MaxTokens
		reqBody.Temperature = options.Temperature
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	apiURL := svc.CompletionURL()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+svc.APIKey())

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
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
		return "", nil, "", llmapi.Usage{}, errors.New(message, props...)
	}

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}
	var oaiResp openaiResponse
	err = json.Unmarshal(rawBody, &oaiResp)
	if err != nil {
		return "", nil, "", llmapi.Usage{}, errors.Trace(err)
	}

	if len(oaiResp.Choices) > 0 {
		choice := oaiResp.Choices[0].Message
		content = choice.Content
		for _, tc := range choice.ToolCalls {
			toolCalls = append(toolCalls, llmapi.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
		stopReason = mapFinishReason(oaiResp.Choices[0].FinishReason)
	}

	// No content and no tool calls is the smoking-gun shape behind downstream "LLM returned no
	// final assistant content" errors - typically an empty choices array from a content-filter
	// prompt block, which otherwise surfaces only as an opaque unknown-stop-reason 502. Logged at
	// debug with the raw body so the cause is visible under MICROBUS_LOG_DEBUG=1.
	if content == "" && len(toolCalls) == 0 {
		finishReason := ""
		if len(oaiResp.Choices) > 0 {
			finishReason = oaiResp.Choices[0].FinishReason
		}
		svc.LogDebug(ctx, "OpenAI returned no content and no tool calls",
			"model", model,
			"finishReason", finishReason,
			"stopReason", stopReason,
			"rawBody", string(rawBody),
		)
	}

	cachedTokens := oaiResp.Usage.PromptTokensDetails.CachedTokens
	usage = llmapi.Usage{
		InputTokens:     oaiResp.Usage.PromptTokens - cachedTokens,
		OutputTokens:    oaiResp.Usage.CompletionTokens,
		CacheReadTokens: cachedTokens,
		Model:           oaiResp.Model,
		Turns:           1,
	}
	if usage.InputTokens < 0 {
		usage.InputTokens = oaiResp.Usage.PromptTokens
		usage.CacheReadTokens = 0
	}

	return content, toolCalls, stopReason, usage, nil
}

// mapFinishReason translates OpenAI's finish_reason field to the normalized llmapi value.
// The OpenAI Chat Completions API documents stop, tool_calls, length, content_filter, and
// function_call (legacy alias for tool_calls). An empty or unrecognized value is reported
// as Unknown so callers surface it instead of treating it as a completion.
func mapFinishReason(s string) string {
	switch s {
	case "stop":
		return llmapi.StopReasonEndTurn
	case "tool_calls", "function_call":
		return llmapi.StopReasonToolUse
	case "length":
		return llmapi.StopReasonMaxTokens
	case "content_filter":
		return llmapi.StopReasonRefusal
	default:
		return llmapi.StopReasonUnknown
	}
}

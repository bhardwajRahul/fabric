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
func (svc *Service) Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) { // MARKER: Turn
	// Convert messages
	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		case "assistant":
			if msg.ToolCalls != "" {
				var tcs []llmapi.ToolCall
				json.Unmarshal([]byte(msg.ToolCalls), &tcs)
				parts := make([]geminiPart, 0, len(tcs)+1)
				if msg.Content != "" {
					parts = append(parts, geminiPart{Text: msg.Content})
				}
				for _, tc := range tcs {
					var args map[string]any
					json.Unmarshal(tc.Arguments, &args)
					parts = append(parts, geminiPart{
						FunctionCall: &geminiFuncCall{Name: tc.Name, Args: args},
					})
				}
				contents = append(contents, geminiContent{Role: "model", Parts: parts})
			} else {
				contents = append(contents, geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: msg.Content}},
				})
			}
		case "tool":
			var resultMap map[string]any
			json.Unmarshal([]byte(msg.Content), &resultMap)
			if resultMap == nil {
				resultMap = map[string]any{"result": msg.Content}
			}
			contents = append(contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFuncResp{
						Name:     msg.ToolCallID,
						Response: resultMap,
					},
				}},
			})
		default:
			contents = append(contents, geminiContent{
				Role:  msg.Role,
				Parts: []geminiPart{{Text: msg.Content}},
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
		Contents: contents,
		Tools:    gemTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Trace(err)
	}

	apiURL := svc.BaseURL() + "/v1beta/models/" + svc.Model() + ":generateContent?key=" + svc.APIKey()
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, errors.New("Gemini API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var gemResp geminiResponse
	err = json.NewDecoder(httpResp.Body).Decode(&gemResp)
	if err != nil {
		return nil, errors.Trace(err)
	}

	completion = &llmapi.TurnCompletion{}
	if len(gemResp.Candidates) > 0 {
		for _, part := range gemResp.Candidates[0].Content.Parts {
			if part.Text != "" {
				completion.Content += part.Text
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				completion.ToolCalls = append(completion.ToolCalls, llmapi.ToolCall{
					ID:        part.FunctionCall.Name,
					Name:      part.FunctionCall.Name,
					Arguments: args,
				})
			}
		}
	}
	return completion, nil
}

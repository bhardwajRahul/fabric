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

package openaillm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/coreservices/openaillm/openaillmapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ openaillmapi.Client
)

/*
Service implements the openai.llm.core microservice.

The OpenAI LLM provider microservice implements the Turn endpoint for the OpenAI Chat Completions API.
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
Turn executes a single LLM turn using the OpenAI provider.
*/
func (svc *Service) Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) { // MARKER: Turn
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
		Model:    svc.Model(),
		Messages: oaiMsgs,
		Tools:    oaiTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Trace(err)
	}

	apiURL := svc.BaseURL() + "/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+svc.APIKey())

	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, errors.New("OpenAI API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	err = json.NewDecoder(httpResp.Body).Decode(&oaiResp)
	if err != nil {
		return nil, errors.Trace(err)
	}

	completion = &llmapi.TurnCompletion{}
	if len(oaiResp.Choices) > 0 {
		choice := oaiResp.Choices[0].Message
		completion.Content = choice.Content
		for _, tc := range choice.ToolCalls {
			completion.ToolCalls = append(completion.ToolCalls, llmapi.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	}
	return completion, nil
}

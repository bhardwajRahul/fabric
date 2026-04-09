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

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

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
func (svc *Service) Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) { // MARKER: Turn
	// Extract system message
	var systemMsg string
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
			break
		}
	}

	// Convert messages to Claude format
	claudeMsgs := make([]claudeMessage, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			continue
		case "assistant":
			if msg.ToolCalls != "" {
				var tcs []llmapi.ToolCall
				json.Unmarshal([]byte(msg.ToolCalls), &tcs)
				blocks := make([]claudeContentBlock, 0, len(tcs)+1)
				if msg.Content != "" {
					blocks = append(blocks, claudeContentBlock{Type: "text", Text: msg.Content})
				}
				for _, tc := range tcs {
					blocks = append(blocks, claudeContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Arguments,
					})
				}
				blocksJSON, _ := json.Marshal(blocks)
				claudeMsgs = append(claudeMsgs, claudeMessage{Role: "assistant", Content: blocksJSON})
			} else {
				content, _ := json.Marshal(msg.Content)
				claudeMsgs = append(claudeMsgs, claudeMessage{Role: "assistant", Content: content})
			}
		case "tool":
			block := claudeContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}
			blocksJSON, _ := json.Marshal([]claudeContentBlock{block})
			claudeMsgs = append(claudeMsgs, claudeMessage{Role: "user", Content: blocksJSON})
		default:
			content, _ := json.Marshal(msg.Content)
			claudeMsgs = append(claudeMsgs, claudeMessage{Role: msg.Role, Content: content})
		}
	}

	// Convert tools to Claude format
	claudeTools := make([]claudeTool, 0, len(tools))
	for _, t := range tools {
		claudeTools = append(claudeTools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	// Build the request
	reqBody := claudeRequest{
		Model:     svc.Model(),
		MaxTokens: 4096,
		Messages:  claudeMsgs,
		Tools:     claudeTools,
		System:    systemMsg,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Build the HTTP request to the Claude API
	apiURL := svc.BaseURL() + "/v1/messages"
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Trace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", svc.APIKey())
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send via HTTP egress proxy
	httpResp, err := httpegressapi.NewClient(svc).Do(ctx, httpReq)
	if err != nil {
		return nil, errors.Trace(err)
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
			return nil, errors.New(
				apiErr.Error.Message,
				httpResp.StatusCode,
				"type", apiErr.Error.Type,
				"requestId", apiErr.RequestID,
			)
		}
		return nil, errors.New("Claude API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse the response
	var claudeResp claudeResponse
	err = json.NewDecoder(httpResp.Body).Decode(&claudeResp)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// Convert to TurnCompletion
	completion = &llmapi.TurnCompletion{}
	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			completion.Content += block.Text
		case "tool_use":
			completion.ToolCalls = append(completion.ToolCalls, llmapi.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	return completion, nil
}

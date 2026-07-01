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
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/chatgptllm/chatgptllmapi"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *workflow.Flow
	_ testarossa.Asserter
	_ chatgptllmapi.Client
	_ llmapi.Message
	_ bufio.Reader
	_ strings.Builder
	_ httpegress.Mock
)

// MARKER: Turn

func TestChatGPTLLM_Turn(t *testing.T) { // MARKER: Turn
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	httpEgressMock := httpegress.NewMock()

	tester := connector.New("tester.client")
	client := chatgptllmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, httpEgressMock, tester)
	app.RunInTest(t)

	svc.SetAPIKey("test-key")

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/responses") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello from OpenAI!"}]}],"status":"completed","model":"gpt-4o","usage":{"input_tokens":10,"output_tokens":5}}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		content, toolCalls, stopReason, usage, err := client.Turn(ctx, "gpt-5.4-mini", messages, nil, nil)
		if assert.NoError(err) {
			assert.Expect(content, "Hello from OpenAI!")
			assert.Expect(len(toolCalls), 0)
			assert.Expect(stopReason, llmapi.StopReasonEndTurn)
			assert.Expect(usage.OutputTokens, 5)
			assert.Expect(usage.Turns, 1)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/responses") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"output":[{"type":"function_call","call_id":"call_1","name":"Arithmetic","arguments":"{\"x\":10,\"op\":\"-\",\"y\":3}"}],"status":"completed","model":"gpt-4o","usage":{"input_tokens":15,"output_tokens":8}}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 10 - 3?"}}
		_, toolCalls, stopReason, _, err := client.Turn(ctx, "gpt-5.4-mini", messages, nil, nil)
		if assert.NoError(err) {
			assert.Expect(len(toolCalls), 1)
			assert.Expect(toolCalls[0].Name, "Arithmetic")
			assert.Expect(stopReason, llmapi.StopReasonToolUse)
		}
	})
}

// TestChatGPTLLM_RealTurn exercises the real OpenAI Responses API end-to-end through the live HTTP
// egress proxy. It is skipped by default; drop an API key into realAPIKey and remove the skip to run
// it against production.
func TestChatGPTLLM_RealTurn(t *testing.T) {
	const realAPIKey = ""
	if realAPIKey == "" {
		t.Skip("set realAPIKey to run against the live OpenAI Responses API")
	}
	const realModel = "gpt-4o-mini"

	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	client := chatgptllmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, httpegress.NewService(), tester)
	app.RunInTest(t)

	svc.SetAPIKey(realAPIKey)

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{
			{Role: "system", Content: "You are a terse assistant. Answer in one word."},
			{Role: "user", Content: "What is the capital of France?"},
		}
		content, toolCalls, stopReason, usage, err := client.Turn(ctx, realModel, messages, nil, nil)
		if assert.NoError(err) {
			assert.True(strings.Contains(strings.ToLower(content), "paris"))
			assert.Expect(len(toolCalls), 0)
			assert.Expect(stopReason, llmapi.StopReasonEndTurn)
			assert.True(usage.InputTokens > 0)
			assert.True(usage.OutputTokens > 0)
			assert.Expect(usage.Turns, 1)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)

		tools := []llmapi.Tool{{
			Name:        "Arithmetic",
			Description: "Computes the result of an arithmetic operation.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"},"op":{"type":"string"},"y":{"type":"number"}},"required":["x","op","y"]}`),
		}}
		messages := []llmapi.Message{{Role: "user", Content: "Use the Arithmetic tool to compute 10 - 3."}}
		content, toolCalls, stopReason, _, err := client.Turn(ctx, realModel, messages, tools, nil)
		if assert.NoError(err) {
			assert.Expect(stopReason, llmapi.StopReasonToolUse)
			if assert.True(len(toolCalls) > 0) {
				assert.Expect(toolCalls[0].Name, "Arithmetic")
			}

			// Round-trip the tool result back to confirm function_call / function_call_output items
			// are threaded through correctly.
			messages = append(messages,
				llmapi.Message{Role: "assistant", Content: content, ToolCalls: toolCallsJSON(t, toolCalls)},
				llmapi.Message{Role: "tool", ToolCallID: toolCalls[0].ID, Content: `{"result":7}`},
			)
			content, _, stopReason, _, err = client.Turn(ctx, realModel, messages, tools, nil)
			if assert.NoError(err) {
				assert.Expect(stopReason, llmapi.StopReasonEndTurn)
				assert.True(strings.Contains(content, "7"))
			}
		}
	})
}

// toolCallsJSON serializes tool calls the way llm.core persists them on an assistant message.
func toolCallsJSON(t *testing.T, toolCalls []llmapi.ToolCall) string {
	b, err := json.Marshal(toolCalls)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

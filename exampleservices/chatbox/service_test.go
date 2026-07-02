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

package chatbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/exampleservices/calculator"
	"github.com/microbus-io/fabric/exampleservices/calculator/calculatorapi"
	"github.com/microbus-io/fabric/exampleservices/chatbox/chatboxapi"
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
	_ httpx.BodyReader
	_ pub.Option
	_ sub.Option
	_ *workflow.Flow
	_ testarossa.Asserter
	_ chatboxapi.Client
	_ json.Encoder
	_ llm.Service
	_ calculator.Service
)

// MARKER: Turn

// MARKER: Demo

func TestChatbox_Turn(t *testing.T) { // MARKER: Turn
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	client := chatboxapi.NewClient(tester)

	app := application.New()
	app.Add(svc, tester)
	app.RunInTest(t)

	t.Run("math_with_tool", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewMessage("user", "What is 6 times 7?").AsItem()}
		tools := []llmapi.Tool{{Name: "Arithmetic", Description: "Calculator", InputSchema: json.RawMessage(`{}`)}}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, tools, nil)
		if assert.NoError(err) {
			calls := llmapi.PendingToolCalls(out)
			assert.Expect(len(calls), 1)
			assert.Expect(calls[0].Name, "Arithmetic")
		}
	})

	t.Run("math_without_tool", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewMessage("user", "What is 6 times 7?").AsItem()}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "42")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewMessage("user", "Hello there").AsItem()}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "don't understand")
		}
	})

	t.Run("tool_result", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewToolResult("chatbox_1", `{"result":42}`).AsItem()}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "42")
		}
	})

	t.Run("empty_messages", func(t *testing.T) {
		assert := testarossa.For(t)

		out, _, _, err := client.Turn(ctx, "chatbox-default", nil, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "Chatbox demo")
		}
	})

	t.Run("division_by_zero", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewMessage("user", "What is 5 divided by 0?").AsItem()}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "zero")
		}
	})

	t.Run("unknown_role", func(t *testing.T) {
		assert := testarossa.For(t)

		items := []llmapi.Item{llmapi.NewMessage("system", "You are a bot.").AsItem()}
		out, _, _, err := client.Turn(ctx, "chatbox-default", items, nil, nil)
		if assert.NoError(err) {
			assert.Contains(llmapi.LastAssistantMessage(out), "don't understand")
		}
	})
}

func TestChatbox_Demo(t *testing.T) { // MARKER: Demo
	t.Parallel()
	ctx := t.Context()

	chatboxSvc := NewService()
	llmSvc := llm.NewService()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := chatboxapi.NewClient(tester)

	app := application.New()
	app.Add(chatboxSvc, llmSvc, calcSvc, tester)
	app.RunInTest(t)

	t.Run("get_renders_page", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Demo(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Chatbox Demo")
			}
		}
	})

	t.Run("post_processes_message", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Demo(ctx, "POST", "", url.Values{"message": []string{"What is 6 times 7?"}})
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Equal("application/json", res.Header.Get("Content-Type"))
				assert.Contains(body, "42")
			}
		}
	})

	t.Run("post_missing_message", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Demo(ctx, "POST", "", nil)
		if assert.NoError(err) {
			assert.Expect(res.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestChatbox_EndToEnd(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	chatboxSvc := NewService()
	llmSvc := llm.NewService()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(chatboxSvc, llmSvc, calcSvc, tester)
	app.RunInTest(t)

	t.Run("chat_with_calculator", func(t *testing.T) {
		assert := testarossa.For(t)
		items := []llmapi.Item{llmapi.NewMessage("user", "What is 6 times 7?").AsItem()}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, _, _, err := client.Chat(ctx, chatboxapi.Hostname, "chatbox-default", items, tools, nil)
		if assert.NoError(err) {
			assert.Expect(len(result) >= 2, true)
			last := result[len(result)-1]
			if assert.Expect(last.Type(), llmapi.ItemMessage) {
				assert.Expect(last.Message.Role, "assistant")
				assert.Contains(last.Message.Content, "42")
			}
		}
	})
}

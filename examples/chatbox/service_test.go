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

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
	"github.com/microbus-io/fabric/examples/chatbox/chatboxapi"
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

func TestChatbox_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("turn", func(t *testing.T) { // MARKER: Turn
		assert := testarossa.For(t)

		exampleMessages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		expectedContent := "Hi!"

		_, _, _, err := mock.Turn(ctx, "chatbox-default", exampleMessages, nil, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
			return expectedContent, nil, llmapi.Usage{Turns: 1, Model: model}, nil
		})
		content, _, _, err := mock.Turn(ctx, "chatbox-default", exampleMessages, nil, nil)
		assert.Expect(
			content, expectedContent,
			err, nil,
		)
	})

	t.Run("demo", func(t *testing.T) { // MARKER: Demo
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Demo(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockDemo(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Demo(w, r)
		assert.NoError(err)
	})
}

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

		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7?"}}
		tools := []llmapi.Tool{{Name: "Arithmetic", Description: "Calculator", InputSchema: json.RawMessage(`{}`)}}
		_, toolCalls, _, err := client.Turn(ctx, "chatbox-default", messages, tools, nil)
		if assert.NoError(err) {
			assert.Expect(len(toolCalls), 1)
			assert.Expect(toolCalls[0].Name, "Arithmetic")
		}
	})

	t.Run("math_without_tool", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7?"}}
		content, _, _, err := client.Turn(ctx, "chatbox-default", messages, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "42")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "Hello there"}}
		content, _, _, err := client.Turn(ctx, "chatbox-default", messages, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "don't understand")
		}
	})

	t.Run("tool_result", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "tool", Content: `{"result":42}`}}
		content, _, _, err := client.Turn(ctx, "chatbox-default", messages, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "42")
		}
	})

	t.Run("empty_messages", func(t *testing.T) {
		assert := testarossa.For(t)

		content, _, _, err := client.Turn(ctx, "chatbox-default", nil, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "Chatbox demo")
		}
	})

	t.Run("division_by_zero", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is 5 divided by 0?"}}
		content, _, _, err := client.Turn(ctx, "chatbox-default", messages, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "zero")
		}
	})

	t.Run("unknown_role", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "system", Content: "You are a bot."}}
		content, _, _, err := client.Turn(ctx, "chatbox-default", messages, nil, nil)
		if assert.NoError(err) {
			assert.Contains(content, "don't understand")
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
		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7?"}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, _, err := client.Chat(ctx, chatboxapi.Hostname, "chatbox-default", messages, tools, nil)
		if assert.NoError(err) {
			assert.Expect(len(result) >= 2, true)
			last := result[len(result)-1]
			assert.Expect(last.Role, "assistant")
			assert.Contains(last.Content, "42")
		}
	})
}

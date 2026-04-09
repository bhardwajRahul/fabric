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
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
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
		expectedCompletion := &llmapi.TurnCompletion{Content: "Hi!"}

		_, err := mock.Turn(ctx, exampleMessages, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) {
			return expectedCompletion, nil
		})
		result, err := mock.Turn(ctx, exampleMessages, nil)
		assert.Expect(
			result, expectedCompletion,
			err, nil,
		)
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
		completion, err := client.Turn(ctx, messages, tools)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Expect(len(completion.ToolCalls), 1)
			assert.Expect(completion.ToolCalls[0].Name, "Arithmetic")
		}
	})

	t.Run("math_without_tool", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7?"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Contains(completion.Content, "42")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "Hello there"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Contains(completion.Content, "don't understand")
		}
	})

	t.Run("tool_result", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "tool", Content: `{"result":42}`}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Contains(completion.Content, "42")
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

	// Point the LLM service at the chatbox provider
	llmSvc.SetProviderHostname("chatbox.example")

	t.Run("chat_with_calculator", func(t *testing.T) {
		assert := testarossa.For(t)
		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7?"}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, err := client.Chat(ctx, messages, tools)
		if assert.NoError(err) {
			assert.Expect(len(result) >= 2, true)
			last := result[len(result)-1]
			assert.Expect(last.Role, "assistant")
			assert.Contains(last.Content, "42")
		}
	})
}

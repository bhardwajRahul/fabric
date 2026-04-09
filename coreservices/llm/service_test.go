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

package llm

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

	"github.com/microbus-io/fabric/coreservices/chatgptllm"
	"github.com/microbus-io/fabric/coreservices/claudellm"
	"github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
	"github.com/microbus-io/fabric/coreservices/geminillm"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/examples/calculator"
	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
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
	_ llmapi.Client
	_ json.Encoder
	_ httpegress.Service
	_ claudellm.Mock
	_ calculator.Service
)

func TestLLM_Mock(t *testing.T) {
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

	t.Run("chat", func(t *testing.T) { // MARKER: Chat
		assert := testarossa.For(t)

		exampleMessages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		expectedMessages := []llmapi.Message{{Role: "assistant", Content: "Hi there!"}}

		_, err := mock.Chat(ctx, exampleMessages, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []string) (messagesOut []llmapi.Message, err error) {
			return expectedMessages, nil
		})
		result, err := mock.Chat(ctx, exampleMessages, nil)
		assert.Expect(
			result, expectedMessages,
			err, nil,
		)
	})
}

func TestLLM_Chat(t *testing.T) { // MARKER: Chat
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
			result, err := client.Chat(ctx, messages, nil)
			assert.Expect(
				result, expectedResult,
				err, nil,
			)
		})
	*/
}

func TestLLM_ChatLive(t *testing.T) {
	t.Skip("Manual test - set real API keys and comment this line to run")
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	svc.SetProviderHostname(claudellmapi.Hostname) // Select provider

	claude := claudellm.NewService()
	claude.SetAPIKey("") // sk-...
	claude.SetModel("claude-haiku-4-5")

	gemini := geminillm.NewService()
	gemini.SetAPIKey("") // AI...
	gemini.SetModel("gemini-2.0-flash")

	chatgpt := chatgptllm.NewService()
	chatgpt.SetAPIKey("") // sk-...
	chatgpt.SetModel("gpt-4")

	egressSvc := httpegress.NewService()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, claude, gemini, chatgpt, egressSvc, calcSvc, tester)
	app.RunInTest(t)

	t.Run("text_only", func(t *testing.T) {
		assert := testarossa.For(t)

		messages := []llmapi.Message{{Role: "user", Content: "What is the capital of France? Answer in one word."}}
		result, err := client.Chat(ctx, messages, nil)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})

	t.Run("with_tools", func(t *testing.T) {
		assert := testarossa.For(t)
		messages := []llmapi.Message{{Role: "user", Content: "What is 6 times 7? Use the calculator tool."}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, err := client.Chat(ctx, messages, tools)
		if assert.NoError(err) {
			t.Log("Response:", result)
			assert.Expect(len(result) > 0, true)
		}
	})
}

func TestLLM_ChatWithMockedProvider(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	providerMock := claudellm.NewMock()
	calcSvc := calculator.NewService()

	tester := connector.New("tester.client")
	client := llmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, providerMock, calcSvc, tester)
	app.RunInTest(t)

	svc.SetProviderHostname("claude.llm.core")

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		providerMock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) {
			return &llmapi.TurnCompletion{Content: "Hello from mocked provider!"}, nil
		})
		defer providerMock.MockTurn(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		result, err := client.Chat(ctx, messages, nil)
		if assert.NoError(err) {
			assert.Expect(len(result), 1)
			assert.Expect(result[0].Role, "assistant")
			assert.Expect(result[0].Content, "Hello from mocked provider!")
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)
		callCount := 0
		providerMock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) {
			callCount++
			if callCount == 1 {
				return &llmapi.TurnCompletion{
					ToolCalls: []llmapi.ToolCall{{
						ID:        "call_1",
						Name:      "Arithmetic",
						Arguments: json.RawMessage(`{"x":3,"op":"+","y":5}`),
					}},
				}, nil
			}
			return &llmapi.TurnCompletion{Content: "3 + 5 = 8"}, nil
		})
		defer providerMock.MockTurn(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
		tools := []string{calculatorapi.Arithmetic.URL()}
		result, err := client.Chat(ctx, messages, tools)
		if assert.NoError(err) {
			assert.Expect(len(result) >= 2, true)
			last := result[len(result)-1]
			assert.Expect(last.Role, "assistant")
			assert.Expect(last.Content, "3 + 5 = 8")
		}
	})
}

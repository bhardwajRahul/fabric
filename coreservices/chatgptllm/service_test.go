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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
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

func TestChatGPTLLM_Mock(t *testing.T) {
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
		expectedCompletion := &llmapi.TurnCompletion{Content: "Hi there!"}

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
			if strings.Contains(req.URL.String(), "/v1/chat/completions") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello from OpenAI!"}}]}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Expect(completion.Content, "Hello from OpenAI!")
			assert.Expect(len(completion.ToolCalls), 0)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/chat/completions") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Arithmetic","arguments":"{\"x\":10,\"op\":\"-\",\"y\":3}"}}]}}]}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 10 - 3?"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Expect(len(completion.ToolCalls), 1)
			assert.Expect(completion.ToolCalls[0].Name, "Arithmetic")
		}
	})
}

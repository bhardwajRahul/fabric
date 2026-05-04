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
	"bufio"
	"context"
	"encoding/json"
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

	"github.com/microbus-io/fabric/coreservices/claudellm/claudellmapi"
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
	_ claudellmapi.Client
	_ llmapi.Message
	_ json.Encoder
	_ strings.Builder
	_ bufio.Reader
	_ httpegress.Mock
)

func TestClaudeLLM_Mock(t *testing.T) {
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
		expectedContent := "Hi there!"

		_, _, _, err := mock.Turn(ctx, claudellmapi.ModelHaiku45, exampleMessages, nil, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
			return expectedContent, nil, llmapi.Usage{Turns: 1, Model: model}, nil
		})
		content, _, usage, err := mock.Turn(ctx, claudellmapi.ModelHaiku45, exampleMessages, nil, nil)
		assert.Expect(
			content, expectedContent,
			usage.Turns, 1,
			err, nil,
		)
	})
}

func TestClaudeLLM_Turn(t *testing.T) { // MARKER: Turn
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	httpEgressMock := httpegress.NewMock()

	tester := connector.New("tester.client")
	client := claudellmapi.NewClient(tester)

	app := application.New()
	app.Add(svc, httpEgressMock, tester)
	app.RunInTest(t)

	svc.SetAPIKey("test-key")

	t.Run("text_response", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/messages") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"content":[{"type":"text","text":"Hello from Claude!"}],"stop_reason":"end_turn","model":"claude-haiku-4-5","usage":{"input_tokens":10,"output_tokens":5}}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		content, toolCalls, usage, err := client.Turn(ctx, claudellmapi.ModelHaiku45, messages, nil, nil)
		if assert.NoError(err) {
			assert.Expect(content, "Hello from Claude!")
			assert.Expect(len(toolCalls), 0)
			assert.Expect(usage.InputTokens, 10)
			assert.Expect(usage.OutputTokens, 5)
			assert.Expect(usage.Turns, 1)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/messages") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"content":[{"type":"tool_use","id":"toolu_1","name":"Arithmetic","input":{"x":3,"op":"+","y":5}}],"stop_reason":"tool_use","model":"claude-haiku-4-5","usage":{"input_tokens":15,"output_tokens":8}}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
		_, toolCalls, _, err := client.Turn(ctx, claudellmapi.ModelHaiku45, messages, nil, nil)
		if assert.NoError(err) {
			assert.Expect(len(toolCalls), 1)
			assert.Expect(toolCalls[0].Name, "Arithmetic")
		}
	})
}

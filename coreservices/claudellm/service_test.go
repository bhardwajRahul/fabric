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
	"regexp"
	"strings"
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

func TestClaudeLLM_OpenAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")

	app := application.New()
	app.Add(svc, tester)
	app.RunInTest(t)

	rePort := regexp.MustCompile(`:([0-9]+)(/|$)`)
	routes := []string{
		// HINT: Insert routes of functional and web endpoints here
		claudellmapi.Turn.Route, // MARKER: Turn
	}
	for _, route := range routes {
		port := "443"
		matches := rePort.FindStringSubmatch(route)
		if len(matches) > 1 {
			port = matches[1]
		}
		t.Run("port_"+port, func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := tester.Request(
				ctx,
				pub.GET(httpx.JoinHostAndPath(claudellmapi.Hostname, ":"+port+"/openapi.json")),
			)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.Contains(body, "openapi")
					assert.Contains(body, route)
				}
			}
		})
	}
}

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
		expectedCompletion := &llmapi.TurnCompletion{Content: "Hi there!"}

		_, err := mock.Turn(ctx, exampleMessages, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) {
			return expectedCompletion, nil
		})
		result, err := mock.Turn(ctx, exampleMessages, nil)
		assert.Expect(
			result, expectedCompletion,
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
				w.Write([]byte(`{"content":[{"type":"text","text":"Hello from Claude!"}],"stop_reason":"end_turn"}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "Hello"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Expect(completion.Content, "Hello from Claude!")
			assert.Expect(len(completion.ToolCalls), 0)
		}
	})

	t.Run("tool_calling", func(t *testing.T) {
		assert := testarossa.For(t)

		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if strings.Contains(req.URL.String(), "/v1/messages") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"content":[{"type":"tool_use","id":"toolu_1","name":"Arithmetic","input":{"x":3,"op":"+","y":5}}],"stop_reason":"tool_use"}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		messages := []llmapi.Message{{Role: "user", Content: "What is 3 + 5?"}}
		completion, err := client.Turn(ctx, messages, nil)
		if assert.NoError(err) && assert.NotNil(completion) {
			assert.Expect(len(completion.ToolCalls), 1)
			assert.Expect(completion.ToolCalls[0].Name, "Arithmetic")
		}
	})
}

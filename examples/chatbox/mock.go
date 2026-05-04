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
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"

	"github.com/microbus-io/fabric/examples/chatbox/chatboxapi"
)

var (
	_ http.Request
	_ json.Encoder
	_ errors.TracedError
	_ httpx.BodyReader
	_ = utils.RandomIdentifier
	_ *workflow.Flow
	_ chatboxapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockTurn func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) // MARKER: Turn
	mockDemo func(w http.ResponseWriter, r *http.Request) (err error)                                                                                                                                          // MARKER: Demo
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockTurn sets up a mock handler for Turn.
func (svc *Mock) MockTurn(handler func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error)) *Mock { // MARKER: Turn
	svc.mockTurn = handler
	return svc
}

// Turn executes the mock handler.
func (svc *Mock) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) { // MARKER: Turn
	if svc.mockTurn == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	content, toolCalls, usage, err = svc.mockTurn(ctx, model, messages, tools, options)
	return content, toolCalls, usage, errors.Trace(err)
}

// MockDemo sets up a mock handler for Demo.
func (svc *Mock) MockDemo(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Demo
	svc.mockDemo = handler
	return svc
}

// Demo executes the mock handler.
func (svc *Mock) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	if svc.mockDemo == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockDemo(w, r)
	return errors.Trace(err)
}

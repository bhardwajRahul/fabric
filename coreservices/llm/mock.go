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
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
)

var (
	_ http.Request
	_ json.Encoder
	_ errors.TracedError
	_ httpx.BodyReader
	_ = utils.RandomIdentifier
	_ *workflow.Flow
	_ llmapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockChat            func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error)                                                                    // MARKER: Chat
	mockTurn            func(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error)                                                            // MARKER: Turn
	mockInitChat        func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (maxToolRounds int, toolRounds int, err error)                                          // MARKER: InitChat
	mockCallLLM         func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message) (llmContent string, pendingToolCalls any, err error)                                                         // MARKER: CallLLM
	mockProcessResponse func(ctx context.Context, flow *workflow.Flow, llmContent string, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, err error) // MARKER: ProcessResponse
	mockExecuteTool     func(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error)                                                                                    // MARKER: ExecuteTool
	mockChatLoop        func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error)                                               // MARKER: ChatLoop
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

// MockChat sets up a mock handler for Chat.
func (svc *Mock) MockChat(handler func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error)) *Mock { // MARKER: Chat
	svc.mockChat = handler
	return svc
}

// Chat executes the mock handler.
func (svc *Mock) Chat(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error) { // MARKER: Chat
	if svc.mockChat == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	messagesOut, err = svc.mockChat(ctx, messages, tools)
	return messagesOut, errors.Trace(err)
}

// MockTurn sets up a mock handler for Turn.
func (svc *Mock) MockTurn(handler func(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error)) *Mock { // MARKER: Turn
	svc.mockTurn = handler
	return svc
}

// Turn executes the mock handler.
func (svc *Mock) Turn(ctx context.Context, messages []llmapi.Message, tools []llmapi.ToolDef) (completion *llmapi.TurnCompletion, err error) { // MARKER: Turn
	if svc.mockTurn == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	completion, err = svc.mockTurn(ctx, messages, tools)
	return completion, errors.Trace(err)
}

// MockInitChat sets up a mock handler for InitChat.
func (svc *Mock) MockInitChat(handler func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (maxToolRounds int, toolRounds int, err error)) *Mock { // MARKER: InitChat
	svc.mockInitChat = handler
	return svc
}

// InitChat executes the mock handler.
func (svc *Mock) InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (maxToolRounds int, toolRounds int, err error) { // MARKER: InitChat
	if svc.mockInitChat == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	maxToolRounds, toolRounds, err = svc.mockInitChat(ctx, flow, messages, tools)
	return maxToolRounds, toolRounds, errors.Trace(err)
}

// MockCallLLM sets up a mock handler for CallLLM.
func (svc *Mock) MockCallLLM(handler func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message) (llmContent string, pendingToolCalls any, err error)) *Mock { // MARKER: CallLLM
	svc.mockCallLLM = handler
	return svc
}

// CallLLM executes the mock handler.
func (svc *Mock) CallLLM(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message) (llmContent string, pendingToolCalls any, err error) { // MARKER: CallLLM
	if svc.mockCallLLM == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	llmContent, pendingToolCalls, err = svc.mockCallLLM(ctx, flow, messages)
	return llmContent, pendingToolCalls, errors.Trace(err)
}

// MockProcessResponse sets up a mock handler for ProcessResponse.
func (svc *Mock) MockProcessResponse(handler func(ctx context.Context, flow *workflow.Flow, llmContent string, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, err error)) *Mock { // MARKER: ProcessResponse
	svc.mockProcessResponse = handler
	return svc
}

// ProcessResponse executes the mock handler.
func (svc *Mock) ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, err error) { // MARKER: ProcessResponse
	if svc.mockProcessResponse == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	messagesOut, toolsRequested, toolRoundsOut, err = svc.mockProcessResponse(ctx, flow, llmContent, toolRounds, maxToolRounds)
	return messagesOut, toolsRequested, toolRoundsOut, errors.Trace(err)
}

// MockExecuteTool sets up a mock handler for ExecuteTool.
func (svc *Mock) MockExecuteTool(handler func(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error)) *Mock { // MARKER: ExecuteTool
	svc.mockExecuteTool = handler
	return svc
}

// ExecuteTool executes the mock handler.
func (svc *Mock) ExecuteTool(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error) { // MARKER: ExecuteTool
	if svc.mockExecuteTool == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	toolExecutedOut, err = svc.mockExecuteTool(ctx, flow, toolExecuted)
	return toolExecutedOut, errors.Trace(err)
}

// MockChatLoop sets up a mock handler for the ChatLoop workflow.
func (svc *Mock) MockChatLoop(handler func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error)) *Mock { // MARKER: ChatLoop
	svc.mockChatLoop = handler
	return svc
}

// ChatLoop returns a trivial single-task graph when mocked.
func (svc *Mock) ChatLoop(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: ChatLoop
	if svc.mockChatLoop == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	// Return a synthetic single-task graph
	graph = workflow.NewGraph(llmapi.ChatLoop.URL())
	syntheticTask := "https://" + svc.Hostname() + ":428/mock-chat-loop"
	graph.AddTransition(syntheticTask, workflow.END)
	graph.DeclareInputs("*")
	graph.DeclareOutputs("*")
	// Subscribe the synthetic task handler
	svc.Subscribe("POST", ":428/mock-chat-loop", func(w http.ResponseWriter, r *http.Request) error {
		var flow workflow.Flow
		if err := json.NewDecoder(r.Body).Decode(&flow); err != nil {
			return errors.Trace(err)
		}
		snap := flow.Snapshot()
		var in llmapi.ChatLoopIn
		flow.ParseState(&in)
		messagesOut, err := svc.mockChatLoop(r.Context(), &flow, in.Messages, in.Tools)
		if err != nil {
			return err
		}
		out := llmapi.ChatLoopOut{MessagesOut: messagesOut}
		flow.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&flow)
	})
	return graph, nil
}

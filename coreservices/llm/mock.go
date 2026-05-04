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
	"github.com/microbus-io/fabric/sub"
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
	mockChat            func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error)         // MARKER: Chat
	mockTurn            func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error)         // MARKER: Turn
	mockInitChat        func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error)                                // MARKER: InitChat
	mockCallLLM         func(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message) (llmContent string, pendingToolCalls any, turnUsage llmapi.Usage, err error)                     // MARKER: CallLLM
	mockProcessResponse func(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) // MARKER: ProcessResponse
	mockExecuteTool     func(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error)                                                                                                       // MARKER: ExecuteTool
	mockChatLoopGraph   func(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                              // MARKER: ChatLoop
	unsubMockChatLoop   func() error                                                                                                                                                                                              // MARKER: ChatLoop
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
func (svc *Mock) MockChat(handler func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error)) *Mock { // MARKER: Chat
	svc.mockChat = handler
	return svc
}

// Chat executes the mock handler.
func (svc *Mock) Chat(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) { // MARKER: Chat
	if svc.mockChat == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	messagesOut, usage, err = svc.mockChat(ctx, provider, model, messages, toolURLs, options)
	return messagesOut, usage, errors.Trace(err)
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

// MockInitChat sets up a mock handler for InitChat.
func (svc *Mock) MockInitChat(handler func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error)) *Mock { // MARKER: InitChat
	svc.mockInitChat = handler
	return svc
}

// InitChat executes the mock handler.
func (svc *Mock) InitChat(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (maxToolRounds int, toolRounds int, err error) { // MARKER: InitChat
	if svc.mockInitChat == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	maxToolRounds, toolRounds, err = svc.mockInitChat(ctx, flow, messages, tools, options)
	return maxToolRounds, toolRounds, errors.Trace(err)
}

// MockCallLLM sets up a mock handler for CallLLM.
func (svc *Mock) MockCallLLM(handler func(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message) (llmContent string, pendingToolCalls any, turnUsage llmapi.Usage, err error)) *Mock { // MARKER: CallLLM
	svc.mockCallLLM = handler
	return svc
}

// CallLLM executes the mock handler.
func (svc *Mock) CallLLM(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message) (llmContent string, pendingToolCalls any, turnUsage llmapi.Usage, err error) { // MARKER: CallLLM
	if svc.mockCallLLM == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	llmContent, pendingToolCalls, turnUsage, err = svc.mockCallLLM(ctx, flow, provider, model, messages)
	return llmContent, pendingToolCalls, turnUsage, errors.Trace(err)
}

// MockProcessResponse sets up a mock handler for ProcessResponse.
func (svc *Mock) MockProcessResponse(handler func(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error)) *Mock { // MARKER: ProcessResponse
	svc.mockProcessResponse = handler
	return svc
}

// ProcessResponse executes the mock handler.
func (svc *Mock) ProcessResponse(ctx context.Context, flow *workflow.Flow, llmContent string, turnUsage llmapi.Usage, toolRounds int, maxToolRounds int) (messagesOut []llmapi.Message, toolsRequested bool, toolRoundsOut int, usageOut llmapi.Usage, err error) { // MARKER: ProcessResponse
	if svc.mockProcessResponse == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	messagesOut, toolsRequested, toolRoundsOut, usageOut, err = svc.mockProcessResponse(ctx, flow, llmContent, turnUsage, toolRounds, maxToolRounds)
	return messagesOut, toolsRequested, toolRoundsOut, usageOut, errors.Trace(err)
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
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockChatLoop(handler func(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error)) *Mock { // MARKER: ChatLoop
	if svc.unsubMockChatLoop != nil {
		svc.unsubMockChatLoop()
		svc.unsubMockChatLoop = nil
	}
	if handler == nil {
		svc.mockChatLoopGraph = nil
		return svc
	}
	mockName := "MockChatLoop" + utils.RandomIdentifier(8)
	mockRoute := ":428/mock-chat-loop-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockChatLoopGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(llmapi.ChatLoop.URL())
		g.AddTransition(mockTaskURL, workflow.END)
		g.DeclareInputs("*")
		g.DeclareOutputs("*")
		return g, nil
	}
	err := svc.Subscribe(mockName, func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			return errors.Trace(err)
		}
		snap := f.Snapshot()
		var in llmapi.ChatLoopIn
		f.ParseState(&in)
		messagesOut, usage, err := handler(r.Context(), &f, in.Provider, in.Model, in.Messages, in.Tools, in.Options)
		if err != nil {
			return err // No trace
		}
		out := llmapi.ChatLoopOut{MessagesOut: messagesOut, Usage: usage}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	},
		sub.At("POST", mockRoute),
		sub.Task(llmapi.ChatLoopIn{}, llmapi.ChatLoopOut{}),
	)
	if err == nil {
		svc.unsubMockChatLoop = func() error { return svc.Unsubscribe(mockName) }
	}
	return svc
}

// ChatLoop returns the workflow graph, or a mocked graph if MockChatLoop was called.
func (svc *Mock) ChatLoop(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: ChatLoop
	if svc.mockChatLoopGraph == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	graph, err = svc.mockChatLoopGraph(ctx)
	return graph, errors.Trace(err)
}

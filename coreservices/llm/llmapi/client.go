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

package llmapi

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"reflect"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
)

var (
	_ context.Context
	_ json.Encoder
	_ *http.Request
	_ *errors.TracedError
	_ *httpx.BodyReader
	_ = marshalRequest
	_ = marshalPublish
	_ = marshalFunction
	_ = marshalTask
	_ = marshalWorkflow
	_ workflow.Flow
)

// ChatResponse packs the response of Chat.
type ChatResponse multicastResponse // MARKER: Chat

// Get unpacks the return arguments of Chat.
func (_res *ChatResponse) Get() (messagesOut []Message, usage Usage, err error) { // MARKER: Chat
	_d := _res.data.(*ChatOut)
	return _d.MessagesOut, _d.Usage, _res.err
}

/*
Chat sends messages to an LLM with optional tools and returns the response messages.

The provider hostname picks which provider microservice handles the request (e.g.
"claude.llm.core"). The model identifier is provider-specific; use the typed constants
exported by each provider's *api package for compile-time checking.

Each entry in toolURLs is the canonical URL of a Microbus endpoint to expose to the LLM
(e.g. "https://calculator.example/arithmetic"). Only FeatureFunction, FeatureWeb, and
FeatureWorkflow endpoints can be exposed.

Input:
  - provider: provider is the hostname of the LLM provider microservice to use
  - model: model is the provider-specific model identifier
  - messages: messages is the conversation history to send to the LLM
  - toolURLs: toolURLs is the list of Microbus endpoint URLs exposed to the LLM
  - options: options configures tool-call rounds, max tokens, temperature (nil = defaults)

Output:
  - messagesOut: messagesOut is the full conversation including new messages produced by the LLM
  - usage: usage is the aggregate token consumption across all turns
*/
func (_c MulticastClient) Chat(ctx context.Context, provider string, model string, messages []Message, toolURLs []string, options *ChatOptions) iter.Seq[*ChatResponse] { // MARKER: Chat
	_in := ChatIn{Provider: provider, Model: model, Messages: messages, ToolURLs: toolURLs, Options: options}
	_out := ChatOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Chat.Method, Chat.Route, &_in, &_out)
	return func(yield func(*ChatResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ChatResponse)(_r)) {
				return
			}
		}
	}
}

/*
Chat sends messages to an LLM with optional tools and returns the response messages.

The provider hostname picks which provider microservice handles the request (e.g.
"claude.llm.core"). The model identifier is provider-specific; use the typed constants
exported by each provider's *api package for compile-time checking.

Each entry in toolURLs is the canonical URL of a Microbus endpoint to expose to the LLM
(e.g. "https://calculator.example/arithmetic"). Only FeatureFunction, FeatureWeb, and
FeatureWorkflow endpoints can be exposed.

Example:

	messages := []llmapi.Message{
		{Role: "user", Content: "What is 17 * 23, and what's the weather in Paris?"},
	}
	toolURLs := []string{
		calculatorapi.Arithmetic.URL(),
		dataapi.Fetch.URL(),
	}
	messagesOut, usage, err := llmapi.NewClient(svc).Chat(
		ctx,
		claudellmapi.Hostname, claudellmapi.ModelHaiku45,
		messages, toolURLs, nil,
	)

Input:
  - provider: provider is the hostname of the LLM provider microservice to use
  - model: model is the provider-specific model identifier
  - messages: messages is the conversation history to send to the LLM
  - toolURLs: toolURLs is the list of Microbus endpoint URLs exposed to the LLM
  - options: options configures tool-call rounds, max tokens, temperature (nil = defaults)

Output:
  - messagesOut: messagesOut is the full conversation including new messages produced by the LLM
  - usage: usage is the aggregate token consumption across all turns
*/
func (_c Client) Chat(ctx context.Context, provider string, model string, messages []Message, toolURLs []string, options *ChatOptions) (messagesOut []Message, usage Usage, err error) { // MARKER: Chat
	_in := ChatIn{Provider: provider, Model: model, Messages: messages, ToolURLs: toolURLs, Options: options}
	_out := ChatOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Chat.Method, Chat.Route, &_in, &_out)
	return _out.MessagesOut, _out.Usage, err // No trace
}

// TurnResponse packs the response of Turn.
type TurnResponse multicastResponse // MARKER: Turn

// Get unpacks the return arguments of Turn.
func (_res *TurnResponse) Get() (content string, toolCalls []ToolCall, usage Usage, err error) { // MARKER: Turn
	_d := _res.data.(*TurnOut)
	return _d.Content, _d.ToolCalls, _d.Usage, _res.err
}

/*
Turn executes a single LLM turn against a provider microservice.

This endpoint defines the interface that provider microservices (claudellm, chatgptllm,
geminillm) implement. Calling Turn on llm.core itself returns 501 Not Implemented; use
ForHost(<provider hostname>) to invoke a specific provider directly.

Input:
  - model: model is the provider-specific model identifier
  - messages: messages is the conversation history to send to the LLM
  - tools: tools is the resolved tool definitions with schemas
  - options: options configures max tokens and temperature (nil = provider defaults)

Output:
  - content: content is the LLM's text response, if any
  - toolCalls: toolCalls is the list of tool calls requested by the LLM
  - usage: usage is the token consumption for this single turn (Turns=1)
*/
func (_c MulticastClient) Turn(ctx context.Context, model string, messages []Message, tools []Tool, options *TurnOptions) iter.Seq[*TurnResponse] { // MARKER: Turn
	_in := TurnIn{Model: model, Messages: messages, Tools: tools, Options: options}
	_out := TurnOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Turn.Method, Turn.Route, &_in, &_out)
	return func(yield func(*TurnResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*TurnResponse)(_r)) {
				return
			}
		}
	}
}

/*
Turn executes a single LLM turn against a provider microservice.

This endpoint defines the interface that provider microservices (claudellm, chatgptllm,
geminillm) implement. Calling Turn on llm.core itself returns 501 Not Implemented; use
ForHost(<provider hostname>) to invoke a specific provider directly.

Input:
  - model: model is the provider-specific model identifier
  - messages: messages is the conversation history to send to the LLM
  - tools: tools is the resolved tool definitions with schemas
  - options: options configures max tokens and temperature (nil = provider defaults)

Output:
  - content: content is the LLM's text response, if any
  - toolCalls: toolCalls is the list of tool calls requested by the LLM
  - usage: usage is the token consumption for this single turn (Turns=1)
*/
func (_c Client) Turn(ctx context.Context, model string, messages []Message, tools []Tool, options *TurnOptions) (content string, toolCalls []ToolCall, usage Usage, err error) { // MARKER: Turn
	_in := TurnIn{Model: model, Messages: messages, Tools: tools, Options: options}
	_out := TurnOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Turn.Method, Turn.Route, &_in, &_out)
	return _out.Content, _out.ToolCalls, _out.Usage, err // No trace
}

// multicastResponse packs the response of a functional multicast.
type multicastResponse struct {
	data         any
	HTTPResponse *http.Response
	err          error
}

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c Client) ForHost(host string) Client {
	return Client{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c Client) WithOptions(opts ...pub.Option) Client {
	return Client{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// WorkflowRunner executes a workflow by name with initial state, blocking until termination.
// foremanapi.Client satisfies this interface.
type WorkflowRunner interface {
	Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error)
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests.
type Executor struct {
	svc     service.Publisher
	host    string
	opts    []pub.Option
	inFlow  *workflow.Flow
	outFlow *workflow.Flow
	runner  WorkflowRunner
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow, runner: _c.runner}
}

// WithWorkflowRunner returns a copy of the executor with a workflow runner for executing workflows.
// foremanapi.NewClient(svc) satisfies the WorkflowRunner interface.
func (_c Executor) WithWorkflowRunner(runner WorkflowRunner) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: runner}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{svc: caller, host: Hostname}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{svc: listener, host: Hostname}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{svc: c.svc, host: host, opts: c.opts}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{svc: c.svc, host: c.host, opts: append(c.opts, opts...)}
}

// marshalRequest supports functional endpoints.
func marshalRequest(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) (err error) {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return err // No trace
	}
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = httpx.ReadOutputPayload(httpRes, out)
	return errors.Trace(err)
}

// marshalPublish supports multicast functional endpoints.
func marshalPublish(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) iter.Seq[*multicastResponse] {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return func(yield func(*multicastResponse) bool) {
			yield(&multicastResponse{err: err})
		}
	}
	_queue := svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	return func(yield func(*multicastResponse) bool) {
		for qi := range _queue {
			httpResp, err := qi.Get()
			if err == nil {
				reflect.ValueOf(out).Elem().SetZero()
				err = httpx.ReadOutputPayload(httpResp, out)
			}
			if err != nil {
				if !yield(&multicastResponse{err: err, HTTPResponse: httpResp}) {
					return
				}
			} else {
				if !yield(&multicastResponse{data: out, HTTPResponse: httpResp}) {
					return
				}
			}
		}
	}
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// marshalTask supports task execution via the Executor.
func marshalTask(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any, inFlow *workflow.Flow, outFlow *workflow.Flow) (err error) {
	flow := inFlow
	if flow == nil {
		flow = workflow.NewFlow()
	}
	err = flow.SetState(in)
	if err != nil {
		return errors.Trace(err)
	}
	body, err := json.Marshal(flow)
	if err != nil {
		return errors.Trace(err)
	}
	u := httpx.JoinHostAndPath(host, route)
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Body(body),
		pub.ContentType("application/json"),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	flow = workflow.NewFlow()
	err = json.NewDecoder(httpRes.Body).Decode(flow)
	if err != nil {
		return errors.Trace(err)
	}
	if outFlow != nil {
		*outFlow = *flow
	}
	if out != nil {
		err = flow.ParseState(out)
		return errors.Trace(err)
	}
	return nil
}

// marshalWorkflow supports workflow execution via the Executor.
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, workflowURL string, in any, out any) (status string, err error) {
	status, state, err := runner.Run(ctx, workflowURL, in)
	if err != nil {
		return status, err // No trace
	}
	if out != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, out)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

/*
InitChat validates inputs, resolves tool schemas from OpenAPI, and stores them in flow state.
*/
func (_c Executor) InitChat(ctx context.Context, messages []Message, tools []Tool, options *ChatOptions) (err error) { // MARKER: InitChat
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, InitChat.Method, InitChat.Route, InitChatIn{
		Messages: messages,
		Tools:    tools,
		Options:  options,
	}, &InitChatOut{}, _c.inFlow, _c.outFlow)
	return err // No trace
}

/*
CallLLM sends the current messages and tools to the LLM provider.
*/
func (_c Executor) CallLLM(ctx context.Context, provider string, model string, messages []Message) (llmContent string, pendingToolCalls any, turnUsage Usage, err error) { // MARKER: CallLLM
	var out CallLLMOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, CallLLM.Method, CallLLM.Route, CallLLMIn{
		Provider: provider,
		Model:    model,
		Messages: messages,
	}, &out, _c.inFlow, _c.outFlow)
	return out.LLMContent, out.PendingToolCalls, out.TurnUsage, err // No trace
}

/*
ProcessResponse inspects the LLM response, accumulates usage, and routes to the next step.
*/
func (_c Executor) ProcessResponse(ctx context.Context, llmContent string, turnUsage Usage, toolRounds int, maxToolRounds int) (messagesOut []Message, toolsRequested bool, toolRoundsOut int, usageOut Usage, err error) { // MARKER: ProcessResponse
	var out ProcessResponseOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, ProcessResponse.Method, ProcessResponse.Route, ProcessResponseIn{
		LLMContent:    llmContent,
		TurnUsage:     turnUsage,
		ToolRounds:    toolRounds,
		MaxToolRounds: maxToolRounds,
	}, &out, _c.inFlow, _c.outFlow)
	return out.MessagesOut, out.ToolsRequested, out.ToolRoundsOut, out.UsageOut, err // No trace
}

/*
ExecuteTool executes a single tool call, identified by the currentTool forEach variable.
*/
func (_c Executor) ExecuteTool(ctx context.Context, toolExecuted bool) (toolExecutedOut bool, err error) { // MARKER: ExecuteTool
	var out ExecuteToolOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, ExecuteTool.Method, ExecuteTool.Route, ExecuteToolIn{
		ToolExecuted: toolExecuted,
	}, &out, _c.inFlow, _c.outFlow)
	return out.ToolExecutedOut, err // No trace
}

/*
ChatLoop creates and runs the ChatLoop workflow, blocking until termination.
*/
func (_c Executor) ChatLoop(ctx context.Context, provider string, model string, messages []Message, tools []Tool, options *ChatOptions) (messagesOut []Message, usage Usage, status string, err error) { // MARKER: ChatLoop
	if _c.runner == nil {
		return nil, Usage{}, "", errors.New("workflow runner not set, use WithWorkflowRunner")
	}
	var out ChatLoopOut
	status, err = marshalWorkflow(ctx, _c.runner, ChatLoop.URL(), ChatLoopIn{
		Provider: provider,
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Options:  options,
	}, &out)
	return out.MessagesOut, out.Usage, status, err
}

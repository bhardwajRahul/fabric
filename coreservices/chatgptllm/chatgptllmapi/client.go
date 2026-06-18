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

package chatgptllmapi

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"reflect"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
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
	_ workflow.Flow
)

// TurnResponse packs the response of Turn.
type TurnResponse multicastResponse // MARKER: Turn

// Get unpacks the return arguments of Turn.
func (_res *TurnResponse) Get() (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	_d := _res.data.(*TurnOut)
	return _d.Content, _d.ToolCalls, _d.StopReason, _d.Usage, _res.err
}

/*
Turn executes a single LLM turn using the ChatGPT provider.
*/
func (_c MulticastClient) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) iter.Seq[*TurnResponse] { // MARKER: Turn
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
Turn executes a single LLM turn using the ChatGPT provider.
*/
func (_c Client) Turn(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error) { // MARKER: Turn
	_in := TurnIn{Model: model, Messages: messages, Tools: tools, Options: options}
	_out := TurnOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Turn.Method, Turn.Route, &_in, &_out)
	return _out.Content, _out.ToolCalls, _out.StopReason, _out.Usage, err // No trace
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
	return Client{svc: _c.svc, host: host, opts: _c.opts}
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
// WorkflowRunner executes a workflow by name with initial state, blocking until termination.
// foremanapi.Client satisfies this interface.
type WorkflowRunner interface {
	Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (outcome *workflow.FlowOutcome, err error)
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests.
type Executor struct {
	svc         service.Publisher
	host        string
	opts        []pub.Option
	inFlow      *workflow.Flow
	outFlow     *workflow.Flow
	runner      WorkflowRunner
	flowOptions *workflow.FlowOptions
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner, flowOptions: _c.flowOptions}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner, flowOptions: _c.flowOptions}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow, runner: _c.runner, flowOptions: _c.flowOptions}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow, runner: _c.runner, flowOptions: _c.flowOptions}
}

// WithWorkflowRunner returns a copy of the executor with a workflow runner for executing workflows.
// foremanapi.NewClient(svc) satisfies the WorkflowRunner interface.
func (_c Executor) WithWorkflowRunner(runner WorkflowRunner) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: runner, flowOptions: _c.flowOptions}
}

// WithFlowOptions returns a copy of the executor that creates workflows with the given flow options
// (priority, fairness key and weight). It has no effect on task execution.
func (_c Executor) WithFlowOptions(flowOptions *workflow.FlowOptions) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner, flowOptions: flowOptions}
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
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, flowOptions *workflow.FlowOptions, workflowURL string, in any, out any) (status string, err error) {
	outcome, err := runner.Run(ctx, workflowURL, in, flowOptions)
	if err != nil {
		if outcome != nil {
			return outcome.Status, err // No trace
		}
		return "", err // No trace
	}
	if outcome == nil {
		return "", nil
	}
	status = outcome.Status
	if out != nil && outcome.State != nil {
		data, err := json.Marshal(outcome.State)
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

// Subflow runs this microservice's tasks and workflows as isolated child flows from INSIDE a task body.
// Unlike Executor (which carries a service.Publisher and is for tests), Subflow carries the calling
// task's *workflow.Flow: each method parks the calling step and re-enters it when the child terminates,
// returning (..., yield bool, err error). Only the explicit inputs cross into the child and only the
// explicit outputs cross back - the caller's flow state is NOT shared. This is the blessed way for one
// task to invoke another unit of work with state isolation; do not call Executor or foremanapi from a
// task body.
type Subflow struct {
	flow *workflow.Flow
}

// NewSubflow creates a subflow client bound to the calling task's flow carrier.
func NewSubflow(flow *workflow.Flow) Subflow {
	return Subflow{flow: flow}
}

// marshalSubflow runs a child flow via the flow carrier and returns the parker's yield. A non-empty
// taskName selects flow.Subtask (the engine synthesizes a single-task graph named taskName around url);
// an empty taskName selects flow.Subgraph (the host loads the graph by url) - mirroring the engine's
// taskName-presence discriminator. in is marshaled to the child's input; the child's final_state is
// unmarshaled into out.
func marshalSubflow(flow *workflow.Flow, taskName, url string, in any, out any) (yield bool, err error) {
	if flow == nil {
		return false, errors.New("Subflow requires a flow carrier (call from a task body)")
	}
	if taskName != "" {
		return flow.Subtask(taskName, url, in, out)
	}
	return flow.Subgraph(url, in, out)
}

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

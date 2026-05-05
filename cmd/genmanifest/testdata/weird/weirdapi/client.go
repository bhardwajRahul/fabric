package weirdapi

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

// PlainResponse packs the response of Plain.
type PlainResponse multicastResponse // MARKER: Plain

// Get unpacks the return arguments of Plain.
func (_res *PlainResponse) Get() (result string, err error) { // MARKER: Plain
	_d := _res.data.(*PlainOut)
	return _d.Result, _res.err
}

// Plain is a baseline function on the safe trust segment.
func (_c MulticastClient) Plain(ctx context.Context) iter.Seq[*PlainResponse] { // MARKER: Plain
	_in := PlainIn{}
	_out := PlainOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Plain.Method, Plain.Route, &_in, &_out)
	return func(yield func(*PlainResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*PlainResponse)(_r)) {
				return
			}
		}
	}
}

// Plain is a baseline function on the safe trust segment.
func (_c Client) Plain(ctx context.Context) (result string, err error) { // MARKER: Plain
	_in := PlainIn{}
	_out := PlainOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Plain.Method, Plain.Route, &_in, &_out)
	return _out.Result, err // No trace
}

// PathArg accepts a path argument.
func (_c Client) PathArg(ctx context.Context, id string) (err error) { // MARKER: PathArg
	_in := PathArgIn{ID: id}
	_out := PathArgOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, PathArg.Method, PathArg.Route, &_in, &_out)
	return err // No trace
}

// GreedyArg accepts a greedy tail path argument.
func (_c Client) GreedyArg(ctx context.Context, tail string) (err error) { // MARKER: GreedyArg
	_in := GreedyArgIn{Tail: tail}
	_out := GreedyArgOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, GreedyArg.Method, GreedyArg.Route, &_in, &_out)
	return err // No trace
}

// PeriodInPath has a period inside its path segment.
func (_c Client) PeriodInPath(ctx context.Context) (err error) { // MARKER: PeriodInPath
	_in := PeriodInPathIn{}
	_out := PeriodInPathOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, PeriodInPath.Method, PeriodInPath.Route, &_in, &_out)
	return err // No trace
}

// AnyMethod accepts any HTTP method.
func (_c Client) AnyMethod(ctx context.Context) (err error) { // MARKER: AnyMethod
	_in := AnyMethodIn{}
	_out := AnyMethodOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, AnyMethod.Method, AnyMethod.Route, &_in, &_out)
	return err // No trace
}

// InternalPort is on the :444 internal port.
func (_c Client) InternalPort(ctx context.Context) (err error) { // MARKER: InternalPort
	_in := InternalPortIn{}
	_out := InternalPortOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, InternalPort.Method, InternalPort.Route, &_in, &_out)
	return err // No trace
}

// TrustRoot is the trust-root :666 endpoint.
func (_c Client) TrustRoot(ctx context.Context, cmd string) (err error) { // MARKER: TrustRoot
	_in := TrustRootIn{Cmd: cmd}
	_out := TrustRootOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, TrustRoot.Method, TrustRoot.Route, &_in, &_out)
	return err // No trace
}

// SlashHostRoot has route "//root".
func (_c Client) SlashHostRoot(ctx context.Context) (err error) { // MARKER: SlashHostRoot
	_in := SlashHostRootIn{}
	_out := SlashHostRootOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, SlashHostRoot.Method, SlashHostRoot.Route, &_in, &_out)
	return err // No trace
}

// SlashHostPort has route "//alt.host:0/alt-path".
func (_c Client) SlashHostPort(ctx context.Context) (err error) { // MARKER: SlashHostPort
	_in := SlashHostPortIn{}
	_out := SlashHostPortOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, SlashHostPort.Method, SlashHostPort.Route, &_in, &_out)
	return err // No trace
}

// SlashHostPathArg has route "//alt.host/items/{id}".
func (_c Client) SlashHostPathArg(ctx context.Context, id string) (err error) { // MARKER: SlashHostPathArg
	_in := SlashHostPathArgIn{ID: id}
	_out := SlashHostPathArgOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, SlashHostPathArg.Method, SlashHostPathArg.Route, &_in, &_out)
	return err // No trace
}

// OnSomethingResponse packs the response of OnSomething.
type OnSomethingResponse multicastResponse // MARKER: OnSomething

// Get unpacks the return arguments of OnSomething.
func (_res *OnSomethingResponse) Get() (ok bool, err error) { // MARKER: OnSomething
	_d := _res.data.(*OnSomethingOut)
	return _d.OK, _res.err
}

/*
OnSomething is fired when something happens.
*/
func (_c MulticastTrigger) OnSomething(ctx context.Context, detail string) iter.Seq[*OnSomethingResponse] { // MARKER: OnSomething
	_in := OnSomethingIn{Detail: detail}
	_out := OnSomethingOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, OnSomething.Method, OnSomething.Route, &_in, &_out)
	return func(yield func(*OnSomethingResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*OnSomethingResponse)(_r)) {
				return
			}
		}
	}
}

// OnSomething registers a hook for the OnSomething event.
func (_c Hook) OnSomething(handler func(ctx context.Context, detail string) (ok bool, err error)) (err error) { // MARKER: OnSomething
	_ = handler
	return nil
}

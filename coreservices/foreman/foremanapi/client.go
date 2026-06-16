package foremanapi

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

// CreateResponse packs the response of Create.
type CreateResponse multicastResponse // MARKER: Create

// Get unpacks the return arguments of Create.
func (_res *CreateResponse) Get() (flowKey string, err error) { // MARKER: Create
	_d := _res.data.(*CreateOut)
	return _d.FlowKey, _res.err
}

/*
Create creates a new flow for a workflow without starting it.
*/
func (_c MulticastClient) Create(ctx context.Context, workflowURL string, initialState any, opts *workflow.FlowOptions) iter.Seq[*CreateResponse] { // MARKER: Create
	_in := CreateIn{WorkflowURL: workflowURL, InitialState: initialState, Opts: opts}
	_out := CreateOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return func(yield func(*CreateResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*CreateResponse)(_r)) {
				return
			}
		}
	}
}

/*
Create creates a new flow for a workflow without starting it.
*/
func (_c Client) Create(ctx context.Context, workflowURL string, initialState any, opts *workflow.FlowOptions) (flowKey string, err error) { // MARKER: Create
	_in := CreateIn{WorkflowURL: workflowURL, InitialState: initialState, Opts: opts}
	_out := CreateOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return _out.FlowKey, err // No trace
}

/*
Start transitions a created flow to running and enqueues it for execution.
*/
func (_c MulticastClient) Start(ctx context.Context, flowKey string) iter.Seq[*multicastResponse] { // MARKER: Start
	_in := StartIn{FlowKey: flowKey}
	_out := StartOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Start.Method, Start.Route, &_in, &_out)
}

/*
Start transitions a created flow to running and enqueues it for execution.
*/
func (_c Client) Start(ctx context.Context, flowKey string) (err error) { // MARKER: Start
	_in := StartIn{FlowKey: flowKey}
	_out := StartOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Start.Method, Start.Route, &_in, &_out)
	return err // No trace
}

// SnapshotResponse packs the response of Snapshot.
type SnapshotResponse multicastResponse // MARKER: Snapshot

// Get unpacks the return arguments of Snapshot.
func (_res *SnapshotResponse) Get() (outcome *workflow.FlowOutcome, err error) { // MARKER: Snapshot
	_d := _res.data.(*SnapshotOut)
	return _d.Outcome, _res.err
}

/*
Snapshot returns the current outcome of a flow.
*/
func (_c MulticastClient) Snapshot(ctx context.Context, flowKey string) iter.Seq[*SnapshotResponse] { // MARKER: Snapshot
	_in := SnapshotIn{FlowKey: flowKey}
	_out := SnapshotOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Snapshot.Method, Snapshot.Route, &_in, &_out)
	return func(yield func(*SnapshotResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*SnapshotResponse)(_r)) {
				return
			}
		}
	}
}

/*
Snapshot returns the current outcome of a flow.
*/
func (_c Client) Snapshot(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error) { // MARKER: Snapshot
	_in := SnapshotIn{FlowKey: flowKey}
	_out := SnapshotOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Snapshot.Method, Snapshot.Route, &_in, &_out)
	return _out.Outcome, err // No trace
}

// FingerprintResponse packs the response of Fingerprint.
type FingerprintResponse multicastResponse // MARKER: Fingerprint

// Get unpacks the return arguments of Fingerprint.
func (_res *FingerprintResponse) Get() (fingerprint string, status string, err error) { // MARKER: Fingerprint
	_d := _res.data.(*FingerprintOut)
	return _d.Fingerprint, _d.Status, _res.err
}

/*
Fingerprint returns an opaque hash that changes when a flow's status, step count, or any step's
updated_at changes — across the flow and any nested subgraph descendants.
*/
func (_c MulticastClient) Fingerprint(ctx context.Context, flowKey string) iter.Seq[*FingerprintResponse] { // MARKER: Fingerprint
	_in := FingerprintIn{FlowKey: flowKey}
	_out := FingerprintOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Fingerprint.Method, Fingerprint.Route, &_in, &_out)
	return func(yield func(*FingerprintResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*FingerprintResponse)(_r)) {
				return
			}
		}
	}
}

/*
Fingerprint returns an opaque hash that changes when a flow's status, step count, or any step's
updated_at changes — across the flow and any nested subgraph descendants.
*/
func (_c Client) Fingerprint(ctx context.Context, flowKey string) (fingerprint string, status string, err error) { // MARKER: Fingerprint
	_in := FingerprintIn{FlowKey: flowKey}
	_out := FingerprintOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Fingerprint.Method, Fingerprint.Route, &_in, &_out)
	return _out.Fingerprint, _out.Status, err // No trace
}

/*
Resume continues an interrupted flow, delivering resumeData to the task that armed flow.Interrupt. Fails if the flow
is paused at a breakpoint rather than an interrupt.
*/
func (_c MulticastClient) Resume(ctx context.Context, flowKey string, resumeData any) iter.Seq[*multicastResponse] { // MARKER: Resume
	_in := ResumeIn{FlowKey: flowKey, ResumeData: resumeData}
	_out := ResumeOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Resume.Method, Resume.Route, &_in, &_out)
}

/*
Resume continues an interrupted flow, delivering resumeData to the task that armed flow.Interrupt. Fails if the flow
is paused at a breakpoint rather than an interrupt.
*/
func (_c Client) Resume(ctx context.Context, flowKey string, resumeData any) (err error) { // MARKER: Resume
	_in := ResumeIn{FlowKey: flowKey, ResumeData: resumeData}
	_out := ResumeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Resume.Method, Resume.Route, &_in, &_out)
	return err // No trace
}

/*
ResumeBreak continues a flow paused at a breakpoint, merging stateOverrides into the leaf step's input state.
Fails if the flow is paused at an interrupt rather than a breakpoint.
*/
func (_c MulticastClient) ResumeBreak(ctx context.Context, flowKey string, stateOverrides any) iter.Seq[*multicastResponse] { // MARKER: ResumeBreak
	_in := ResumeBreakIn{FlowKey: flowKey, StateOverrides: stateOverrides}
	_out := ResumeBreakOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, ResumeBreak.Method, ResumeBreak.Route, &_in, &_out)
}

/*
ResumeBreak continues a flow paused at a breakpoint, merging stateOverrides into the leaf step's input state.
Fails if the flow is paused at an interrupt rather than a breakpoint.
*/
func (_c Client) ResumeBreak(ctx context.Context, flowKey string, stateOverrides any) (err error) { // MARKER: ResumeBreak
	_in := ResumeBreakIn{FlowKey: flowKey, StateOverrides: stateOverrides}
	_out := ResumeBreakOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, ResumeBreak.Method, ResumeBreak.Route, &_in, &_out)
	return err // No trace
}

/*
Cancel cancels a flow that is not yet in a terminal status. The reason string is recorded on
every flow in the chain as cancel_reason.
*/
func (_c MulticastClient) Cancel(ctx context.Context, flowKey string, reason string) iter.Seq[*multicastResponse] { // MARKER: Cancel
	_in := CancelIn{FlowKey: flowKey, Reason: reason}
	_out := CancelOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Cancel.Method, Cancel.Route, &_in, &_out)
}

/*
Cancel cancels a flow that is not yet in a terminal status. The reason string is recorded on
every flow in the chain as cancel_reason.
*/
func (_c Client) Cancel(ctx context.Context, flowKey string, reason string) (err error) { // MARKER: Cancel
	_in := CancelIn{FlowKey: flowKey, Reason: reason}
	_out := CancelOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Cancel.Method, Cancel.Route, &_in, &_out)
	return err // No trace
}

/*
Restart wipes everything past a flow's entry step, resets the entry step with overrides, and flips the
flow to running. The flow must be in a terminal status.
*/
func (_c MulticastClient) Restart(ctx context.Context, flowKey string, stateOverrides any) iter.Seq[*multicastResponse] { // MARKER: Restart
	_in := RestartIn{FlowKey: flowKey, StateOverrides: stateOverrides}
	_out := RestartOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Restart.Method, Restart.Route, &_in, &_out)
}

/*
Restart wipes everything past a flow's entry step, resets the entry step with overrides, and flips the
flow to running. The flow must be in a terminal status.
*/
func (_c Client) Restart(ctx context.Context, flowKey string, stateOverrides any) (err error) { // MARKER: Restart
	_in := RestartIn{FlowKey: flowKey, StateOverrides: stateOverrides}
	_out := RestartOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Restart.Method, Restart.Route, &_in, &_out)
	return err // No trace
}

/*
RestartFrom sweeps the DAG subtree downstream of the named step, resets the target step with overrides,
and flips the flow to running.
*/
func (_c MulticastClient) RestartFrom(ctx context.Context, stepKey string, stateOverrides any) iter.Seq[*multicastResponse] { // MARKER: RestartFrom
	_in := RestartFromIn{StepKey: stepKey, StateOverrides: stateOverrides}
	_out := RestartFromOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, RestartFrom.Method, RestartFrom.Route, &_in, &_out)
}

/*
RestartFrom sweeps the DAG subtree downstream of the named step, resets the target step with overrides,
and flips the flow to running.
*/
func (_c Client) RestartFrom(ctx context.Context, stepKey string, stateOverrides any) (err error) { // MARKER: RestartFrom
	_in := RestartFromIn{StepKey: stepKey, StateOverrides: stateOverrides}
	_out := RestartFromOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, RestartFrom.Method, RestartFrom.Route, &_in, &_out)
	return err // No trace
}

// HistoryResponse packs the response of History.
type HistoryResponse multicastResponse // MARKER: History

// Get unpacks the return arguments of History.
func (_res *HistoryResponse) Get() (steps []FlowStep, err error) { // MARKER: History
	_d := _res.data.(*HistoryOut)
	return _d.Steps, _res.err
}

/*
History returns the step-by-step execution history of a flow.
*/
func (_c MulticastClient) History(ctx context.Context, flowKey string) iter.Seq[*HistoryResponse] { // MARKER: History
	_in := HistoryIn{FlowKey: flowKey}
	_out := HistoryOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, History.Method, History.Route, &_in, &_out)
	return func(yield func(*HistoryResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*HistoryResponse)(_r)) {
				return
			}
		}
	}
}

/*
History returns the step-by-step execution history of a flow.
*/
func (_c Client) History(ctx context.Context, flowKey string) (steps []FlowStep, err error) { // MARKER: History
	_in := HistoryIn{FlowKey: flowKey}
	_out := HistoryOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, History.Method, History.Route, &_in, &_out)
	return _out.Steps, err // No trace
}

// StepResponse packs the response of Step.
type StepResponse multicastResponse // MARKER: Step

// Get unpacks the return arguments of Step.
func (_res *StepResponse) Get() (step *FlowStep, err error) { // MARKER: Step
	_d := _res.data.(*StepOut)
	return _d.Step, _res.err
}

/*
Step returns the full detail of one execution step, including the state, changes and interrupt payload
that History intentionally omits to keep flow-wide responses bounded.
*/
func (_c MulticastClient) Step(ctx context.Context, stepKey string) iter.Seq[*StepResponse] { // MARKER: Step
	_in := StepIn{StepKey: stepKey}
	_out := StepOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Step.Method, Step.Route, &_in, &_out)
	return func(yield func(*StepResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*StepResponse)(_r)) {
				return
			}
		}
	}
}

/*
Step returns the full detail of one execution step, including the state, changes and interrupt payload
that History intentionally omits to keep flow-wide responses bounded.
*/
func (_c Client) Step(ctx context.Context, stepKey string) (step *FlowStep, err error) { // MARKER: Step
	_in := StepIn{StepKey: stepKey}
	_out := StepOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Step.Method, Step.Route, &_in, &_out)
	return _out.Step, err // No trace
}

// ListResponse packs the response of List.
type ListResponse multicastResponse // MARKER: List

// Get unpacks the return arguments of List.
func (_res *ListResponse) Get() (flows []FlowSummary, nextCursor string, err error) { // MARKER: List
	_d := _res.data.(*ListOut)
	return _d.Flows, _d.NextCursor, _res.err
}

/*
List queries flows by status or workflow URL with per-shard pagination. Set Query.Cursor to the
previous call's NextCursor to paginate; an empty NextCursor signals end-of-results.
*/
func (_c MulticastClient) List(ctx context.Context, query Query) iter.Seq[*ListResponse] { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return func(yield func(*ListResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*ListResponse)(_r)) {
				return
			}
		}
	}
}

/*
List queries flows by status or workflow URL with per-shard pagination. Set Query.Cursor to the
previous call's NextCursor to paginate; an empty NextCursor signals end-of-results.
*/
func (_c Client) List(ctx context.Context, query Query) (flows []FlowSummary, nextCursor string, err error) { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return _out.Flows, _out.NextCursor, err // No trace
}

/*
Delete removes a flow and its steps from the database. The flow must not be running.
Subgraph and thread lineage references become dangling.
*/
func (_c MulticastClient) Delete(ctx context.Context, flowKey string) iter.Seq[*multicastResponse] { // MARKER: Delete
	_in := DeleteIn{FlowKey: flowKey}
	_out := DeleteOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Delete.Method, Delete.Route, &_in, &_out)
}

/*
Delete removes a flow and its steps from the database. The flow must not be running.
Subgraph and thread lineage references become dangling.
*/
func (_c Client) Delete(ctx context.Context, flowKey string) (err error) { // MARKER: Delete
	_in := DeleteIn{FlowKey: flowKey}
	_out := DeleteOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Delete.Method, Delete.Route, &_in, &_out)
	return err // No trace
}

// PurgeResponse packs the response of Purge.
type PurgeResponse multicastResponse // MARKER: Purge

// Get unpacks the return arguments of Purge.
func (_res *PurgeResponse) Get() (deleted int, err error) { // MARKER: Purge
	_d := _res.data.(*PurgeOut)
	return _d.Deleted, _res.err
}

/*
Purge deletes flows matching the query, except those currently running. Capped at 10000
flows per call. Returns the count of flows actually deleted.
*/
func (_c MulticastClient) Purge(ctx context.Context, query Query) iter.Seq[*PurgeResponse] { // MARKER: Purge
	_in := PurgeIn{Query: query}
	_out := PurgeOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Purge.Method, Purge.Route, &_in, &_out)
	return func(yield func(*PurgeResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*PurgeResponse)(_r)) {
				return
			}
		}
	}
}

/*
Purge deletes flows matching the query, except those currently running. Capped at 10000
flows per call. Returns the count of flows actually deleted.
*/
func (_c Client) Purge(ctx context.Context, query Query) (deleted int, err error) { // MARKER: Purge
	_in := PurgeIn{Query: query}
	_out := PurgeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Purge.Method, Purge.Route, &_in, &_out)
	return _out.Deleted, err // No trace
}

// ShardInfoResponse packs the response of ShardInfo.
type ShardInfoResponse multicastResponse // MARKER: ShardInfo

// Get unpacks the return arguments of ShardInfo.
func (_res *ShardInfoResponse) Get() (shards []ShardSummary, err error) { // MARKER: ShardInfo
	_d := _res.data.(*ShardInfoOut)
	return _d.Shards, _res.err
}

/*
ShardInfo returns per-shard health (latency, row counts, error) for every database shard.
*/
func (_c MulticastClient) ShardInfo(ctx context.Context) iter.Seq[*ShardInfoResponse] { // MARKER: ShardInfo
	_in := ShardInfoIn{}
	_out := ShardInfoOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, ShardInfo.Method, ShardInfo.Route, &_in, &_out)
	return func(yield func(*ShardInfoResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*ShardInfoResponse)(_r)) {
				return
			}
		}
	}
}

/*
ShardInfo returns per-shard health (latency, row counts, error) for every database shard.
*/
func (_c Client) ShardInfo(ctx context.Context) (shards []ShardSummary, err error) { // MARKER: ShardInfo
	_in := ShardInfoIn{}
	_out := ShardInfoOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, ShardInfo.Method, ShardInfo.Route, &_in, &_out)
	return _out.Shards, err // No trace
}

// CreateTaskResponse packs the response of CreateTask.
type CreateTaskResponse multicastResponse // MARKER: CreateTask

// Get unpacks the return arguments of CreateTask.
func (_res *CreateTaskResponse) Get() (flowKey string, err error) { // MARKER: CreateTask
	_d := _res.data.(*CreateTaskOut)
	return _d.FlowKey, _res.err
}

/*
CreateTask creates a flow that executes a single task and then terminates, without starting it.
*/
func (_c MulticastClient) CreateTask(ctx context.Context, taskURL string, initialState any, opts *workflow.FlowOptions) iter.Seq[*CreateTaskResponse] { // MARKER: CreateTask
	_in := CreateTaskIn{TaskURL: taskURL, InitialState: initialState, Opts: opts}
	_out := CreateTaskOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, CreateTask.Method, CreateTask.Route, &_in, &_out)
	return func(yield func(*CreateTaskResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*CreateTaskResponse)(_r)) {
				return
			}
		}
	}
}

/*
CreateTask creates a flow that executes a single task and then terminates, without starting it.
*/
func (_c Client) CreateTask(ctx context.Context, taskURL string, initialState any, opts *workflow.FlowOptions) (flowKey string, err error) { // MARKER: CreateTask
	_in := CreateTaskIn{TaskURL: taskURL, InitialState: initialState, Opts: opts}
	_out := CreateTaskOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, CreateTask.Method, CreateTask.Route, &_in, &_out)
	return _out.FlowKey, err // No trace
}

// AwaitResponse packs the response of Await.
type AwaitResponse multicastResponse // MARKER: Await

// Get unpacks the return arguments of Await.
func (_res *AwaitResponse) Get() (outcome *workflow.FlowOutcome, err error) { // MARKER: Await
	_d := _res.data.(*AwaitOut)
	return _d.Outcome, _res.err
}

/*
Await blocks until the flow stops (i.e. is no longer created, pending, or running), then returns the workflow.FlowOutcome.
*/
func (_c MulticastClient) Await(ctx context.Context, flowKey string) iter.Seq[*AwaitResponse] { // MARKER: Await
	_in := AwaitIn{FlowKey: flowKey}
	_out := AwaitOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Await.Method, Await.Route, &_in, &_out)
	return func(yield func(*AwaitResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*AwaitResponse)(_r)) {
				return
			}
		}
	}
}

/*
Await blocks until the flow stops (i.e. is no longer created, pending, or running), then returns the workflow.FlowOutcome.
*/
func (_c Client) Await(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error) { // MARKER: Await
	_in := AwaitIn{FlowKey: flowKey}
	_out := AwaitOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Await.Method, Await.Route, &_in, &_out)
	return _out.Outcome, err // No trace
}

/*
BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
*/
func (_c MulticastClient) BreakBefore(ctx context.Context, flowKey string, taskName string, enabled bool) iter.Seq[*multicastResponse] { // MARKER: BreakBefore
	_in := BreakBeforeIn{FlowKey: flowKey, TaskName: taskName, Enabled: enabled}
	_out := BreakBeforeOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, BreakBefore.Method, BreakBefore.Route, &_in, &_out)
}

/*
BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
*/
func (_c Client) BreakBefore(ctx context.Context, flowKey string, taskName string, enabled bool) (err error) { // MARKER: BreakBefore
	_in := BreakBeforeIn{FlowKey: flowKey, TaskName: taskName, Enabled: enabled}
	_out := BreakBeforeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BreakBefore.Method, BreakBefore.Route, &_in, &_out)
	return err // No trace
}

// RunResponse packs the response of Run.
type RunResponse multicastResponse // MARKER: Run

// Get unpacks the return arguments of Run.
func (_res *RunResponse) Get() (outcome *workflow.FlowOutcome, err error) { // MARKER: Run
	_d := _res.data.(*RunOut)
	return _d.Outcome, _res.err
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal workflow.FlowOutcome.
A workflow failure surfaces as outcome.Status="failed" with outcome.Error populated; the Go-level
error return is for transport/foreman/timeout failures only.
*/
func (_c MulticastClient) Run(ctx context.Context, workflowURL string, initialState any, opts *workflow.FlowOptions) iter.Seq[*RunResponse] { // MARKER: Run
	_in := RunIn{WorkflowURL: workflowURL, InitialState: initialState, Opts: opts}
	_out := RunOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Run.Method, Run.Route, &_in, &_out)
	return func(yield func(*RunResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*RunResponse)(_r)) {
				return
			}
		}
	}
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal workflow.FlowOutcome.
A workflow failure surfaces as outcome.Status="failed" with outcome.Error populated; the Go-level
error return is for transport/foreman/timeout failures only.
*/
func (_c Client) Run(ctx context.Context, workflowURL string, initialState any, opts *workflow.FlowOptions) (outcome *workflow.FlowOutcome, err error) { // MARKER: Run
	_in := RunIn{WorkflowURL: workflowURL, InitialState: initialState, Opts: opts}
	_out := RunOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Run.Method, Run.Route, &_in, &_out)
	return _out.Outcome, err // No trace
}

// ContinueResponse packs the response of Continue.
type ContinueResponse multicastResponse // MARKER: Continue

// Get unpacks the return arguments of Continue.
func (_res *ContinueResponse) Get() (newFlowKey string, err error) { // MARKER: Continue
	_d := _res.data.(*ContinueOut)
	return _d.NewFlowKey, _res.err
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state using
the graph's reducers. The threadKey can be any flowKey belonging to the thread. The new flow belongs to the
same thread and is returned in created status.
*/
func (_c MulticastClient) Continue(ctx context.Context, threadKey string, additionalState any, opts *workflow.FlowOptions) iter.Seq[*ContinueResponse] { // MARKER: Continue
	_in := ContinueIn{ThreadKey: threadKey, AdditionalState: additionalState, Opts: opts}
	_out := ContinueOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Continue.Method, Continue.Route, &_in, &_out)
	return func(yield func(*ContinueResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*ContinueResponse)(_r)) {
				return
			}
		}
	}
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state using
the graph's reducers. The threadKey can be any flowKey belonging to the thread. The new flow belongs to the
same thread and is returned in created status.
*/
func (_c Client) Continue(ctx context.Context, threadKey string, additionalState any, opts *workflow.FlowOptions) (newFlowKey string, err error) { // MARKER: Continue
	_in := ContinueIn{ThreadKey: threadKey, AdditionalState: additionalState, Opts: opts}
	_out := ContinueOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Continue.Method, Continue.Route, &_in, &_out)
	return _out.NewFlowKey, err // No trace
}

/*
Signal multicasts an opaque cross-replica coordination signal (op, payload) to peer foreman replicas.
*/
func (_c MulticastClient) Signal(ctx context.Context, op string, payload []byte) iter.Seq[*multicastResponse] { // MARKER: Signal
	_in := SignalIn{Op: op, Payload: payload}
	_out := SignalOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Signal.Method, Signal.Route, &_in, &_out)
}

/*
Signal multicasts an opaque cross-replica coordination signal (op, payload) to peer foreman replicas.
*/
func (_c Client) Signal(ctx context.Context, op string, payload []byte) (err error) { // MARKER: Signal
	_in := SignalIn{Op: op, Payload: payload}
	_out := SignalOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Signal.Method, Signal.Route, &_in, &_out)
	return err // No trace
}

/*
HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) HistoryMermaid(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: HistoryMermaid
	return _c.svc.Request(
		ctx,
		pub.Method(HistoryMermaid.Method),
		pub.URL(httpx.JoinHostAndPath(_c.host, HistoryMermaid.Route)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) HistoryMermaid(ctx context.Context, relativeURL string) iter.Seq[*pub.Response] { // MARKER: HistoryMermaid
	return _c.svc.Publish(
		ctx,
		pub.Method(HistoryMermaid.Method),
		pub.URL(httpx.JoinHostAndPath(_c.host, HistoryMermaid.Route)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

// OnFlowStoppedResponse packs the response of OnFlowStopped.
type OnFlowStoppedResponse multicastResponse // MARKER: OnFlowStopped

// Get retrieves the return values.
func (_res *OnFlowStoppedResponse) Get() (err error) { // MARKER: OnFlowStopped
	return _res.err
}

/*
OnFlowStopped is triggered when a flow stops (completed, failed, cancelled, or interrupted). Subscribe with ForHost(svc.Hostname()) for flows created with FlowOptions.NotifyOnStop.
*/
func (_c MulticastTrigger) OnFlowStopped(ctx context.Context, outcome *workflow.FlowOutcome) iter.Seq[*OnFlowStoppedResponse] { // MARKER: OnFlowStopped
	_in := OnFlowStoppedIn{Outcome: outcome}
	_out := OnFlowStoppedOut{}
	_inner := marshalPublish(ctx, _c.svc, _c.opts, _c.host, OnFlowStopped.Method, OnFlowStopped.Route, &_in, &_out)
	return func(yield func(*OnFlowStoppedResponse) bool) {
		for _r := range _inner {
			_clone := _out
			_r.data = &_clone
			if !yield((*OnFlowStoppedResponse)(_r)) {
				return
			}
		}
	}
}

/*
OnFlowStopped is triggered when a flow stops (completed, failed, cancelled, or interrupted). Subscribe with ForHost(svc.Hostname()) for flows created with FlowOptions.NotifyOnStop.
*/
func (c Hook) OnFlowStopped(handler func(ctx context.Context, outcome *workflow.FlowOutcome) (err error)) (unsub func() error, err error) { // MARKER: OnFlowStopped
	doOnFlowStopped := func(w http.ResponseWriter, r *http.Request) error {
		var in OnFlowStoppedIn
		var out OnFlowStoppedOut
		err = marshalFunction(w, r, OnFlowStopped.Route, &in, &out, func(_ any, _ any) error {
			err = handler(r.Context(), in.Outcome)
			return err
		})
		return err // No trace
	}
	const name = "OnFlowStopped"
	path := httpx.JoinHostAndPath(c.host, OnFlowStopped.Route)
	subOpts := append([]sub.Option{
		sub.At(OnFlowStopped.Method, path),
		sub.InboundEvent(OnFlowStoppedIn{}, OnFlowStoppedOut{}),
	}, c.opts...)
	if err := c.svc.Subscribe(name, doOnFlowStopped, subOpts...); err != nil {
		return nil, errors.Trace(err)
	}
	return func() error { return c.svc.Unsubscribe(name) }, nil
}

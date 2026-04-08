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

package foremanapi

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
	_ workflow.Flow
)

// Hostname is the default hostname of the microservice.
const Hostname = "foreman.core"

// Def defines an endpoint of the microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL to the endpoint.
func (d *Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	Create             = Def{Method: "ANY", Route: ":444/create"}                // MARKER: Create
	CreateTask         = Def{Method: "POST", Route: ":444/create-task"}          // MARKER: CreateTask
	Start              = Def{Method: "ANY", Route: ":444/start"}                 // MARKER: Start
	StartNotify        = Def{Method: "ANY", Route: ":444/start-notify"}          // MARKER: StartNotify
	Snapshot           = Def{Method: "GET", Route: ":444/snapshot"}              // MARKER: Snapshot
	Resume             = Def{Method: "POST", Route: ":444/resume"}               // MARKER: Resume
	Fork               = Def{Method: "POST", Route: ":444/fork"}                 // MARKER: Fork
	Cancel             = Def{Method: "POST", Route: ":444/cancel"}               // MARKER: Cancel
	History            = Def{Method: "GET", Route: ":444/history"}               // MARKER: History
	Retry              = Def{Method: "POST", Route: ":444/retry"}                // MARKER: Retry
	List               = Def{Method: "GET", Route: ":444/list"}                  // MARKER: List
	Enqueue            = Def{Method: "POST", Route: ":444/enqueue"}              // MARKER: Enqueue
	Await              = Def{Method: "POST", Route: ":444/wait-for-stop"}        // MARKER: Await
	NotifyStatusChange = Def{Method: "POST", Route: ":444/notify-status-change"} // MARKER: NotifyStatusChange
	OnFlowStopped      = Def{Method: "POST", Route: ":417/on-flow-terminated"}   // MARKER: OnFlowStopped
	BreakBefore        = Def{Method: "POST", Route: ":444/break-before"}         // MARKER: BreakBefore
	Run                = Def{Method: "POST", Route: ":444/run"}                  // MARKER: Run
	Continue           = Def{Method: "POST", Route: ":444/continue"}             // MARKER: Continue
	HistoryMermaid     = Def{Method: "GET", Route: ":444/history-mermaid"}       // MARKER: HistoryMermaid
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

// --- Create ---

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	WorkflowName string `json:"workflowName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	FlowID string `json:"flowID,omitzero"`
}

// CreateResponse packs the response of Create.
type CreateResponse multicastResponse // MARKER: Create

// Get unpacks the return arguments of Create.
func (_res *CreateResponse) Get() (flowID string, err error) { // MARKER: Create
	_d := _res.data.(*CreateOut)
	return _d.FlowID, _res.err
}

/*
Create creates a new flow for a workflow without starting it.
*/
func (_c MulticastClient) Create(ctx context.Context, workflowName string, initialState any) iter.Seq[*CreateResponse] { // MARKER: Create
	_in := CreateIn{WorkflowName: workflowName, InitialState: initialState}
	_out := CreateOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return func(yield func(*CreateResponse) bool) {
		for _r := range _queue {
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
func (_c Client) Create(ctx context.Context, workflowName string, initialState any) (flowID string, err error) { // MARKER: Create
	_in := CreateIn{WorkflowName: workflowName, InitialState: initialState}
	_out := CreateOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return _out.FlowID, err // No trace
}

// --- Start ---

// StartIn are the input arguments of Start.
type StartIn struct { // MARKER: Start
	FlowID string `json:"flowID,omitzero"`
}

// StartOut are the output arguments of Start.
type StartOut struct { // MARKER: Start
}

/*
Start transitions a created flow to running and enqueues it for execution.
*/
func (_c MulticastClient) Start(ctx context.Context, flowID string) iter.Seq[*multicastResponse] { // MARKER: Start
	_in := StartIn{FlowID: flowID}
	_out := StartOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Start.Method, Start.Route, &_in, &_out)
}

/*
Start transitions a created flow to running and enqueues it for execution.
*/
func (_c Client) Start(ctx context.Context, flowID string) (err error) { // MARKER: Start
	_in := StartIn{FlowID: flowID}
	_out := StartOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Start.Method, Start.Route, &_in, &_out)
	return err // No trace
}

// --- StartNotify ---

// StartNotifyIn are the input arguments of StartNotify.
type StartNotifyIn struct { // MARKER: StartNotify
	FlowID         string `json:"flowID,omitzero"`
	NotifyHostname string `json:"notifyHostname,omitzero"`
}

// StartNotifyOut are the output arguments of StartNotify.
type StartNotifyOut struct { // MARKER: StartNotify
}

/*
StartNotify transitions a created flow to running, stores the notification
hostname, and enqueues it for execution. The caller receives an
OnFlowStopped event at the given hostname when the flow reaches a terminal status.
*/
func (_c MulticastClient) StartNotify(ctx context.Context, flowID string, notifyHostname string) iter.Seq[*multicastResponse] { // MARKER: StartNotify
	_in := StartNotifyIn{FlowID: flowID, NotifyHostname: notifyHostname}
	_out := StartNotifyOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, StartNotify.Method, StartNotify.Route, &_in, &_out)
}

/*
StartNotify transitions a created flow to running, stores the notification
hostname, and enqueues it for execution. The caller receives an
OnFlowStopped event at the given hostname when the flow reaches a terminal status.
*/
func (_c Client) StartNotify(ctx context.Context, flowID string, notifyHostname string) (err error) { // MARKER: StartNotify
	_in := StartNotifyIn{FlowID: flowID, NotifyHostname: notifyHostname}
	_out := StartNotifyOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, StartNotify.Method, StartNotify.Route, &_in, &_out)
	return err // No trace
}

// --- Snapshot ---

// SnapshotIn are the input arguments of Snapshot.
type SnapshotIn struct { // MARKER: Snapshot
	FlowID string `json:"flowID,omitzero"`
}

// SnapshotOut are the output arguments of Snapshot.
type SnapshotOut struct { // MARKER: Snapshot
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
}

// SnapshotResponse packs the response of Snapshot.
type SnapshotResponse multicastResponse // MARKER: Snapshot

// Get unpacks the return arguments of Snapshot.
func (_res *SnapshotResponse) Get() (status string, state map[string]any, err error) { // MARKER: Snapshot
	_d := _res.data.(*SnapshotOut)
	return _d.Status, _d.State, _res.err
}

/*
Snapshot returns the current status and state of a flow.
*/
func (_c MulticastClient) Snapshot(ctx context.Context, flowID string) iter.Seq[*SnapshotResponse] { // MARKER: Snapshot
	_in := SnapshotIn{FlowID: flowID}
	_out := SnapshotOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Snapshot.Method, Snapshot.Route, &_in, &_out)
	return func(yield func(*SnapshotResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*SnapshotResponse)(_r)) {
				return
			}
		}
	}
}

/*
Snapshot returns the current status and state of a flow.
*/
func (_c Client) Snapshot(ctx context.Context, flowID string) (status string, state map[string]any, err error) { // MARKER: Snapshot
	_in := SnapshotIn{FlowID: flowID}
	_out := SnapshotOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Snapshot.Method, Snapshot.Route, &_in, &_out)
	return _out.Status, _out.State, err // No trace
}

// --- Resume ---

// ResumeIn are the input arguments of Resume.
type ResumeIn struct { // MARKER: Resume
	FlowID     string `json:"flowID,omitzero"`
	ResumeData any    `json:"resumeData,omitzero"`
}

// ResumeOut are the output arguments of Resume.
type ResumeOut struct { // MARKER: Resume
}

/*
Resume resumes an interrupted flow by merging resumeData into the current step's changes
and re-enqueuing it for execution.
*/
func (_c MulticastClient) Resume(ctx context.Context, flowID string, resumeData any) iter.Seq[*multicastResponse] { // MARKER: Resume
	_in := ResumeIn{FlowID: flowID, ResumeData: resumeData}
	_out := ResumeOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Resume.Method, Resume.Route, &_in, &_out)
}

/*
Resume resumes an interrupted flow by merging resumeData into the current step's changes
and re-enqueuing it for execution.
*/
func (_c Client) Resume(ctx context.Context, flowID string, resumeData any) (err error) { // MARKER: Resume
	_in := ResumeIn{FlowID: flowID, ResumeData: resumeData}
	_out := ResumeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Resume.Method, Resume.Route, &_in, &_out)
	return err // No trace
}

// --- Fork ---

// ForkIn are the input arguments of Fork.
type ForkIn struct { // MARKER: Fork
	StepKey        string `json:"stepKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// ForkOut are the output arguments of Fork.
type ForkOut struct { // MARKER: Fork
	NewFlowKey string `json:"newFlowKey,omitzero"`
}

// ForkResponse packs the response of Fork.
type ForkResponse multicastResponse // MARKER: Fork

// Get unpacks the return arguments of Fork.
func (_res *ForkResponse) Get() (newFlowKey string, err error) { // MARKER: Fork
	_d := _res.data.(*ForkOut)
	return _d.NewFlowKey, _res.err
}

/*
Fork creates a new flow from an existing step's checkpoint.
*/
func (_c MulticastClient) Fork(ctx context.Context, stepKey string, stateOverrides any) iter.Seq[*ForkResponse] { // MARKER: Fork
	_in := ForkIn{StepKey: stepKey, StateOverrides: stateOverrides}
	_out := ForkOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Fork.Method, Fork.Route, &_in, &_out)
	return func(yield func(*ForkResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ForkResponse)(_r)) {
				return
			}
		}
	}
}

/*
Fork creates a new flow from an existing step's checkpoint.
*/
func (_c Client) Fork(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error) { // MARKER: Fork
	_in := ForkIn{StepKey: stepKey, StateOverrides: stateOverrides}
	_out := ForkOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Fork.Method, Fork.Route, &_in, &_out)
	return _out.NewFlowKey, err // No trace
}

// --- Cancel ---

// CancelIn are the input arguments of Cancel.
type CancelIn struct { // MARKER: Cancel
	FlowID string `json:"flowID,omitzero"`
}

// CancelOut are the output arguments of Cancel.
type CancelOut struct { // MARKER: Cancel
}

/*
Cancel cancels a flow that is not yet in a terminal status.
*/
func (_c MulticastClient) Cancel(ctx context.Context, flowID string) iter.Seq[*multicastResponse] { // MARKER: Cancel
	_in := CancelIn{FlowID: flowID}
	_out := CancelOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Cancel.Method, Cancel.Route, &_in, &_out)
}

/*
Cancel cancels a flow that is not yet in a terminal status.
*/
func (_c Client) Cancel(ctx context.Context, flowID string) (err error) { // MARKER: Cancel
	_in := CancelIn{FlowID: flowID}
	_out := CancelOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Cancel.Method, Cancel.Route, &_in, &_out)
	return err // No trace
}

// --- History ---

// HistoryIn are the input arguments of History.
type HistoryIn struct { // MARKER: History
	FlowID string `json:"flowID,omitzero"`
}

// HistoryOut are the output arguments of History.
type HistoryOut struct { // MARKER: History
	Steps []FlowStep `json:"steps,omitzero"`
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
func (_c MulticastClient) History(ctx context.Context, flowID string) iter.Seq[*HistoryResponse] { // MARKER: History
	_in := HistoryIn{FlowID: flowID}
	_out := HistoryOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, History.Method, History.Route, &_in, &_out)
	return func(yield func(*HistoryResponse) bool) {
		for _r := range _queue {
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
func (_c Client) History(ctx context.Context, flowID string) (steps []FlowStep, err error) { // MARKER: History
	_in := HistoryIn{FlowID: flowID}
	_out := HistoryOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, History.Method, History.Route, &_in, &_out)
	return _out.Steps, err // No trace
}

// --- Retry ---

// RetryIn are the input arguments of Retry.
type RetryIn struct { // MARKER: Retry
	FlowID string `json:"flowID,omitzero"`
}

// RetryOut are the output arguments of Retry.
type RetryOut struct { // MARKER: Retry
}

/*
Retry re-executes the last failed step of a flow.
*/
func (_c MulticastClient) Retry(ctx context.Context, flowID string) iter.Seq[*multicastResponse] { // MARKER: Retry
	_in := RetryIn{FlowID: flowID}
	_out := RetryOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Retry.Method, Retry.Route, &_in, &_out)
}

/*
Retry re-executes the last failed step of a flow.
*/
func (_c Client) Retry(ctx context.Context, flowID string) (err error) { // MARKER: Retry
	_in := RetryIn{FlowID: flowID}
	_out := RetryOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Retry.Method, Retry.Route, &_in, &_out)
	return err // No trace
}

// --- List ---

// ListIn are the input arguments of List.
type ListIn struct { // MARKER: List
	Query Query `json:"query,omitzero"`
}

// ListOut are the output arguments of List.
type ListOut struct { // MARKER: List
	Flows []FlowSummary `json:"flows,omitzero"`
}

// ListResponse packs the response of List.
type ListResponse multicastResponse // MARKER: List

// Get unpacks the return arguments of List.
func (_res *ListResponse) Get() (flows []FlowSummary, err error) { // MARKER: List
	_d := _res.data.(*ListOut)
	return _d.Flows, _res.err
}

/*
List queries flows by status or workflow name.
Results are ordered by flow ID descending (newest first).
Set CursorFlowID in the query to the last result's flow ID to paginate. Limit defaults to 100 if not set.
*/
func (_c MulticastClient) List(ctx context.Context, query Query) iter.Seq[*ListResponse] { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return func(yield func(*ListResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ListResponse)(_r)) {
				return
			}
		}
	}
}

/*
List queries flows by status or workflow name.
Results are ordered by flow ID descending (newest first).
Set CursorFlowID in the query to the last result's flow ID to paginate. Limit defaults to 100 if not set.
*/
func (_c Client) List(ctx context.Context, query Query) (flows []FlowSummary, err error) { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return _out.Flows, err // No trace
}

// --- Enqueue ---

// EnqueueIn are the input arguments of Enqueue.
type EnqueueIn struct { // MARKER: Enqueue
	Shard  int `json:"shard,omitzero"`
	StepID int `json:"stepID,omitzero"`
}

// EnqueueOut are the output arguments of Enqueue.
type EnqueueOut struct { // MARKER: Enqueue
}

/*
Enqueue adds a step to the local work queue for processing.
*/
func (_c MulticastClient) Enqueue(ctx context.Context, shard int, stepID int) iter.Seq[*multicastResponse] { // MARKER: Enqueue
	_in := EnqueueIn{Shard: shard, StepID: stepID}
	_out := EnqueueOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, Enqueue.Method, Enqueue.Route, &_in, &_out)
}

/*
Enqueue adds a step to the local work queue for processing.
*/
func (_c Client) Enqueue(ctx context.Context, shard int, stepID int) (err error) { // MARKER: Enqueue
	_in := EnqueueIn{Shard: shard, StepID: stepID}
	_out := EnqueueOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Enqueue.Method, Enqueue.Route, &_in, &_out)
	return err // No trace
}

// --- CreateTask ---

// CreateTaskIn are the input arguments of CreateTask.
type CreateTaskIn struct { // MARKER: CreateTask
	TaskName     string `json:"taskName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
}

// CreateTaskOut are the output arguments of CreateTask.
type CreateTaskOut struct { // MARKER: CreateTask
	FlowID string `json:"flowID,omitzero"`
}

// CreateTaskResponse packs the response of CreateTask.
type CreateTaskResponse multicastResponse // MARKER: CreateTask

// Get unpacks the return arguments of CreateTask.
func (_res *CreateTaskResponse) Get() (flowID string, err error) { // MARKER: CreateTask
	_d := _res.data.(*CreateTaskOut)
	return _d.FlowID, _res.err
}

/*
CreateTask creates a flow that executes a single task and then terminates, without starting it.
*/
func (_c MulticastClient) CreateTask(ctx context.Context, taskName string, initialState any) iter.Seq[*CreateTaskResponse] { // MARKER: CreateTask
	_in := CreateTaskIn{TaskName: taskName, InitialState: initialState}
	_out := CreateTaskOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, CreateTask.Method, CreateTask.Route, &_in, &_out)
	return func(yield func(*CreateTaskResponse) bool) {
		for _r := range _queue {
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
func (_c Client) CreateTask(ctx context.Context, taskName string, initialState any) (flowID string, err error) { // MARKER: CreateTask
	_in := CreateTaskIn{TaskName: taskName, InitialState: initialState}
	_out := CreateTaskOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, CreateTask.Method, CreateTask.Route, &_in, &_out)
	return _out.FlowID, err // No trace
}

// --- Await ---

// AwaitIn are the input arguments of Await.
type AwaitIn struct { // MARKER: Await
	FlowID string `json:"flowID,omitzero"`
}

// AwaitOut are the output arguments of Await.
type AwaitOut struct { // MARKER: Await
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
}

// AwaitResponse packs the response of Await.
type AwaitResponse multicastResponse // MARKER: Await

// Get unpacks the return arguments of Await.
func (_res *AwaitResponse) Get() (status string, state map[string]any, err error) { // MARKER: Await
	_d := _res.data.(*AwaitOut)
	return _d.Status, _d.State, _res.err
}

/*
Await blocks until the flow stops (i.e. is no longer created, pending, or running),
then returns the status and state. Returns empty status and nil state on timeout.
*/
func (_c MulticastClient) Await(ctx context.Context, flowID string) iter.Seq[*AwaitResponse] { // MARKER: Await
	_in := AwaitIn{FlowID: flowID}
	_out := AwaitOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Await.Method, Await.Route, &_in, &_out)
	return func(yield func(*AwaitResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*AwaitResponse)(_r)) {
				return
			}
		}
	}
}

/*
Await blocks until the flow stops (i.e. is no longer created, pending, or running),
then returns the status and state. Returns empty status and nil state on timeout.
*/
func (_c Client) Await(ctx context.Context, flowID string) (status string, state map[string]any, err error) { // MARKER: Await
	_in := AwaitIn{FlowID: flowID}
	_out := AwaitOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Await.Method, Await.Route, &_in, &_out)
	return _out.Status, _out.State, err // No trace
}

// --- NotifyStatusChange ---

// NotifyStatusChangeIn are the input arguments of NotifyStatusChange.
type NotifyStatusChangeIn struct { // MARKER: NotifyStatusChange
	FlowID string `json:"flowID,omitzero"`
	Status string `json:"status,omitzero"`
}

// NotifyStatusChangeOut are the output arguments of NotifyStatusChange.
type NotifyStatusChangeOut struct { // MARKER: NotifyStatusChange
}

/*
NotifyStatusChange is an internal multicast signal fired when a flow's status changes,
to wake up all replicas that may be holding Await requests.
*/
func (_c MulticastClient) NotifyStatusChange(ctx context.Context, flowID string, status string) iter.Seq[*multicastResponse] { // MARKER: NotifyStatusChange
	_in := NotifyStatusChangeIn{FlowID: flowID, Status: status}
	_out := NotifyStatusChangeOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, NotifyStatusChange.Method, NotifyStatusChange.Route, &_in, &_out)
}

// --- OnFlowStopped event ---

// OnFlowStoppedIn are the input arguments of OnFlowStopped.
type OnFlowStoppedIn struct { // MARKER: OnFlowStopped
	FlowID   string         `json:"flowID,omitzero"`
	Status   string         `json:"status,omitzero"`
	Snapshot map[string]any `json:"snapshot,omitzero"`
}

// OnFlowStoppedOut are the output arguments of OnFlowStopped.
type OnFlowStoppedOut struct { // MARKER: OnFlowStopped
}

// OnFlowStoppedResponse packs the response of OnFlowStopped.
type OnFlowStoppedResponse multicastResponse // MARKER: OnFlowStopped

// Get unpacks the return arguments of OnFlowStopped.
func (_res *OnFlowStoppedResponse) Get() (err error) { // MARKER: OnFlowStopped
	return _res.err
}

/*
OnFlowStopped is triggered when a flow reaches a terminal status (completed, failed, cancelled).
This is a targeted event - it is delivered only to the hostname specified via StartNotify.
Subscribe with ForHost(svc.Hostname()) to receive notifications for flows you started.
*/
func (_c MulticastTrigger) OnFlowStopped(ctx context.Context, flowID string, status string, snapshot map[string]any) iter.Seq[*OnFlowStoppedResponse] { // MARKER: OnFlowStopped
	_in := OnFlowStoppedIn{FlowID: flowID, Status: status, Snapshot: snapshot}
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
OnFlowStopped is triggered when a flow reaches a terminal status (completed, failed, cancelled).
This is a targeted event - it is delivered only to the hostname specified via StartNotify.
Subscribe with ForHost(svc.Hostname()) to receive notifications for flows you started.
*/
func (c Hook) OnFlowStopped(handler func(ctx context.Context, flowID string, status string, snapshot map[string]any) (err error)) (unsub func() error, err error) { // MARKER: OnFlowStopped
	doOnFlowStopped := func(w http.ResponseWriter, r *http.Request) error {
		var in OnFlowStoppedIn
		var out OnFlowStoppedOut
		err = marshalFunction(w, r, OnFlowStopped.Route, &in, &out, func(_ any, _ any) error {
			err = handler(r.Context(), in.FlowID, in.Status, in.Snapshot)
			return err
		})
		return err // No trace
	}
	path := httpx.JoinHostAndPath(c.host, OnFlowStopped.Route)
	unsub, err = c.svc.Subscribe(OnFlowStopped.Method, path, doOnFlowStopped, c.opts...)
	return unsub, errors.Trace(err)
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests. Production code should use
// the foreman Client to create and start flows asynchronously.
type Executor struct {
	svc     service.Publisher
	host    string
	opts    []pub.Option
	inFlow  *workflow.Flow
	outFlow *workflow.Flow
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow}
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

// --- BreakBefore ---

// BreakBeforeIn are the input arguments of BreakBefore.
type BreakBeforeIn struct { // MARKER: BreakBefore
	FlowID   string `json:"flowID,omitzero"`
	TaskName string `json:"taskName,omitzero"`
	Enabled  bool   `json:"enabled,omitzero"`
}

// BreakBeforeOut are the output arguments of BreakBefore.
type BreakBeforeOut struct { // MARKER: BreakBefore
}

/*
BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
*/
func (_c MulticastClient) BreakBefore(ctx context.Context, flowID string, taskName string, enabled bool) iter.Seq[*multicastResponse] { // MARKER: BreakBefore
	_in := BreakBeforeIn{FlowID: flowID, TaskName: taskName, Enabled: enabled}
	_out := BreakBeforeOut{}
	return marshalPublish(ctx, _c.svc, _c.opts, _c.host, BreakBefore.Method, BreakBefore.Route, &_in, &_out)
}

/*
BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
*/
func (_c Client) BreakBefore(ctx context.Context, flowID string, taskName string, enabled bool) (err error) { // MARKER: BreakBefore
	_in := BreakBeforeIn{FlowID: flowID, TaskName: taskName, Enabled: enabled}
	_out := BreakBeforeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BreakBefore.Method, BreakBefore.Route, &_in, &_out)
	return err // No trace
}

// --- Run ---

// RunIn are the input arguments of Run.
type RunIn struct { // MARKER: Run
	WorkflowName string `json:"workflowName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
}

// RunOut are the output arguments of Run.
type RunOut struct { // MARKER: Run
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
}

// RunResponse packs the response of Run.
type RunResponse multicastResponse // MARKER: Run

// Get unpacks the return arguments of Run.
func (_res *RunResponse) Get() (status string, state map[string]any, err error) { // MARKER: Run
	_d := _res.data.(*RunOut)
	return _d.Status, _d.State, _res.err
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal status and state.
*/
func (_c MulticastClient) Run(ctx context.Context, workflowName string, initialState any) iter.Seq[*RunResponse] { // MARKER: Run
	_in := RunIn{WorkflowName: workflowName, InitialState: initialState}
	_out := RunOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Run.Method, Run.Route, &_in, &_out)
	return func(yield func(*RunResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*RunResponse)(_r)) {
				return
			}
		}
	}
}

/*
Run creates a new flow, starts it, and blocks until it stops. Returns the terminal status and state.
*/
func (_c Client) Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) { // MARKER: Run
	_in := RunIn{WorkflowName: workflowName, InitialState: initialState}
	_out := RunOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Run.Method, Run.Route, &_in, &_out)
	return _out.Status, _out.State, err // No trace
}

// ContinueIn are the input arguments of Continue.
type ContinueIn struct { // MARKER: Continue
	ThreadKey       string `json:"threadKey,omitzero"`
	AdditionalState any    `json:"additionalState,omitzero"`
}

// ContinueOut are the output arguments of Continue.
type ContinueOut struct { // MARKER: Continue
	NewFlowKey string `json:"newFlowKey,omitzero"`
}

// ContinueResponse packs the response of Continue.
type ContinueResponse multicastResponse // MARKER: Continue

// Get unpacks the return arguments of Continue.
func (_res *ContinueResponse) Get() (newFlowKey string, err error) { // MARKER: Continue
	_d := _res.data.(*ContinueOut)
	return _d.NewFlowKey, _res.err
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state.
The threadKey can be any flowKey belonging to the thread. The new flow belongs to the same thread
and is returned in created status. It is intended for multi-turn workflows where outputs feed back as inputs.
*/
func (_c MulticastClient) Continue(ctx context.Context, threadKey string, additionalState any) iter.Seq[*ContinueResponse] { // MARKER: Continue
	_in := ContinueIn{ThreadKey: threadKey, AdditionalState: additionalState}
	_out := ContinueOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Continue.Method, Continue.Route, &_in, &_out)
	return func(yield func(*ContinueResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ContinueResponse)(_r)) {
				return
			}
		}
	}
}

/*
Continue creates a new flow from the latest completed flow in a thread, merged with additional state.
The threadKey can be any flowKey belonging to the thread. The new flow belongs to the same thread
and is returned in created status. It is intended for multi-turn workflows where outputs feed back as inputs.
*/
func (_c Client) Continue(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error) { // MARKER: Continue
	_in := ContinueIn{ThreadKey: threadKey, AdditionalState: additionalState}
	_out := ContinueOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Continue.Method, Continue.Route, &_in, &_out)
	return _out.NewFlowKey, err // No trace
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

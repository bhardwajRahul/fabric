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

package foreman

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/coreservices/foreman/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ foremanapi.Client
	_ *workflow.Flow
)

const (
	Hostname = foremanapi.Hostname
	Version  = 9
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, workflowName string, initialState any) (flowKey string, err error)                   // MARKER: Create
	Start(ctx context.Context, flowKey string) (err error)                                                           // MARKER: Start
	StartNotify(ctx context.Context, flowKey string, notifyHostname string) (err error)                              // MARKER: StartNotify
	Snapshot(ctx context.Context, flowKey string) (status string, state map[string]any, err error)                   // MARKER: Snapshot
	Resume(ctx context.Context, flowKey string, resumeData any) (err error)                                          // MARKER: Resume
	Fork(ctx context.Context, stepKey string, stateOverrides any) (newFlowKey string, err error)                     // MARKER: Fork
	Cancel(ctx context.Context, flowKey string) (err error)                                                          // MARKER: Cancel
	History(ctx context.Context, flowKey string) (steps []foremanapi.FlowStep, err error)                            // MARKER: History
	Retry(ctx context.Context, flowKey string) (err error)                                                           // MARKER: Retry
	List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, err error)                    // MARKER: List
	CreateTask(ctx context.Context, taskName string, initialState any) (flowKey string, err error)                   // MARKER: CreateTask
	Enqueue(ctx context.Context, shard int, stepID int) (err error)                                                  // MARKER: Enqueue
	Await(ctx context.Context, flowKey string) (status string, state map[string]any, err error)                      // MARKER: Await
	NotifyStatusChange(ctx context.Context, flowKey string, status string) (err error)                               // MARKER: NotifyStatusChange
	BreakBefore(ctx context.Context, flowKey string, taskName string, enabled bool) (err error)                      // MARKER: BreakBefore
	Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error) // MARKER: Run
	Continue(ctx context.Context, threadKey string, additionalState any) (newFlowKey string, err error)              // MARKER: Continue
	HistoryMermaid(w http.ResponseWriter, r *http.Request) (err error)                                               // MARKER: HistoryMermaid
	PurgeExpiredFlows(ctx context.Context) (err error)                                                               // MARKER: PurgeExpiredFlows
	OnChangedNumShards(ctx context.Context) (err error)                                                              // MARKER: NumShards
	OnObserveQueueDepth(ctx context.Context) (err error)                                                             // MARKER: QueueDepth
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`Foreman orchestrates agentic workflow execution.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe( // MARKER: Create
		"Create", svc.doCreate,
		sub.At(foremanapi.Create.Method, foremanapi.Create.Route),
		sub.Description(`Create creates a new flow for a workflow without starting it.`),
		sub.Function(foremanapi.CreateIn{}, foremanapi.CreateOut{}),
	)
	svc.Subscribe( // MARKER: Start
		"Start", svc.doStart,
		sub.At(foremanapi.Start.Method, foremanapi.Start.Route),
		sub.Description(`Start transitions a created flow to running and enqueues it for execution.`),
		sub.Function(foremanapi.StartIn{}, foremanapi.StartOut{}),
	)
	svc.Subscribe( // MARKER: StartNotify
		"StartNotify", svc.doStartNotify,
		sub.At(foremanapi.StartNotify.Method, foremanapi.StartNotify.Route),
		sub.Description(`StartNotify transitions a created flow to running with status change notifications sent to the given hostname.`),
		sub.Function(foremanapi.StartNotifyIn{}, foremanapi.StartNotifyOut{}),
	)
	svc.Subscribe( // MARKER: Snapshot
		"Snapshot", svc.doSnapshot,
		sub.At(foremanapi.Snapshot.Method, foremanapi.Snapshot.Route),
		sub.Description(`Snapshot returns the current status and state of a flow.`),
		sub.Function(foremanapi.SnapshotIn{}, foremanapi.SnapshotOut{}),
	)
	svc.Subscribe( // MARKER: Resume
		"Resume", svc.doResume,
		sub.At(foremanapi.Resume.Method, foremanapi.Resume.Route),
		sub.Description(`Resume resumes an interrupted flow by merging resumeData into the leaf step's state and re-enqueuing it for execution.`),
		sub.Function(foremanapi.ResumeIn{}, foremanapi.ResumeOut{}),
	)
	svc.Subscribe( // MARKER: Fork
		"Fork", svc.doFork,
		sub.At(foremanapi.Fork.Method, foremanapi.Fork.Route),
		sub.Description(`Fork creates a new flow from an existing step's checkpoint.`),
		sub.Function(foremanapi.ForkIn{}, foremanapi.ForkOut{}),
	)
	svc.Subscribe( // MARKER: Cancel
		"Cancel", svc.doCancel,
		sub.At(foremanapi.Cancel.Method, foremanapi.Cancel.Route),
		sub.Description(`Cancel cancels a flow that is not yet in a terminal status.`),
		sub.Function(foremanapi.CancelIn{}, foremanapi.CancelOut{}),
	)
	svc.Subscribe( // MARKER: History
		"History", svc.doHistory,
		sub.At(foremanapi.History.Method, foremanapi.History.Route),
		sub.Description(`History returns the step-by-step execution history of a flow.`),
		sub.Function(foremanapi.HistoryIn{}, foremanapi.HistoryOut{}),
	)
	svc.Subscribe( // MARKER: Retry
		"Retry", svc.doRetry,
		sub.At(foremanapi.Retry.Method, foremanapi.Retry.Route),
		sub.Description(`Retry re-executes the last failed step of a flow.`),
		sub.Function(foremanapi.RetryIn{}, foremanapi.RetryOut{}),
	)
	svc.Subscribe( // MARKER: List
		"List", svc.doList,
		sub.At(foremanapi.List.Method, foremanapi.List.Route),
		sub.Description(`List queries flows by status or workflow name. Results are ordered newest first. Set CursorFlowKey in the query to paginate.`),
		sub.Function(foremanapi.ListIn{}, foremanapi.ListOut{}),
	)
	svc.Subscribe( // MARKER: CreateTask
		"CreateTask", svc.doCreateTask,
		sub.At(foremanapi.CreateTask.Method, foremanapi.CreateTask.Route),
		sub.Description(`CreateTask creates a flow that executes a single task and then terminates, without starting it.`),
		sub.Function(foremanapi.CreateTaskIn{}, foremanapi.CreateTaskOut{}),
	)
	svc.Subscribe( // MARKER: Enqueue
		"Enqueue", svc.doEnqueue,
		sub.At(foremanapi.Enqueue.Method, foremanapi.Enqueue.Route),
		sub.Description(`Enqueue adds a step to the local work queue for processing.`),
		sub.Function(foremanapi.EnqueueIn{}, foremanapi.EnqueueOut{}),
	)
	svc.Subscribe( // MARKER: Await
		"Await", svc.doAwait,
		sub.At(foremanapi.Await.Method, foremanapi.Await.Route),
		sub.Description(`Await blocks until the flow stops (i.e. is no longer created, pending, or running), then returns the status and snapshot.`),
		sub.Function(foremanapi.AwaitIn{}, foremanapi.AwaitOut{}),
	)
	svc.Subscribe( // MARKER: NotifyStatusChange
		"NotifyStatusChange", svc.doNotifyStatusChange,
		sub.At(foremanapi.NotifyStatusChange.Method, foremanapi.NotifyStatusChange.Route),
		sub.Description(`NotifyStatusChange is an internal multicast signal to wake up Await callers across replicas.`),
		sub.Function(foremanapi.NotifyStatusChangeIn{}, foremanapi.NotifyStatusChangeOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: BreakBefore
		"BreakBefore", svc.doBreakBefore,
		sub.At(foremanapi.BreakBefore.Method, foremanapi.BreakBefore.Route),
		sub.Description(`BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.`),
		sub.Function(foremanapi.BreakBeforeIn{}, foremanapi.BreakBeforeOut{}),
	)
	svc.Subscribe( // MARKER: Run
		"Run", svc.doRun,
		sub.At(foremanapi.Run.Method, foremanapi.Run.Route),
		sub.Description(`Run creates a new flow, starts it, and blocks until it stops. Returns the terminal status and state.`),
		sub.Function(foremanapi.RunIn{}, foremanapi.RunOut{}),
	)
	svc.Subscribe( // MARKER: Continue
		"Continue", svc.doContinue,
		sub.At(foremanapi.Continue.Method, foremanapi.Continue.Route),
		sub.Description(`Continue creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers. The threadKey can be any flowKey belonging to the thread. The new flow belongs to the same thread and is returned in created status.`),
		sub.Function(foremanapi.ContinueIn{}, foremanapi.ContinueOut{}),
	)

	// HINT: Add web endpoints here
	svc.Subscribe( // MARKER: HistoryMermaid
		"HistoryMermaid", svc.HistoryMermaid,
		sub.At(foremanapi.HistoryMermaid.Method, foremanapi.HistoryMermaid.Route),
		sub.Description(`HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.`),
		sub.Web(),
	)

	// HINT: Add metrics here
	svc.DescribeCounter("microbus_flows_started_total", "FlowsStarted counts the number of flows that have been started.")                               // MARKER: FlowsStarted
	svc.DescribeCounter("microbus_flows_terminated_total", "FlowsTerminated counts the number of flows that have reached a terminal status.")            // MARKER: FlowsTerminated
	svc.DescribeCounter("microbus_steps_executed_total", "StepsExecuted counts the number of steps that have been executed.")                            // MARKER: StepsExecuted
	svc.DescribeGauge("microbus_queue_depth", "QueueDepth records the number of steps waiting in the local worker queue.")                               // MARKER: QueueDepth
	svc.DescribeCounter("microbus_steps_recovered_total", "StepsRecovered counts the number of steps recovered by pollPendingSteps after lease expiry.") // MARKER: StepsRecovered

	// HINT: Add tickers here
	svc.StartTicker("PurgeExpiredFlows", 24*time.Hour, svc.PurgeExpiredFlows) // MARKER: PurgeExpiredFlows

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: SQLDataSourceName
		"SQLDataSourceName",
		cfg.Description(`SQLDataSourceName is the connection string of the SQL database.`),
		cfg.Secret(),
	)
	svc.DefineConfig( // MARKER: Workers
		"Workers",
		cfg.Description(`Workers is the number of concurrent workers that process flow steps.`),
		cfg.DefaultValue(`4`),
		cfg.Validation(`int [1,64]`),
	)
	svc.DefineConfig( // MARKER: RetentionDays
		"RetentionDays",
		cfg.Description(`RetentionDays is the number of days to retain terminated flows and their steps. Set to 0 to disable purging.`),
		cfg.DefaultValue(`0`),
		cfg.Validation(`int [0,]`),
	)
	svc.DefineConfig( // MARKER: DefaultTimeBudget
		"DefaultTimeBudget",
		cfg.Description(`DefaultTimeBudget is the default execution timeout for task steps when the graph does not specify a per-task time budget.`),
		cfg.DefaultValue(`2m`),
		cfg.Validation(`dur [1s,15m]`),
	)
	svc.DefineConfig( // MARKER: NumShards
		"NumShards",
		cfg.Description(`NumShards is the number of database shards. Each shard is a separate database instance. Shards can be added dynamically but never removed.`),
		cfg.DefaultValue(`1`),
		cfg.Validation(`int [1,]`),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
		// HINT: Call JIT observers to record the metric here
		func() (err error) { return svc.OnObserveQueueDepth(ctx) }, // MARKER: QueueDepth
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	if changed("NumShards") { // MARKER: NumShards
		if err := svc.OnChangedNumShards(ctx); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
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

// doCreate handles marshaling for the Create function.
func (svc *Intermediate) doCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Create
	var in foremanapi.CreateIn
	var out foremanapi.CreateOut
	err = marshalFunction(w, r, foremanapi.Create.Route, &in, &out, func(_ any, _ any) error {
		out.FlowKey, err = svc.Create(r.Context(), in.WorkflowName, in.InitialState)
		return err
	})
	return err // No trace
}

// doStart handles marshaling for the Start function.
func (svc *Intermediate) doStart(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Start
	var in foremanapi.StartIn
	var out foremanapi.StartOut
	err = marshalFunction(w, r, foremanapi.Start.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Start(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doStartNotify handles marshaling for the StartNotify function.
func (svc *Intermediate) doStartNotify(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: StartNotify
	var in foremanapi.StartNotifyIn
	var out foremanapi.StartNotifyOut
	err = marshalFunction(w, r, foremanapi.StartNotify.Route, &in, &out, func(_ any, _ any) error {
		err = svc.StartNotify(r.Context(), in.FlowKey, in.NotifyHostname)
		return err
	})
	return err // No trace
}

// doSnapshot handles marshaling for the Snapshot function.
func (svc *Intermediate) doSnapshot(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Snapshot
	var in foremanapi.SnapshotIn
	var out foremanapi.SnapshotOut
	err = marshalFunction(w, r, foremanapi.Snapshot.Route, &in, &out, func(_ any, _ any) error {
		out.Status, out.State, err = svc.Snapshot(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doResume handles marshaling for the Resume function.
func (svc *Intermediate) doResume(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Resume
	var in foremanapi.ResumeIn
	var out foremanapi.ResumeOut
	err = marshalFunction(w, r, foremanapi.Resume.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Resume(r.Context(), in.FlowKey, in.ResumeData)
		return err
	})
	return err // No trace
}

// doFork handles marshaling for the Fork function.
func (svc *Intermediate) doFork(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Fork
	var in foremanapi.ForkIn
	var out foremanapi.ForkOut
	err = marshalFunction(w, r, foremanapi.Fork.Route, &in, &out, func(_ any, _ any) error {
		out.NewFlowKey, err = svc.Fork(r.Context(), in.StepKey, in.StateOverrides)
		return err
	})
	return err // No trace
}

// doCancel handles marshaling for the Cancel function.
func (svc *Intermediate) doCancel(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Cancel
	var in foremanapi.CancelIn
	var out foremanapi.CancelOut
	err = marshalFunction(w, r, foremanapi.Cancel.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Cancel(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doHistory handles marshaling for the History function.
func (svc *Intermediate) doHistory(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: History
	var in foremanapi.HistoryIn
	var out foremanapi.HistoryOut
	err = marshalFunction(w, r, foremanapi.History.Route, &in, &out, func(_ any, _ any) error {
		out.Steps, err = svc.History(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doRetry handles marshaling for the Retry function.
func (svc *Intermediate) doRetry(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Retry
	var in foremanapi.RetryIn
	var out foremanapi.RetryOut
	err = marshalFunction(w, r, foremanapi.Retry.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Retry(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doList handles marshaling for the List function.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	var in foremanapi.ListIn
	var out foremanapi.ListOut
	err = marshalFunction(w, r, foremanapi.List.Route, &in, &out, func(_ any, _ any) error {
		out.Flows, err = svc.List(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doCreateTask handles marshaling for the CreateTask function.
func (svc *Intermediate) doCreateTask(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CreateTask
	var in foremanapi.CreateTaskIn
	var out foremanapi.CreateTaskOut
	err = marshalFunction(w, r, foremanapi.CreateTask.Route, &in, &out, func(_ any, _ any) error {
		out.FlowKey, err = svc.CreateTask(r.Context(), in.TaskName, in.InitialState)
		return err
	})
	return err // No trace
}

// doEnqueue handles marshaling for the Enqueue function.
func (svc *Intermediate) doEnqueue(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Enqueue
	var in foremanapi.EnqueueIn
	var out foremanapi.EnqueueOut
	err = marshalFunction(w, r, foremanapi.Enqueue.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Enqueue(r.Context(), in.Shard, in.StepID)
		return err
	})
	return err // No trace
}

// doAwait handles marshaling for the Await function.
func (svc *Intermediate) doAwait(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Await
	var in foremanapi.AwaitIn
	var out foremanapi.AwaitOut
	err = marshalFunction(w, r, foremanapi.Await.Route, &in, &out, func(_ any, _ any) error {
		out.Status, out.State, err = svc.Await(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doNotifyStatusChange handles marshaling for the NotifyStatusChange function.
func (svc *Intermediate) doNotifyStatusChange(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NotifyStatusChange
	var in foremanapi.NotifyStatusChangeIn
	var out foremanapi.NotifyStatusChangeOut
	err = marshalFunction(w, r, foremanapi.NotifyStatusChange.Route, &in, &out, func(_ any, _ any) error {
		err = svc.NotifyStatusChange(r.Context(), in.FlowKey, in.Status)
		return err
	})
	return err // No trace
}

// doBreakBefore handles marshaling for the BreakBefore function.
func (svc *Intermediate) doBreakBefore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BreakBefore
	var in foremanapi.BreakBeforeIn
	var out foremanapi.BreakBeforeOut
	err = marshalFunction(w, r, foremanapi.BreakBefore.Route, &in, &out, func(_ any, _ any) error {
		err = svc.BreakBefore(r.Context(), in.FlowKey, in.TaskName, in.Enabled)
		return err
	})
	return err // No trace
}

// doRun handles marshaling for the Run function.
func (svc *Intermediate) doRun(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Run
	var in foremanapi.RunIn
	var out foremanapi.RunOut
	err = marshalFunction(w, r, foremanapi.Run.Route, &in, &out, func(_ any, _ any) error {
		out.Status, out.State, err = svc.Run(r.Context(), in.WorkflowName, in.InitialState)
		return err
	})
	return err // No trace
}

// doContinue handles marshaling for the Continue function.
func (svc *Intermediate) doContinue(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Continue
	var in foremanapi.ContinueIn
	var out foremanapi.ContinueOut
	err = marshalFunction(w, r, foremanapi.Continue.Route, &in, &out, func(_ any, _ any) error {
		out.NewFlowKey, err = svc.Continue(r.Context(), in.ThreadKey, in.AdditionalState)
		return err
	})
	return err // No trace
}

/*
SQLDataSourceName is the connection string of the SQL database.
*/
func (svc *Intermediate) SQLDataSourceName() (value string) { // MARKER: SQLDataSourceName
	return svc.Config("SQLDataSourceName")
}

/*
SetSQLDataSourceName sets the value of the configuration property.
*/
func (svc *Intermediate) SetSQLDataSourceName(value string) (err error) { // MARKER: SQLDataSourceName
	return svc.SetConfig("SQLDataSourceName", value)
}

/*
Workers is the number of concurrent workers that process flow steps.
*/
func (svc *Intermediate) Workers() (workers int) { // MARKER: Workers
	_val := svc.Config("Workers")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetWorkers sets the value of the configuration property.
*/
func (svc *Intermediate) SetWorkers(workers int) (err error) { // MARKER: Workers
	return svc.SetConfig("Workers", strconv.Itoa(workers))
}

/*
RetentionDays is the number of days to retain terminated flows and their steps. Set to 0 to disable purging.
*/
func (svc *Intermediate) RetentionDays() (retentionDays int) { // MARKER: RetentionDays
	_val := svc.Config("RetentionDays")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetRetentionDays sets the value of the configuration property.
*/
func (svc *Intermediate) SetRetentionDays(retentionDays int) (err error) { // MARKER: RetentionDays
	return svc.SetConfig("RetentionDays", strconv.Itoa(retentionDays))
}

/*
DefaultTimeBudget is the default execution timeout for task steps when the graph does not specify a per-task time budget.
*/
func (svc *Intermediate) DefaultTimeBudget() (budget time.Duration) { // MARKER: DefaultTimeBudget
	_val := svc.Config("DefaultTimeBudget")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetDefaultTimeBudget sets the value of the configuration property.
*/
func (svc *Intermediate) SetDefaultTimeBudget(budget time.Duration) (err error) { // MARKER: DefaultTimeBudget
	return svc.SetConfig("DefaultTimeBudget", budget.String())
}

/*
NumShards is the number of database shards. Each shard is a separate database instance. Shards can be added dynamically but never removed.
*/
func (svc *Intermediate) NumShards() (numShards int) { // MARKER: NumShards
	_val := svc.Config("NumShards")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetNumShards sets the value of the configuration property.
*/
func (svc *Intermediate) SetNumShards(numShards int) (err error) { // MARKER: NumShards
	return svc.SetConfig("NumShards", strconv.Itoa(numShards))
}

/*
IncrementFlowsStarted counts the number of flows that have been started.
*/
func (svc *Intermediate) IncrementFlowsStarted(ctx context.Context, value int, workflowName string) (err error) { // MARKER: FlowsStarted
	return svc.IncrementCounter(ctx, "microbus_flows_started_total", float64(value),
		"workflow_name", utils.AnyToString(workflowName),
	)
}

/*
IncrementFlowsTerminated counts the number of flows that have reached a terminal status.
*/
func (svc *Intermediate) IncrementFlowsTerminated(ctx context.Context, value int, workflowName string, status string) (err error) { // MARKER: FlowsTerminated
	return svc.IncrementCounter(ctx, "microbus_flows_terminated_total", float64(value),
		"workflow_name", utils.AnyToString(workflowName),
		"status", utils.AnyToString(status),
	)
}

/*
IncrementStepsExecuted counts the number of steps that have been executed.
*/
func (svc *Intermediate) IncrementStepsExecuted(ctx context.Context, value int, task string, status string) (err error) { // MARKER: StepsExecuted
	return svc.IncrementCounter(ctx, "microbus_steps_executed_total", float64(value),
		"task", utils.AnyToString(task),
		"status", utils.AnyToString(status),
	)
}

/*
RecordQueueDepth records the number of steps waiting in the local worker queue.
*/
func (svc *Intermediate) RecordQueueDepth(ctx context.Context, value int) (err error) { // MARKER: QueueDepth
	return svc.RecordGauge(ctx, "microbus_queue_depth", float64(value))
}

/*
IncrementStepsRecovered counts the number of steps recovered by pollPendingSteps after lease expiry.
*/
func (svc *Intermediate) IncrementStepsRecovered(ctx context.Context, value int) (err error) { // MARKER: StepsRecovered
	return svc.IncrementCounter(ctx, "microbus_steps_recovered_total", float64(value))
}

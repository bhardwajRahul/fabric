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
	Version  = 51
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (flowKey string, err error)             // MARKER: Create
	Start(ctx context.Context, flowKey string) (err error)                                                                                 // MARKER: Start
	StartNotify(ctx context.Context, flowKey string, notifyHostname string) (err error)                                                    // MARKER: StartNotify
	Snapshot(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error)                                               // MARKER: Snapshot
	Fingerprint(ctx context.Context, flowKey string) (fingerprint string, status string, err error)                                        // MARKER: Fingerprint
	Resume(ctx context.Context, flowKey string, resumeData any) (err error)                                                                // MARKER: Resume
	ResumeBreak(ctx context.Context, flowKey string, stateOverrides any) (err error)                                                       // MARKER: ResumeBreak
	Cancel(ctx context.Context, flowKey string, reason string) (err error)                                                                 // MARKER: Cancel
	Restart(ctx context.Context, flowKey string, stateOverrides any) (err error)                                                           // MARKER: Restart
	RestartFrom(ctx context.Context, stepKey string, stateOverrides any) (err error)                                                       // MARKER: RestartFrom
	History(ctx context.Context, flowKey string) (steps []foremanapi.FlowStep, err error)                                                  // MARKER: History
	Step(ctx context.Context, stepKey string) (step *foremanapi.FlowStep, err error)                                                       // MARKER: Step
	List(ctx context.Context, query foremanapi.Query) (flows []foremanapi.FlowSummary, nextCursor string, err error)                       // MARKER: List
	Delete(ctx context.Context, flowKey string) (err error)                                                                                // MARKER: Delete
	Purge(ctx context.Context, query foremanapi.Query) (deleted int, err error)                                                            // MARKER: Purge
	ShardInfo(ctx context.Context) (shards []foremanapi.ShardSummary, err error)                                                           // MARKER: ShardInfo
	CreateTask(ctx context.Context, taskName string, initialState any) (flowKey string, err error)                                         // MARKER: CreateTask
	Enqueue(ctx context.Context, shard int, stepID int) (err error)                                                                        // MARKER: Enqueue
	Await(ctx context.Context, flowKey string) (outcome *workflow.FlowOutcome, err error)                                                  // MARKER: Await
	NotifyStatusChange(ctx context.Context, flowKey string, status string) (err error)                                                     // MARKER: NotifyStatusChange
	BreakBefore(ctx context.Context, flowKey string, taskName string, enabled bool) (err error)                                            // MARKER: BreakBefore
	Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (outcome *workflow.FlowOutcome, err error) // MARKER: Run
	Continue(ctx context.Context, threadKey string, additionalState any, opts *workflow.FlowOptions) (newFlowKey string, err error)        // MARKER: Continue
	HistoryMermaid(w http.ResponseWriter, r *http.Request) (err error)                                                                     // MARKER: HistoryMermaid
	SyncValve(ctx context.Context, taskName string, wCong int, tCong time.Time) (err error)                                                // MARKER: SyncValve
	TripBreaker(ctx context.Context, taskName string) (err error)                                                                          // MARKER: TripBreaker
	OnChangedNumShards(ctx context.Context) (err error)                                                                                    // MARKER: NumShards
	OnObserveStepsQueueDepth(ctx context.Context) (err error)                                                                              // MARKER: StepsQueueDepth
	OnObserveStepsPending(ctx context.Context) (err error)                                                                                 // MARKER: StepsPending
	OnObserveStepsOldestPendingAgeSeconds(ctx context.Context) (err error)                                                                 // MARKER: StepsOldestPendingAgeSeconds
	OnObserveTaskRateLimit(ctx context.Context) (err error)                                                                                // MARKER: TaskRateLimit
	OnObserveTaskConcurrencyRunning(ctx context.Context) (err error)                                                                       // MARKER: TaskConcurrencyRunning
	OnObserveTaskBreakerState(ctx context.Context) (err error)                                                                             // MARKER: TaskBreakerState
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
	svc.Subscribe( // MARKER: Fingerprint
		"Fingerprint", svc.doFingerprint,
		sub.At(foremanapi.Fingerprint.Method, foremanapi.Fingerprint.Route),
		sub.Description(`Fingerprint returns a short opaque hash that changes when the flow's status, step count, or any step's updated_at changes — across the flow and any nested subgraph descendants. Cheap (one indexed query per descendant level); intended as the change-detection probe for long-polling watchers.`),
		sub.Function(foremanapi.FingerprintIn{}, foremanapi.FingerprintOut{}),
	)
	svc.Subscribe( // MARKER: Resume
		"Resume", svc.doResume,
		sub.At(foremanapi.Resume.Method, foremanapi.Resume.Route),
		sub.Description(`Resume continues an interrupted flow, delivering resumeData to the task that armed flow.Interrupt (recorded on the leaf step's resume_data column and returned by flow.Interrupt on re-entry). Fails with 409 if the flow is paused at a breakpoint rather than an interrupt.`),
		sub.Function(foremanapi.ResumeIn{}, foremanapi.ResumeOut{}),
	)
	svc.Subscribe( // MARKER: ResumeBreak
		"ResumeBreak", svc.doResumeBreak,
		sub.At(foremanapi.ResumeBreak.Method, foremanapi.ResumeBreak.Route),
		sub.Description(`ResumeBreak continues a flow paused at a breakpoint, merging stateOverrides into the leaf step's input state so the about-to-run task observes them. Fails with 409 if the flow is paused at an interrupt rather than a breakpoint.`),
		sub.Function(foremanapi.ResumeBreakIn{}, foremanapi.ResumeBreakOut{}),
	)
	svc.Subscribe( // MARKER: Cancel
		"Cancel", svc.doCancel,
		sub.At(foremanapi.Cancel.Method, foremanapi.Cancel.Route),
		sub.Description(`Cancel cancels a flow that is not yet in a terminal status.`),
		sub.Function(foremanapi.CancelIn{}, foremanapi.CancelOut{}),
	)
	svc.Subscribe( // MARKER: Restart
		"Restart", svc.doRestart,
		sub.At(foremanapi.Restart.Method, foremanapi.Restart.Route),
		sub.Description(`Restart wipes everything past a flow's entry step and resets the entry with overrides.`),
		sub.Function(foremanapi.RestartIn{}, foremanapi.RestartOut{}),
	)
	svc.Subscribe( // MARKER: RestartFrom
		"RestartFrom", svc.doRestartFrom,
		sub.At(foremanapi.RestartFrom.Method, foremanapi.RestartFrom.Route),
		sub.Description(`RestartFrom sweeps the DAG subtree below a chosen step and resets that step with overrides.`),
		sub.Function(foremanapi.RestartFromIn{}, foremanapi.RestartFromOut{}),
	)
	svc.Subscribe( // MARKER: History
		"History", svc.doHistory,
		sub.At(foremanapi.History.Method, foremanapi.History.Route),
		sub.Description(`History returns the step-by-step execution history of a flow.`),
		sub.Function(foremanapi.HistoryIn{}, foremanapi.HistoryOut{}),
	)
	svc.Subscribe( // MARKER: Step
		"Step", svc.doStep,
		sub.At(foremanapi.Step.Method, foremanapi.Step.Route),
		sub.Description(`Step returns the full detail of one execution step, including the state, changes and interrupt payload that History omits.`),
		sub.Function(foremanapi.StepIn{}, foremanapi.StepOut{}),
	)
	svc.Subscribe( // MARKER: List
		"List", svc.doList,
		sub.At(foremanapi.List.Method, foremanapi.List.Route),
		sub.Description(`List queries flows by status or workflow name. Set Query.Cursor to the previous call's NextCursor to paginate.`),
		sub.Function(foremanapi.ListIn{}, foremanapi.ListOut{}),
	)
	svc.Subscribe( // MARKER: Delete
		"Delete", svc.doDelete,
		sub.At(foremanapi.Delete.Method, foremanapi.Delete.Route),
		sub.Description(`Delete removes a flow and its steps from the database. The flow must not be running. Subgraph, fork, and thread lineage references become dangling.`),
		sub.Function(foremanapi.DeleteIn{}, foremanapi.DeleteOut{}),
	)
	svc.Subscribe( // MARKER: Purge
		"Purge", svc.doPurge,
		sub.At(foremanapi.Purge.Method, foremanapi.Purge.Route),
		sub.Description(`Purge deletes flows matching the query, except those currently running. Capped at 10000 flows per call.`),
		sub.Function(foremanapi.PurgeIn{}, foremanapi.PurgeOut{}),
	)
	svc.Subscribe( // MARKER: ShardInfo
		"ShardInfo", svc.doShardInfo,
		sub.At(foremanapi.ShardInfo.Method, foremanapi.ShardInfo.Route),
		sub.Description(`ShardInfo returns per-shard health (latency, row counts, error) for every database shard.`),
		sub.Function(foremanapi.ShardInfoIn{}, foremanapi.ShardInfoOut{}),
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
		sub.Description(`Enqueue rings the work doorbell, signalling that a step is pending, so the receiving replica refills or flushes-and-refills its candidate cache.`),
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
	svc.Subscribe( // MARKER: SyncValve
		"SyncValve", svc.doSyncValve,
		sub.At(foremanapi.SyncValve.Method, foremanapi.SyncValve.Route),
		sub.Description(`SyncValve broadcasts a per-task congestion point so every replica's admission limit converges from the same anchor.`),
		sub.Function(foremanapi.SyncValveIn{}, foremanapi.SyncValveOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: TripBreaker
		"TripBreaker", svc.doTripBreaker,
		sub.At(foremanapi.TripBreaker.Method, foremanapi.TripBreaker.Route),
		sub.Description(`TripBreaker fires a per-task breaker trip to peer foreman replicas. Each peer stamps its own clock on receipt; closures are not gossiped.`),
		sub.Function(foremanapi.TripBreakerIn{}, foremanapi.TripBreakerOut{}),
		sub.NoQueue(),
	)

	// HINT: Add web endpoints here
	svc.Subscribe( // MARKER: HistoryMermaid
		"HistoryMermaid", svc.HistoryMermaid,
		sub.At(foremanapi.HistoryMermaid.Method, foremanapi.HistoryMermaid.Route),
		sub.Description(`HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.`),
		sub.Web(),
	)

	// HINT: Add metrics here
	svc.DescribeCounter("microbus_flows_started_total", "FlowsStarted counts the number of flows that have been started.")                                                                                                                             // MARKER: FlowsStarted
	svc.DescribeCounter("microbus_flows_terminated_total", "FlowsTerminated counts the number of flows that have reached a terminal status.")                                                                                                          // MARKER: FlowsTerminated
	svc.DescribeCounter("microbus_steps_executed_total", "StepsExecuted counts the number of steps that have been executed.")                                                                                                                          // MARKER: StepsExecuted
	svc.DescribeGauge("microbus_steps_queue_depth", "StepsQueueDepth records the number of steps waiting in the local worker queue.")                                                                                                                  // MARKER: StepsQueueDepth
	svc.DescribeCounter("microbus_steps_recovered_total", "StepsRecovered counts the number of steps recovered by pollPendingSteps after lease expiry.")                                                                                               // MARKER: StepsRecovered
	svc.DescribeGauge("microbus_steps_pending", "StepsPending records the number of due pending steps in each priority band.")                                                                                                                         // MARKER: StepsPending
	svc.DescribeGauge("microbus_steps_oldest_pending_age_seconds", "StepsOldestPendingAgeSeconds records the age in seconds of the oldest due pending step in each priority band.")                                                                    // MARKER: StepsOldestPendingAgeSeconds
	svc.DescribeGauge("microbus_steps_fairness_keys", "StepsFairnessKeys records the number of distinct fairness keys in the most recent refill selection at the given priority band.")                                                                // MARKER: StepsFairnessKeys
	svc.DescribeCounter("microbus_steps_skipped_saturated_total", "StepsSkippedSaturated counts the number of step admissions skipped because the task was at its current rate-limit ceiling.")                                                        // MARKER: StepsSkippedSaturated
	svc.DescribeGauge("microbus_task_rate_limit", "TaskRateLimit records the current adaptive per-task dispatch-rate ceiling (ops/sec) derived from the gossiped congestion point.")                                                                   // MARKER: TaskRateLimit
	svc.DescribeGauge("microbus_task_concurrency_running", "TaskConcurrencyRunning records the cluster-wide number of running steps per task; pairs with TaskRateLimit on one panel.")                                                                 // MARKER: TaskConcurrencyRunning
	svc.DescribeCounter("microbus_task_rate_cuts_total", "TaskRateCuts counts the additive-decrease cuts to the per-task rate ceiling triggered by a 429 from a task dispatch.")                                                                       // MARKER: TaskRateCuts
	svc.DescribeCounter("microbus_task_breaker_trips_total", "TaskBreakerTrips counts the number of times a per-task circuit breaker transitioned to tripped, labelled by cause (ack_timeout for a 404 ack-timeout, overloaded for a 529).")           // MARKER: TaskBreakerTrips
	svc.DescribeCounter("microbus_task_breaker_probes_total", "TaskBreakerProbes counts probe attempts against a tripped per-task circuit breaker, labelled by outcome (success or failure) and the original trip cause (ack_timeout or overloaded).") // MARKER: TaskBreakerProbes
	svc.DescribeGauge("microbus_task_breaker_state", "TaskBreakerState records the current state of each per-task circuit breaker: 0 = closed (admitting), 1 = tripped (blocked).")                                                                    // MARKER: TaskBreakerState

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: SQLDataSourceName
		"SQLDataSourceName",
		cfg.Description(`SQLDataSourceName is the connection string of the SQL database.`),
		cfg.Secret(),
	)
	svc.DefineConfig( // MARKER: Workers
		"Workers",
		cfg.Description(`Workers is the number of concurrent workers that process flow steps.`),
		cfg.DefaultValue(`64`),
		cfg.Validation(`int [1,]`),
	)
	svc.DefineConfig( // MARKER: TimeBudget
		"TimeBudget",
		cfg.Description(`TimeBudget is the hard ceiling on the execution time of any task step. It is applied as the timeout on the task dispatch call; a task endpoint may declare a shorter budget of its own via sub.TimeBudget.`),
		cfg.DefaultValue(`2m`),
		cfg.Validation(`dur [1s,15m]`),
	)
	svc.DefineConfig( // MARKER: DefaultPriority
		"DefaultPriority",
		cfg.Description(`DefaultPriority is the priority assigned to a flow when the caller does not specify one. Priority is an integer >= 1, lower numbers run first.`),
		cfg.DefaultValue(`5`),
		cfg.Validation(`int [1,]`),
	)
	svc.DefineConfig( // MARKER: NumShards
		"NumShards",
		cfg.Description(`NumShards is the number of database shards. Each shard is a separate database instance. Shards can be added dynamically but never removed.`),
		cfg.DefaultValue(`1`),
		cfg.Validation(`int [1,]`),
	)
	svc.DefineConfig( // MARKER: SQLConnectionPool
		"SQLConnectionPool",
		cfg.Description(`SQLConnectionPool is the number of database connections kept open per shard.`),
		cfg.DefaultValue(`8`),
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
		func() (err error) { return svc.OnObserveStepsQueueDepth(ctx) },              // MARKER: StepsQueueDepth
		func() (err error) { return svc.OnObserveStepsPending(ctx) },                 // MARKER: StepsPending
		func() (err error) { return svc.OnObserveStepsOldestPendingAgeSeconds(ctx) }, // MARKER: StepsOldestPendingAgeSeconds
		func() (err error) { return svc.OnObserveTaskRateLimit(ctx) },                // MARKER: TaskRateLimit
		func() (err error) { return svc.OnObserveTaskConcurrencyRunning(ctx) },       // MARKER: TaskConcurrencyRunning
		func() (err error) { return svc.OnObserveTaskBreakerState(ctx) },             // MARKER: TaskBreakerState
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
		out.FlowKey, err = svc.Create(r.Context(), in.WorkflowName, in.InitialState, in.Opts)
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
		out.Outcome, err = svc.Snapshot(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doFingerprint handles marshaling for the Fingerprint function.
func (svc *Intermediate) doFingerprint(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Fingerprint
	var in foremanapi.FingerprintIn
	var out foremanapi.FingerprintOut
	err = marshalFunction(w, r, foremanapi.Fingerprint.Route, &in, &out, func(_ any, _ any) error {
		out.Fingerprint, out.Status, err = svc.Fingerprint(r.Context(), in.FlowKey)
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

// doResumeBreak handles marshaling for the ResumeBreak function.
func (svc *Intermediate) doResumeBreak(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ResumeBreak
	var in foremanapi.ResumeBreakIn
	var out foremanapi.ResumeBreakOut
	err = marshalFunction(w, r, foremanapi.ResumeBreak.Route, &in, &out, func(_ any, _ any) error {
		err = svc.ResumeBreak(r.Context(), in.FlowKey, in.StateOverrides)
		return err
	})
	return err // No trace
}

// doCancel handles marshaling for the Cancel function.
func (svc *Intermediate) doCancel(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Cancel
	var in foremanapi.CancelIn
	var out foremanapi.CancelOut
	err = marshalFunction(w, r, foremanapi.Cancel.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Cancel(r.Context(), in.FlowKey, in.Reason)
		return err
	})
	return err // No trace
}

// doRestart handles marshaling for the Restart function.
func (svc *Intermediate) doRestart(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Restart
	var in foremanapi.RestartIn
	var out foremanapi.RestartOut
	err = marshalFunction(w, r, foremanapi.Restart.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Restart(r.Context(), in.FlowKey, in.StateOverrides)
		return err
	})
	return err // No trace
}

// doRestartFrom handles marshaling for the RestartFrom function.
func (svc *Intermediate) doRestartFrom(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RestartFrom
	var in foremanapi.RestartFromIn
	var out foremanapi.RestartFromOut
	err = marshalFunction(w, r, foremanapi.RestartFrom.Route, &in, &out, func(_ any, _ any) error {
		err = svc.RestartFrom(r.Context(), in.StepKey, in.StateOverrides)
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

// doStep handles marshaling for the Step function.
func (svc *Intermediate) doStep(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Step
	var in foremanapi.StepIn
	var out foremanapi.StepOut
	err = marshalFunction(w, r, foremanapi.Step.Route, &in, &out, func(_ any, _ any) error {
		out.Step, err = svc.Step(r.Context(), in.StepKey)
		return err
	})
	return err // No trace
}

// doList handles marshaling for the List function.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	var in foremanapi.ListIn
	var out foremanapi.ListOut
	err = marshalFunction(w, r, foremanapi.List.Route, &in, &out, func(_ any, _ any) error {
		out.Flows, out.NextCursor, err = svc.List(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doDelete handles marshaling for the Delete function.
func (svc *Intermediate) doDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Delete
	var in foremanapi.DeleteIn
	var out foremanapi.DeleteOut
	err = marshalFunction(w, r, foremanapi.Delete.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Delete(r.Context(), in.FlowKey)
		return err
	})
	return err // No trace
}

// doPurge handles marshaling for the Purge function.
func (svc *Intermediate) doPurge(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Purge
	var in foremanapi.PurgeIn
	var out foremanapi.PurgeOut
	err = marshalFunction(w, r, foremanapi.Purge.Route, &in, &out, func(_ any, _ any) error {
		out.Deleted, err = svc.Purge(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doShardInfo handles marshaling for the ShardInfo function.
func (svc *Intermediate) doShardInfo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ShardInfo
	var in foremanapi.ShardInfoIn
	var out foremanapi.ShardInfoOut
	err = marshalFunction(w, r, foremanapi.ShardInfo.Route, &in, &out, func(_ any, _ any) error {
		out.Shards, err = svc.ShardInfo(r.Context())
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
		out.Outcome, err = svc.Await(r.Context(), in.FlowKey)
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
		out.Outcome, err = svc.Run(r.Context(), in.WorkflowName, in.InitialState, in.Opts)
		return err
	})
	return err // No trace
}

// doContinue handles marshaling for the Continue function.
func (svc *Intermediate) doContinue(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Continue
	var in foremanapi.ContinueIn
	var out foremanapi.ContinueOut
	err = marshalFunction(w, r, foremanapi.Continue.Route, &in, &out, func(_ any, _ any) error {
		out.NewFlowKey, err = svc.Continue(r.Context(), in.ThreadKey, in.AdditionalState, in.Opts)
		return err
	})
	return err // No trace
}

// doSyncValve handles marshaling for the SyncValve function.
func (svc *Intermediate) doSyncValve(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SyncValve
	var in foremanapi.SyncValveIn
	var out foremanapi.SyncValveOut
	err = marshalFunction(w, r, foremanapi.SyncValve.Route, &in, &out, func(_ any, _ any) error {
		err = svc.SyncValve(r.Context(), in.TaskName, in.WCong, in.TCong)
		return err
	})
	return err // No trace
}

// doTripBreaker handles marshaling for the TripBreaker function.
func (svc *Intermediate) doTripBreaker(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TripBreaker
	var in foremanapi.TripBreakerIn
	var out foremanapi.TripBreakerOut
	err = marshalFunction(w, r, foremanapi.TripBreaker.Route, &in, &out, func(_ any, _ any) error {
		err = svc.TripBreaker(r.Context(), in.TaskName)
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
TimeBudget is the hard ceiling on the execution time of any task step. It is applied as the timeout on the task dispatch call; a task endpoint may declare a shorter budget of its own via sub.TimeBudget.
*/
func (svc *Intermediate) TimeBudget() (budget time.Duration) { // MARKER: TimeBudget
	_val := svc.Config("TimeBudget")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetTimeBudget sets the value of the configuration property.
*/
func (svc *Intermediate) SetTimeBudget(budget time.Duration) (err error) { // MARKER: TimeBudget
	return svc.SetConfig("TimeBudget", budget.String())
}

/*
DefaultPriority is the priority assigned to a flow when the caller does not specify one. Priority is an integer >= 0, lower numbers run first.
*/
func (svc *Intermediate) DefaultPriority() (defaultPriority int) { // MARKER: DefaultPriority
	_val := svc.Config("DefaultPriority")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetDefaultPriority sets the value of the configuration property.
*/
func (svc *Intermediate) SetDefaultPriority(defaultPriority int) (err error) { // MARKER: DefaultPriority
	return svc.SetConfig("DefaultPriority", strconv.Itoa(defaultPriority))
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
SQLConnectionPool is the number of database connections kept open per shard.
*/
func (svc *Intermediate) SQLConnectionPool() (value int) { // MARKER: SQLConnectionPool
	_val := svc.Config("SQLConnectionPool")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetSQLConnectionPool sets the value of the configuration property.
*/
func (svc *Intermediate) SetSQLConnectionPool(value int) (err error) { // MARKER: SQLConnectionPool
	return svc.SetConfig("SQLConnectionPool", strconv.Itoa(value))
}

/*
IncrementFlowsStarted counts the number of flows that have been started.
*/
func (svc *Intermediate) IncrementFlowsStarted(ctx context.Context, value int, workflow string) (err error) { // MARKER: FlowsStarted
	return svc.IncrementCounter(ctx, "microbus_flows_started_total", float64(value),
		"workflow", utils.AnyToString(workflow),
	)
}

/*
IncrementFlowsTerminated counts the number of flows that have reached a terminal status.
*/
func (svc *Intermediate) IncrementFlowsTerminated(ctx context.Context, value int, workflow string, status string) (err error) { // MARKER: FlowsTerminated
	return svc.IncrementCounter(ctx, "microbus_flows_terminated_total", float64(value),
		"workflow", utils.AnyToString(workflow),
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
RecordStepsQueueDepth records the number of steps waiting in the local worker queue.
*/
func (svc *Intermediate) RecordStepsQueueDepth(ctx context.Context, value int) (err error) { // MARKER: StepsQueueDepth
	return svc.RecordGauge(ctx, "microbus_steps_queue_depth", float64(value))
}

/*
IncrementStepsRecovered counts the number of steps recovered by pollPendingSteps after lease expiry.
*/
func (svc *Intermediate) IncrementStepsRecovered(ctx context.Context, value int) (err error) { // MARKER: StepsRecovered
	return svc.IncrementCounter(ctx, "microbus_steps_recovered_total", float64(value))
}

/*
RecordStepsPending records the number of due pending steps in each priority band.
*/
func (svc *Intermediate) RecordStepsPending(ctx context.Context, value int, priority string) (err error) { // MARKER: StepsPending
	return svc.RecordGauge(ctx, "microbus_steps_pending", float64(value),
		"priority", utils.AnyToString(priority),
	)
}

/*
RecordStepsOldestPendingAgeSeconds records the age in seconds of the oldest due pending step in each priority band.
*/
func (svc *Intermediate) RecordStepsOldestPendingAgeSeconds(ctx context.Context, value int, priority string) (err error) { // MARKER: StepsOldestPendingAgeSeconds
	return svc.RecordGauge(ctx, "microbus_steps_oldest_pending_age_seconds", float64(value),
		"priority", utils.AnyToString(priority),
	)
}

/*
RecordStepsFairnessKeys records the number of distinct fairness keys in the most recent refill selection at the given priority band.
*/
func (svc *Intermediate) RecordStepsFairnessKeys(ctx context.Context, value int, priority string) (err error) { // MARKER: StepsFairnessKeys
	return svc.RecordGauge(ctx, "microbus_steps_fairness_keys", float64(value),
		"priority", utils.AnyToString(priority),
	)
}

/*
IncrementStepsSkippedSaturated counts the number of step admissions skipped because the task was at its current rate-limit ceiling.
*/
func (svc *Intermediate) IncrementStepsSkippedSaturated(ctx context.Context, value int, task string) (err error) { // MARKER: StepsSkippedSaturated
	return svc.IncrementCounter(ctx, "microbus_steps_skipped_saturated_total", float64(value),
		"task", utils.AnyToString(task),
	)
}

/*
RecordTaskRateLimit records the current adaptive per-task dispatch-rate ceiling in ops/sec.
*/
func (svc *Intermediate) RecordTaskRateLimit(ctx context.Context, value int, task string) (err error) { // MARKER: TaskRateLimit
	return svc.RecordGauge(ctx, "microbus_task_rate_limit", float64(value),
		"task", utils.AnyToString(task),
	)
}

/*
RecordTaskConcurrencyRunning records the cluster-wide number of running steps per task.
*/
func (svc *Intermediate) RecordTaskConcurrencyRunning(ctx context.Context, value int, task string) (err error) { // MARKER: TaskConcurrencyRunning
	return svc.RecordGauge(ctx, "microbus_task_concurrency_running", float64(value),
		"task", utils.AnyToString(task),
	)
}

/*
IncrementTaskRateCuts counts the additive-decrease cuts to the per-task rate ceiling triggered by a 429 from a task dispatch.
*/
func (svc *Intermediate) IncrementTaskRateCuts(ctx context.Context, value int, task string) (err error) { // MARKER: TaskRateCuts
	return svc.IncrementCounter(ctx, "microbus_task_rate_cuts_total", float64(value),
		"task", utils.AnyToString(task),
	)
}

/*
IncrementTaskBreakerTrips counts the number of times a per-task breaker transitioned to tripped, labelled by cause.
*/
func (svc *Intermediate) IncrementTaskBreakerTrips(ctx context.Context, value int, task string, cause string) (err error) { // MARKER: TaskBreakerTrips
	return svc.IncrementCounter(ctx, "microbus_task_breaker_trips_total", float64(value),
		"task", utils.AnyToString(task),
		"cause", utils.AnyToString(cause),
	)
}

/*
IncrementTaskBreakerProbes counts probe attempts against a tripped breaker, labelled by outcome and the original trip cause.
*/
func (svc *Intermediate) IncrementTaskBreakerProbes(ctx context.Context, value int, task string, outcome string, cause string) (err error) { // MARKER: TaskBreakerProbes
	return svc.IncrementCounter(ctx, "microbus_task_breaker_probes_total", float64(value),
		"task", utils.AnyToString(task),
		"outcome", utils.AnyToString(outcome),
		"cause", utils.AnyToString(cause),
	)
}

/*
RecordTaskBreakerState records the current state (0=closed (admitting), 1=tripped (blocked)) of a per-task breaker.
*/
func (svc *Intermediate) RecordTaskBreakerState(ctx context.Context, value int, task string) (err error) { // MARKER: TaskBreakerState
	return svc.RecordGauge(ctx, "microbus_task_breaker_state", float64(value),
		"task", utils.AnyToString(task),
	)
}

package foremanapi

import (
	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "foreman.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "Foreman"

// Version is the major version of the microservice's public API.
const Version = 52

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Foreman orchestrates agentic workflow execution.`

// SQLDataSourceName is the connection string of the SQL database.
var SQLDataSourceName = define.Config{ // MARKER: SQLDataSourceName
	Value:  string(""),
	Secret: true,
}

// Workers is the number of concurrent workers that process flow steps.
var Workers = define.Config{ // MARKER: Workers
	Value:      int(0),
	Default:    "64",
	Validation: "int [1,]",
}

// TimeBudget is the hard ceiling on the execution time of any task step. It is applied as the timeout on the task dispatch call; a task endpoint may declare a shorter budget of its own via sub.TimeBudget.
var TimeBudget = define.Config{ // MARKER: TimeBudget
	Value:      time.Duration(0),
	Default:    "2m",
	Validation: "dur [1s,15m]",
}

// DefaultPriority is the priority assigned to a flow when the caller does not specify one. Priority is an integer >= 1, lower numbers run first.
var DefaultPriority = define.Config{ // MARKER: DefaultPriority
	Value:      int(0),
	Default:    "5",
	Validation: "int [1,]",
}

// NumShards is the number of database shards. Each shard is a separate database instance. Shards can be added dynamically but never removed.
var NumShards = define.Config{ // MARKER: NumShards
	Value:      int(0),
	Default:    "1",
	Validation: "int [1,]",
	Callback:   true,
}

// SQLConnectionPool is the number of database connections kept open per shard.
var SQLConnectionPool = define.Config{ // MARKER: SQLConnectionPool
	Value:      int(0),
	Default:    "8",
	Validation: "int [1,]",
}

// OnFlowStopped is triggered when a flow stops (completed, failed, cancelled, or interrupted). Subscribe with ForHost(svc.Hostname()) for flows created with FlowOptions.NotifyOnStop.
var OnFlowStopped = define.OutboundEvent{ // MARKER: OnFlowStopped
	Host: Hostname, Method: "POST", Route: ":417/on-flow-terminated",
	In: OnFlowStoppedIn{}, Out: OnFlowStoppedOut{},
}

// OnFlowStoppedIn are the input arguments of OnFlowStopped.
type OnFlowStoppedIn struct { // MARKER: OnFlowStopped
	FlowKey string                `json:"flowKey,omitzero"`
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
}

// OnFlowStoppedOut are the output arguments of OnFlowStopped.
type OnFlowStoppedOut struct { // MARKER: OnFlowStopped
}

// Create creates a new flow for a workflow without starting it.
var Create = define.Function{ // MARKER: Create
	Host: Hostname, Method: "ANY", Route: ":444/create",
	In: CreateIn{}, Out: CreateOut{},
}

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	WorkflowURL  string                `json:"workflowURL,omitzero"`
	InitialState any                   `json:"initialState,omitzero"`
	Opts         *workflow.FlowOptions `json:"opts,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	FlowKey string `json:"flowKey,omitzero"`
}

// Start transitions a created flow to running and enqueues it for execution.
var Start = define.Function{ // MARKER: Start
	Host: Hostname, Method: "ANY", Route: ":444/start",
	In: StartIn{}, Out: StartOut{},
}

// StartIn are the input arguments of Start.
type StartIn struct { // MARKER: Start
	FlowKey string `json:"flowKey,omitzero"`
}

// StartOut are the output arguments of Start.
type StartOut struct { // MARKER: Start
}

// Snapshot returns the current outcome of a flow.
var Snapshot = define.Function{ // MARKER: Snapshot
	Host: Hostname, Method: "GET", Route: ":444/snapshot",
	In: SnapshotIn{}, Out: SnapshotOut{},
}

// SnapshotIn are the input arguments of Snapshot.
type SnapshotIn struct { // MARKER: Snapshot
	FlowKey string `json:"flowKey,omitzero"`
}

// SnapshotOut are the output arguments of Snapshot.
type SnapshotOut struct { // MARKER: Snapshot
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
}

// Fingerprint returns a short opaque hash that changes when the flow's status, step count, or any step's updated_at changes — across the flow and any nested subgraph descendants.
var Fingerprint = define.Function{ // MARKER: Fingerprint
	Host: Hostname, Method: "GET", Route: ":444/fingerprint",
	In: FingerprintIn{}, Out: FingerprintOut{},
}

// FingerprintIn are the input arguments of Fingerprint.
type FingerprintIn struct { // MARKER: Fingerprint
	FlowKey string `json:"flowKey,omitzero"`
}

// FingerprintOut are the output arguments of Fingerprint.
type FingerprintOut struct { // MARKER: Fingerprint
	Fingerprint string `json:"fingerprint,omitzero"`
	Status      string `json:"status,omitzero"`
}

// Resume continues an interrupted flow, delivering resumeData to the task that armed flow.Interrupt. Fails with 409 if the flow is paused at a breakpoint rather than an interrupt.
var Resume = define.Function{ // MARKER: Resume
	Host: Hostname, Method: "POST", Route: ":444/resume",
	In: ResumeIn{}, Out: ResumeOut{},
}

// ResumeIn are the input arguments of Resume.
type ResumeIn struct { // MARKER: Resume
	FlowKey    string `json:"flowKey,omitzero"`
	ResumeData any    `json:"resumeData,omitzero"`
}

// ResumeOut are the output arguments of Resume.
type ResumeOut struct { // MARKER: Resume
}

// ResumeBreak continues a flow paused at a breakpoint, merging stateOverrides into the leaf step's input state so the about-to-run task observes them. Fails with 409 if the flow is paused at an interrupt rather than a breakpoint.
var ResumeBreak = define.Function{ // MARKER: ResumeBreak
	Host: Hostname, Method: "POST", Route: ":444/resume-break",
	In: ResumeBreakIn{}, Out: ResumeBreakOut{},
}

// ResumeBreakIn are the input arguments of ResumeBreak.
type ResumeBreakIn struct { // MARKER: ResumeBreak
	FlowKey        string `json:"flowKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// ResumeBreakOut are the output arguments of ResumeBreak.
type ResumeBreakOut struct { // MARKER: ResumeBreak
}

// Cancel cancels a flow that is not yet in a terminal status.
var Cancel = define.Function{ // MARKER: Cancel
	Host: Hostname, Method: "POST", Route: ":444/cancel",
	In: CancelIn{}, Out: CancelOut{},
}

// CancelIn are the input arguments of Cancel.
type CancelIn struct { // MARKER: Cancel
	FlowKey string `json:"flowKey,omitzero"`
	Reason  string `json:"reason,omitzero"`
}

// CancelOut are the output arguments of Cancel.
type CancelOut struct { // MARKER: Cancel
}

// Restart wipes everything past a flow's entry step and resets the entry with overrides.
var Restart = define.Function{ // MARKER: Restart
	Host: Hostname, Method: "POST", Route: ":444/restart",
	In: RestartIn{}, Out: RestartOut{},
}

// RestartIn are the input arguments of Restart.
type RestartIn struct { // MARKER: Restart
	FlowKey        string `json:"flowKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// RestartOut are the output arguments of Restart.
type RestartOut struct { // MARKER: Restart
}

// RestartFrom sweeps the DAG subtree below a chosen step and resets that step with overrides.
var RestartFrom = define.Function{ // MARKER: RestartFrom
	Host: Hostname, Method: "POST", Route: ":444/restart-from",
	In: RestartFromIn{}, Out: RestartFromOut{},
}

// RestartFromIn are the input arguments of RestartFrom.
type RestartFromIn struct { // MARKER: RestartFrom
	StepKey        string `json:"stepKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// RestartFromOut are the output arguments of RestartFrom.
type RestartFromOut struct { // MARKER: RestartFrom
}

// History returns the step-by-step execution history of a flow.
var History = define.Function{ // MARKER: History
	Host: Hostname, Method: "GET", Route: ":444/history",
	In: HistoryIn{}, Out: HistoryOut{},
}

// HistoryIn are the input arguments of History.
type HistoryIn struct { // MARKER: History
	FlowKey string `json:"flowKey,omitzero"`
}

// HistoryOut are the output arguments of History.
type HistoryOut struct { // MARKER: History
	Steps []FlowStep `json:"steps,omitzero"`
}

// Step returns the full detail of one execution step, including the state, changes and interrupt payload that History omits.
var Step = define.Function{ // MARKER: Step
	Host: Hostname, Method: "GET", Route: ":444/step",
	In: StepIn{}, Out: StepOut{},
}

// StepIn are the input arguments of Step.
type StepIn struct { // MARKER: Step
	StepKey string `json:"stepKey,omitzero"`
}

// StepOut are the output arguments of Step.
type StepOut struct { // MARKER: Step
	Step *FlowStep `json:"step,omitzero"`
}

// List queries flows by status or workflow URL. Set Query.Cursor to the previous call's NextCursor to paginate.
var List = define.Function{ // MARKER: List
	Host: Hostname, Method: "GET", Route: ":444/list",
	In: ListIn{}, Out: ListOut{},
}

// ListIn are the input arguments of List.
type ListIn struct { // MARKER: List
	Query Query `json:"query,omitzero"`
}

// ListOut are the output arguments of List.
type ListOut struct { // MARKER: List
	Flows []FlowSummary `json:"flows,omitzero"`
	// NextCursor is the opaque pagination cursor for the next page; pass it back as Query.Cursor.
	// Empty when every shard has been drained.
	NextCursor string `json:"nextCursor,omitzero"`
}

// Delete removes a flow and its steps from the database. The flow must not be running. Subgraph and thread lineage references become dangling.
var Delete = define.Function{ // MARKER: Delete
	Host: Hostname, Method: "POST", Route: ":444/delete",
	In: DeleteIn{}, Out: DeleteOut{},
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	FlowKey string `json:"flowKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
}

// Purge deletes flows matching the query, except those currently running. Capped at 10000 flows per call.
var Purge = define.Function{ // MARKER: Purge
	Host: Hostname, Method: "POST", Route: ":444/purge",
	In: PurgeIn{}, Out: PurgeOut{},
}

// PurgeIn are the input arguments of Purge.
type PurgeIn struct { // MARKER: Purge
	Query Query `json:"query,omitzero"`
}

// PurgeOut are the output arguments of Purge.
type PurgeOut struct { // MARKER: Purge
	// Deleted is the count of flows actually deleted (excluding running flows skipped by the guard).
	Deleted int `json:"deleted,omitzero"`
}

// ShardInfo returns per-shard health (latency, row counts, error) for every database shard.
var ShardInfo = define.Function{ // MARKER: ShardInfo
	Host: Hostname, Method: "GET", Route: ":444/shard-info",
	In: ShardInfoIn{}, Out: ShardInfoOut{},
}

// ShardInfoIn are the input arguments of ShardInfo.
type ShardInfoIn struct { // MARKER: ShardInfo
}

// ShardInfoOut are the output arguments of ShardInfo.
type ShardInfoOut struct { // MARKER: ShardInfo
	Shards []ShardSummary `json:"shards,omitzero"`
}

// CreateTask creates a flow that executes a single task and then terminates, without starting it.
var CreateTask = define.Function{ // MARKER: CreateTask
	Host: Hostname, Method: "POST", Route: ":444/create-task",
	In: CreateTaskIn{}, Out: CreateTaskOut{},
}

// CreateTaskIn are the input arguments of CreateTask.
type CreateTaskIn struct { // MARKER: CreateTask
	Name         string                `json:"name,omitzero"`
	TaskURL      string                `json:"taskURL,omitzero"`
	InitialState any                   `json:"initialState,omitzero"`
	Opts         *workflow.FlowOptions `json:"opts,omitzero"`
}

// CreateTaskOut are the output arguments of CreateTask.
type CreateTaskOut struct { // MARKER: CreateTask
	FlowKey string `json:"flowKey,omitzero"`
}

// Await blocks until the flow stops (i.e. is no longer created, pending, or running), then returns the outcome.
var Await = define.Function{ // MARKER: Await
	Host: Hostname, Method: "POST", Route: ":444/wait-for-stop",
	In: AwaitIn{}, Out: AwaitOut{},
}

// AwaitIn are the input arguments of Await.
type AwaitIn struct { // MARKER: Await
	FlowKey string `json:"flowKey,omitzero"`
}

// AwaitOut are the output arguments of Await.
type AwaitOut struct { // MARKER: Await
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
}

// BreakBefore sets or clears a breakpoint that pauses execution before the named task runs.
var BreakBefore = define.Function{ // MARKER: BreakBefore
	Host: Hostname, Method: "POST", Route: ":444/break-before",
	In: BreakBeforeIn{}, Out: BreakBeforeOut{},
}

// BreakBeforeIn are the input arguments of BreakBefore.
type BreakBeforeIn struct { // MARKER: BreakBefore
	FlowKey  string `json:"flowKey,omitzero"`
	TaskName string `json:"taskName,omitzero"`
	Enabled  bool   `json:"enabled,omitzero"`
}

// BreakBeforeOut are the output arguments of BreakBefore.
type BreakBeforeOut struct { // MARKER: BreakBefore
}

// Run creates a new flow, starts it, and blocks until it stops. Returns the terminal outcome.
var Run = define.Function{ // MARKER: Run
	Host: Hostname, Method: "POST", Route: ":444/run",
	In: RunIn{}, Out: RunOut{},
}

// RunIn are the input arguments of Run.
type RunIn struct { // MARKER: Run
	WorkflowURL  string                `json:"workflowURL,omitzero"`
	InitialState any                   `json:"initialState,omitzero"`
	Opts         *workflow.FlowOptions `json:"opts,omitzero"`
}

// RunOut are the output arguments of Run.
type RunOut struct { // MARKER: Run
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
}

// Continue creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers. The threadKey can be any flowKey belonging to the thread. The new flow belongs to the same thread and is returned in created status.
var Continue = define.Function{ // MARKER: Continue
	Host: Hostname, Method: "POST", Route: ":444/continue",
	In: ContinueIn{}, Out: ContinueOut{},
}

// ContinueIn are the input arguments of Continue.
type ContinueIn struct { // MARKER: Continue
	ThreadKey       string                `json:"threadKey,omitzero"`
	AdditionalState any                   `json:"additionalState,omitzero"`
	Opts            *workflow.FlowOptions `json:"opts,omitzero"`
}

// ContinueOut are the output arguments of Continue.
type ContinueOut struct { // MARKER: Continue
	NewFlowKey string `json:"newFlowKey,omitzero"`
}

// Signal delivers an opaque cross-replica coordination signal (op, payload) to the embedded engine. Excludes self-delivery; processes only signals originating from a peer foreman replica.
var Signal = define.Function{ // MARKER: Signal
	Host: Hostname, Method: "POST", Route: ":444/signal",
	LoadBalancing: define.None,
	In:            SignalIn{}, Out: SignalOut{},
}

// SignalIn are the input arguments of Signal.
type SignalIn struct { // MARKER: Signal
	Op      string `json:"op,omitzero"`
	Payload []byte `json:"payload,omitzero"`
}

// SignalOut are the output arguments of Signal.
type SignalOut struct { // MARKER: Signal
}

// HistoryMermaid renders an HTML page with a Mermaid diagram of the flow's execution history.
var HistoryMermaid = define.Web{ // MARKER: HistoryMermaid
	Host: Hostname, Method: "GET", Route: ":444/history-mermaid",
}

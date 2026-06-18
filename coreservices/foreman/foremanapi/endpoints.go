package foremanapi

import (
	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "foreman.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
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

// StartIn are the input arguments of Start.
type StartIn struct { // MARKER: Start
	FlowKey string `json:"flowKey,omitzero"`
}

// StartOut are the output arguments of Start.
type StartOut struct { // MARKER: Start
}

// SnapshotIn are the input arguments of Snapshot.
type SnapshotIn struct { // MARKER: Snapshot
	FlowKey string `json:"flowKey,omitzero"`
}

// SnapshotOut are the output arguments of Snapshot.
type SnapshotOut struct { // MARKER: Snapshot
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
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

// ResumeIn are the input arguments of Resume.
type ResumeIn struct { // MARKER: Resume
	FlowKey    string `json:"flowKey,omitzero"`
	ResumeData any    `json:"resumeData,omitzero"`
}

// ResumeOut are the output arguments of Resume.
type ResumeOut struct { // MARKER: Resume
}

// ResumeBreakIn are the input arguments of ResumeBreak.
type ResumeBreakIn struct { // MARKER: ResumeBreak
	FlowKey        string `json:"flowKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// ResumeBreakOut are the output arguments of ResumeBreak.
type ResumeBreakOut struct { // MARKER: ResumeBreak
}

// CancelIn are the input arguments of Cancel.
type CancelIn struct { // MARKER: Cancel
	FlowKey string `json:"flowKey,omitzero"`
	Reason  string `json:"reason,omitzero"`
}

// CancelOut are the output arguments of Cancel.
type CancelOut struct { // MARKER: Cancel
}

// HistoryIn are the input arguments of History.
type HistoryIn struct { // MARKER: History
	FlowKey string `json:"flowKey,omitzero"`
}

// HistoryOut are the output arguments of History.
type HistoryOut struct { // MARKER: History
	Steps []FlowStep `json:"steps,omitzero"`
}

// StepIn are the input arguments of Step.
type StepIn struct { // MARKER: Step
	StepKey string `json:"stepKey,omitzero"`
}

// StepOut are the output arguments of Step.
type StepOut struct { // MARKER: Step
	Step *FlowStep `json:"step,omitzero"`
}

// RestartIn are the input arguments of Restart.
type RestartIn struct { // MARKER: Restart
	FlowKey        string `json:"flowKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// RestartOut are the output arguments of Restart.
type RestartOut struct { // MARKER: Restart
}

// RestartFromIn are the input arguments of RestartFrom.
type RestartFromIn struct { // MARKER: RestartFrom
	StepKey        string `json:"stepKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// RestartFromOut are the output arguments of RestartFrom.
type RestartFromOut struct { // MARKER: RestartFrom
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	FlowKey string `json:"flowKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
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

// AwaitIn are the input arguments of Await.
type AwaitIn struct { // MARKER: Await
	FlowKey string `json:"flowKey,omitzero"`
}

// AwaitOut are the output arguments of Await.
type AwaitOut struct { // MARKER: Await
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
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

// SignalIn are the input arguments of Signal.
type SignalIn struct { // MARKER: Signal
	Op      string `json:"op,omitzero"`
	Payload []byte `json:"payload,omitzero"`
}

// SignalOut are the output arguments of Signal.
type SignalOut struct { // MARKER: Signal
}

// OnFlowStoppedIn are the input arguments of OnFlowStopped.
type OnFlowStoppedIn struct { // MARKER: OnFlowStopped
	FlowKey string                `json:"flowKey,omitzero"`
	Outcome *workflow.FlowOutcome `json:"outcome,omitzero"`
}

// OnFlowStoppedOut are the output arguments of OnFlowStopped.
type OnFlowStoppedOut struct { // MARKER: OnFlowStopped
}

// ShardInfoIn are the input arguments of ShardInfo.
type ShardInfoIn struct { // MARKER: ShardInfo
}

// ShardInfoOut are the output arguments of ShardInfo.
type ShardInfoOut struct { // MARKER: ShardInfo
	Shards []ShardSummary `json:"shards,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Create         = Def{Method: "ANY", Route: ":444/create"}              // MARKER: Create
	Start          = Def{Method: "ANY", Route: ":444/start"}               // MARKER: Start
	Snapshot       = Def{Method: "GET", Route: ":444/snapshot"}            // MARKER: Snapshot
	Fingerprint    = Def{Method: "GET", Route: ":444/fingerprint"}         // MARKER: Fingerprint
	Resume         = Def{Method: "POST", Route: ":444/resume"}             // MARKER: Resume
	ResumeBreak    = Def{Method: "POST", Route: ":444/resume-break"}       // MARKER: ResumeBreak
	Restart        = Def{Method: "POST", Route: ":444/restart"}            // MARKER: Restart
	RestartFrom    = Def{Method: "POST", Route: ":444/restart-from"}       // MARKER: RestartFrom
	Cancel         = Def{Method: "POST", Route: ":444/cancel"}             // MARKER: Cancel
	History        = Def{Method: "GET", Route: ":444/history"}             // MARKER: History
	Step           = Def{Method: "GET", Route: ":444/step"}                // MARKER: Step
	List           = Def{Method: "GET", Route: ":444/list"}                // MARKER: List
	Delete         = Def{Method: "POST", Route: ":444/delete"}             // MARKER: Delete
	Purge          = Def{Method: "POST", Route: ":444/purge"}              // MARKER: Purge
	CreateTask     = Def{Method: "POST", Route: ":444/create-task"}        // MARKER: CreateTask
	Await          = Def{Method: "POST", Route: ":444/wait-for-stop"}      // MARKER: Await
	BreakBefore    = Def{Method: "POST", Route: ":444/break-before"}       // MARKER: BreakBefore
	Run            = Def{Method: "POST", Route: ":444/run"}                // MARKER: Run
	Continue       = Def{Method: "POST", Route: ":444/continue"}           // MARKER: Continue
	HistoryMermaid = Def{Method: "GET", Route: ":444/history-mermaid"}     // MARKER: HistoryMermaid
	Signal         = Def{Method: "POST", Route: ":444/signal"}             // MARKER: Signal
	OnFlowStopped  = Def{Method: "POST", Route: ":417/on-flow-terminated"} // MARKER: OnFlowStopped
	ShardInfo      = Def{Method: "GET", Route: ":444/shard-info"}          // MARKER: ShardInfo
)

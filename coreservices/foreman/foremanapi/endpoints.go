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
	WorkflowName string `json:"workflowName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
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

// StartNotifyIn are the input arguments of StartNotify.
type StartNotifyIn struct { // MARKER: StartNotify
	FlowKey        string `json:"flowKey,omitzero"`
	NotifyHostname string `json:"notifyHostname,omitzero"`
}

// StartNotifyOut are the output arguments of StartNotify.
type StartNotifyOut struct { // MARKER: StartNotify
}

// SnapshotIn are the input arguments of Snapshot.
type SnapshotIn struct { // MARKER: Snapshot
	FlowKey string `json:"flowKey,omitzero"`
}

// SnapshotOut are the output arguments of Snapshot.
type SnapshotOut struct { // MARKER: Snapshot
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
}

// ResumeIn are the input arguments of Resume.
type ResumeIn struct { // MARKER: Resume
	FlowKey    string `json:"flowKey,omitzero"`
	ResumeData any    `json:"resumeData,omitzero"`
}

// ResumeOut are the output arguments of Resume.
type ResumeOut struct { // MARKER: Resume
}

// ForkIn are the input arguments of Fork.
type ForkIn struct { // MARKER: Fork
	StepKey        string `json:"stepKey,omitzero"`
	StateOverrides any    `json:"stateOverrides,omitzero"`
}

// ForkOut are the output arguments of Fork.
type ForkOut struct { // MARKER: Fork
	NewFlowKey string `json:"newFlowKey,omitzero"`
}

// CancelIn are the input arguments of Cancel.
type CancelIn struct { // MARKER: Cancel
	FlowKey string `json:"flowKey,omitzero"`
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

// RetryIn are the input arguments of Retry.
type RetryIn struct { // MARKER: Retry
	FlowKey string `json:"flowKey,omitzero"`
}

// RetryOut are the output arguments of Retry.
type RetryOut struct { // MARKER: Retry
}

// ListIn are the input arguments of List.
type ListIn struct { // MARKER: List
	Query Query `json:"query,omitzero"`
}

// ListOut are the output arguments of List.
type ListOut struct { // MARKER: List
	Flows []FlowSummary `json:"flows,omitzero"`
}

// CreateTaskIn are the input arguments of CreateTask.
type CreateTaskIn struct { // MARKER: CreateTask
	TaskName     string `json:"taskName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
}

// CreateTaskOut are the output arguments of CreateTask.
type CreateTaskOut struct { // MARKER: CreateTask
	FlowKey string `json:"flowKey,omitzero"`
}

// EnqueueIn are the input arguments of Enqueue.
type EnqueueIn struct { // MARKER: Enqueue
	Shard  int `json:"shard,omitzero"`
	StepID int `json:"stepID,omitzero"`
}

// EnqueueOut are the output arguments of Enqueue.
type EnqueueOut struct { // MARKER: Enqueue
}

// AwaitIn are the input arguments of Await.
type AwaitIn struct { // MARKER: Await
	FlowKey string `json:"flowKey,omitzero"`
}

// AwaitOut are the output arguments of Await.
type AwaitOut struct { // MARKER: Await
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
}

// NotifyStatusChangeIn are the input arguments of NotifyStatusChange.
type NotifyStatusChangeIn struct { // MARKER: NotifyStatusChange
	FlowKey string `json:"flowKey,omitzero"`
	Status  string `json:"status,omitzero"`
}

// NotifyStatusChangeOut are the output arguments of NotifyStatusChange.
type NotifyStatusChangeOut struct { // MARKER: NotifyStatusChange
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
	WorkflowName string `json:"workflowName,omitzero"`
	InitialState any    `json:"initialState,omitzero"`
}

// RunOut are the output arguments of Run.
type RunOut struct { // MARKER: Run
	Status string         `json:"status,omitzero"`
	State  map[string]any `json:"state,omitzero"`
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

// OnFlowStoppedIn are the input arguments of OnFlowStopped.
type OnFlowStoppedIn struct { // MARKER: OnFlowStopped
	FlowKey  string         `json:"flowKey,omitzero"`
	Status   string         `json:"status,omitzero"`
	Snapshot map[string]any `json:"snapshot,omitzero"`
}

// OnFlowStoppedOut are the output arguments of OnFlowStopped.
type OnFlowStoppedOut struct { // MARKER: OnFlowStopped
}

var (
	// HINT: Insert endpoint definitions here
	Create             = Def{Method: "ANY", Route: ":444/create"}                // MARKER: Create
	Start              = Def{Method: "ANY", Route: ":444/start"}                 // MARKER: Start
	StartNotify        = Def{Method: "ANY", Route: ":444/start-notify"}          // MARKER: StartNotify
	Snapshot           = Def{Method: "GET", Route: ":444/snapshot"}              // MARKER: Snapshot
	Resume             = Def{Method: "POST", Route: ":444/resume"}               // MARKER: Resume
	Fork               = Def{Method: "POST", Route: ":444/fork"}                 // MARKER: Fork
	Cancel             = Def{Method: "POST", Route: ":444/cancel"}               // MARKER: Cancel
	History            = Def{Method: "GET", Route: ":444/history"}               // MARKER: History
	Retry              = Def{Method: "POST", Route: ":444/retry"}                // MARKER: Retry
	List               = Def{Method: "GET", Route: ":444/list"}                  // MARKER: List
	CreateTask         = Def{Method: "POST", Route: ":444/create-task"}          // MARKER: CreateTask
	Enqueue            = Def{Method: "POST", Route: ":444/enqueue"}              // MARKER: Enqueue
	Await              = Def{Method: "POST", Route: ":444/wait-for-stop"}        // MARKER: Await
	NotifyStatusChange = Def{Method: "POST", Route: ":444/notify-status-change"} // MARKER: NotifyStatusChange
	BreakBefore        = Def{Method: "POST", Route: ":444/break-before"}         // MARKER: BreakBefore
	Run                = Def{Method: "POST", Route: ":444/run"}                  // MARKER: Run
	Continue           = Def{Method: "POST", Route: ":444/continue"}             // MARKER: Continue
	HistoryMermaid     = Def{Method: "GET", Route: ":444/history-mermaid"}       // MARKER: HistoryMermaid
	OnFlowStopped      = Def{Method: "POST", Route: ":417/on-flow-terminated"}   // MARKER: OnFlowStopped
)

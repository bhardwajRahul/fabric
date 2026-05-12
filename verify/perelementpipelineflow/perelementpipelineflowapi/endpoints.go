package perelementpipelineflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "perelementpipelineflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	TaskS          = Def{Method: "POST", Route: ":428/task-s"}             // MARKER: TaskS
	TaskH          = Def{Method: "POST", Route: ":428/task-h"}             // MARKER: TaskH
	TaskA          = Def{Method: "POST", Route: ":428/task-a"}             // MARKER: TaskA
	TaskB          = Def{Method: "POST", Route: ":428/task-b"}             // MARKER: TaskB
	TaskM          = Def{Method: "POST", Route: ":428/task-m"}             // MARKER: TaskM
	TaskL          = Def{Method: "POST", Route: ":428/task-l"}             // MARKER: TaskL
	PerElementPipeline = Def{Method: "GET", Route: ":428/per-element-pipeline"} // MARKER: PerElementPipeline
)

// TaskSIn are the input arguments of TaskS.
type TaskSIn struct { // MARKER: TaskS
	Items []string `json:"items,omitzero"`
}

// TaskSOut are the output arguments of TaskS.
type TaskSOut struct { // MARKER: TaskS
	ItemsOut []string `json:"items,omitzero"`
}

// TaskHIn are the input arguments of TaskH. Per-element entry of the inner pipeline.
type TaskHIn struct { // MARKER: TaskH
	Item string `json:"item,omitzero"`
}

// TaskHOut are the output arguments of TaskH.
type TaskHOut struct { // MARKER: TaskH
	ItemUpper string `json:"itemUpper,omitzero"`
}

// TaskAIn are the input arguments of TaskA. One of two parallel branches per element.
type TaskAIn struct { // MARKER: TaskA
	ItemUpper string `json:"itemUpper,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	AProcessed string `json:"aProcessed,omitzero"`
}

// TaskBIn are the input arguments of TaskB. The other parallel branch per element.
type TaskBIn struct { // MARKER: TaskB
	ItemUpper string `json:"itemUpper,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	BProcessed string `json:"bProcessed,omitzero"`
}

// TaskMIn are the input arguments of TaskM. Per-element fan-in.
type TaskMIn struct { // MARKER: TaskM
	AProcessed string `json:"aProcessed,omitzero"`
	BProcessed string `json:"bProcessed,omitzero"`
}

// TaskMOut are the output arguments of TaskM. Contributes one item to the outer set-reducer field.
type TaskMOut struct { // MARKER: TaskM
	SetMerged []string `json:"setMerged,omitzero"`
}

// TaskLIn are the input arguments of TaskL.
type TaskLIn struct { // MARKER: TaskL
	SetMerged []string `json:"setMerged,omitzero"`
}

// TaskLOut are the output arguments of TaskL.
type TaskLOut struct { // MARKER: TaskL
	FinalCount int `json:"finalCount,omitzero"`
}

// PerElementPipelineIn are the input arguments of PerElementPipeline.
type PerElementPipelineIn struct { // MARKER: PerElementPipeline
	Items []string `json:"items,omitzero"`
}

// PerElementPipelineOut are the output arguments of PerElementPipeline.
type PerElementPipelineOut struct { // MARKER: PerElementPipeline
	FinalCount int `json:"finalCount,omitzero"`
}

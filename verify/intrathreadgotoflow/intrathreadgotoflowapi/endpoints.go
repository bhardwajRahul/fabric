package intrathreadgotoflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "intrathreadgotoflow.verify"

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
	TaskA            = Def{Method: "POST", Route: ":428/task-a"}             // MARKER: TaskA
	LoopTask         = Def{Method: "POST", Route: ":428/loop-task"}          // MARKER: LoopTask
	NormalC          = Def{Method: "POST", Route: ":428/normal-c"}           // MARKER: NormalC
	TaskD            = Def{Method: "POST", Route: ":428/task-d"}             // MARKER: TaskD
	IntraThreadGoto  = Def{Method: "GET", Route: ":428/intra-thread-goto"}   // MARKER: IntraThreadGoto
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Target int `json:"target,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	TargetOut int `json:"target,omitzero"`
}

// LoopTaskIn are the input arguments of LoopTask.
type LoopTaskIn struct { // MARKER: LoopTask
	Loops  int `json:"loops,omitzero"`
	Target int `json:"target,omitzero"`
}

// LoopTaskOut are the output arguments of LoopTask.
type LoopTaskOut struct { // MARKER: LoopTask
	LoopsOut int `json:"loops,omitzero"`
}

// NormalCIn are the input arguments of NormalC.
type NormalCIn struct { // MARKER: NormalC
}

// NormalCOut are the output arguments of NormalC.
type NormalCOut struct { // MARKER: NormalC
	Stamp string `json:"stamp,omitzero"`
}

// TaskDIn are the input arguments of TaskD.
type TaskDIn struct { // MARKER: TaskD
	Loops int    `json:"loops,omitzero"`
	Stamp string `json:"stamp,omitzero"`
}

// TaskDOut are the output arguments of TaskD.
type TaskDOut struct { // MARKER: TaskD
	FinalResult string `json:"finalResult,omitzero"`
}

// IntraThreadGotoIn are the input arguments of IntraThreadGoto.
type IntraThreadGotoIn struct { // MARKER: IntraThreadGoto
	Target int `json:"target,omitzero"`
}

// IntraThreadGotoOut are the output arguments of IntraThreadGoto.
type IntraThreadGotoOut struct { // MARKER: IntraThreadGoto
	FinalResult string `json:"finalResult,omitzero"`
}

package gotoflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "gotoflow.verify"

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
	TaskA      = Def{Method: "POST", Route: ":428/task-a"}      // MARKER: TaskA
	TaskB      = Def{Method: "POST", Route: ":428/task-b"}      // MARKER: TaskB
	TaskC      = Def{Method: "POST", Route: ":428/task-c"}      // MARKER: TaskC
	BadGotoer  = Def{Method: "POST", Route: ":428/bad-gotoer"}  // MARKER: BadGotoer
	Goto       = Def{Method: "GET", Route: ":428/goto"}         // MARKER: Goto
	BadGoto    = Def{Method: "GET", Route: ":428/bad-goto"}     // MARKER: BadGoto
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Loops int `json:"loops,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	LoopsOut int `json:"loops,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Loops  int `json:"loops,omitzero"`
	Target int `json:"target,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	Visited bool `json:"visited,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Loops int `json:"loops,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	FinalLoops int `json:"finalLoops,omitzero"`
}

// GotoIn are the input arguments of Goto.
type GotoIn struct { // MARKER: Goto
	Target int `json:"target,omitzero"`
}

// GotoOut are the output arguments of Goto.
type GotoOut struct { // MARKER: Goto
	FinalLoops int `json:"finalLoops,omitzero"`
}

// BadGotoerIn are the input arguments of BadGotoer.
type BadGotoerIn struct { // MARKER: BadGotoer
}

// BadGotoerOut are the output arguments of BadGotoer.
type BadGotoerOut struct { // MARKER: BadGotoer
	Stamp bool `json:"stamp,omitzero"`
}

// BadGotoIn are the input arguments of BadGoto.
type BadGotoIn struct { // MARKER: BadGoto
}

// BadGotoOut are the output arguments of BadGoto.
type BadGotoOut struct { // MARKER: BadGoto
	Stamp bool `json:"stamp,omitzero"`
}

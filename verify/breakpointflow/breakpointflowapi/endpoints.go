package breakpointflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "breakpointflow.verify"

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
	TaskA      = Def{Method: "POST", Route: ":428/task-a"}     // MARKER: TaskA
	TaskB      = Def{Method: "POST", Route: ":428/task-b"}     // MARKER: TaskB
	TaskC      = Def{Method: "POST", Route: ":428/task-c"}     // MARKER: TaskC
	Breakpoint = Def{Method: "GET", Route: ":428/breakpoint"}  // MARKER: Breakpoint
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	StepA bool `json:"stepA,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	StepA bool `json:"stepA,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	StepB bool `json:"stepB,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	StepB bool `json:"stepB,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	StepC bool `json:"stepC,omitzero"`
}

// BreakpointIn are the input arguments of Breakpoint.
type BreakpointIn struct { // MARKER: Breakpoint
}

// BreakpointOut are the output arguments of Breakpoint.
type BreakpointOut struct { // MARKER: Breakpoint
	StepC bool `json:"stepC,omitzero"`
}

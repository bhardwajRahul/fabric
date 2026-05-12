package errorflowapi

import (
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "errorflow.verify"

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
	TaskA   = Def{Method: "POST", Route: ":428/task-a"}   // MARKER: TaskA
	TaskB   = Def{Method: "POST", Route: ":428/task-b"}   // MARKER: TaskB
	Handler = Def{Method: "POST", Route: ":428/handler"}  // MARKER: Handler
	TaskC   = Def{Method: "POST", Route: ":428/task-c"}   // MARKER: TaskC
	Error   = Def{Method: "GET", Route: ":428/error"}     // MARKER: Error
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Trigger string `json:"trigger,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	TriggerOut string `json:"trigger,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Trigger string `json:"trigger,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	Result string `json:"result,omitzero"`
}

// HandlerIn are the input arguments of Handler.
type HandlerIn struct { // MARKER: Handler
	OnErr *errors.TracedError `json:"onErr,omitzero"`
}

// HandlerOut are the output arguments of Handler.
type HandlerOut struct { // MARKER: Handler
	Result string `json:"result,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Result string `json:"result,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	FinalResult string `json:"finalResult,omitzero"`
}

// ErrorIn are the input arguments of Error.
type ErrorIn struct { // MARKER: Error
	Trigger string `json:"trigger,omitzero"`
}

// ErrorOut are the output arguments of Error.
type ErrorOut struct { // MARKER: Error
	FinalResult string `json:"finalResult,omitzero"`
}

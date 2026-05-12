package fanouterrorflowapi

import (
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "fanouterrorflow.verify"

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
	TaskA         = Def{Method: "POST", Route: ":428/task-a"}            // MARKER: TaskA
	TaskB         = Def{Method: "POST", Route: ":428/task-b"}            // MARKER: TaskB
	TaskC         = Def{Method: "POST", Route: ":428/task-c"}            // MARKER: TaskC
	TaskD         = Def{Method: "POST", Route: ":428/task-d"}            // MARKER: TaskD
	Handler       = Def{Method: "POST", Route: ":428/handler"}           // MARKER: Handler
	TaskE         = Def{Method: "POST", Route: ":428/task-e"}            // MARKER: TaskE
	FanOutError   = Def{Method: "GET", Route: ":428/fan-out-error"}      // MARKER: FanOutError
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	Started bool `json:"started,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Started bool `json:"started,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	MarkB bool `json:"markB,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Started bool `json:"started,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	MarkC bool `json:"markC,omitzero"`
}

// TaskDIn are the input arguments of TaskD.
type TaskDIn struct { // MARKER: TaskD
	Started bool `json:"started,omitzero"`
}

// TaskDOut are the output arguments of TaskD.
type TaskDOut struct { // MARKER: TaskD
	MarkD bool `json:"markD,omitzero"`
}

// HandlerIn are the input arguments of Handler.
type HandlerIn struct { // MARKER: Handler
	OnErr *errors.TracedError `json:"onErr,omitzero"`
}

// HandlerOut are the output arguments of Handler.
type HandlerOut struct { // MARKER: Handler
	Handled bool `json:"handled,omitzero"`
}

// TaskEIn are the input arguments of TaskE.
type TaskEIn struct { // MARKER: TaskE
	Handled bool `json:"handled,omitzero"`
	MarkB   bool `json:"markB,omitzero"`
	MarkC   bool `json:"markC,omitzero"`
	MarkD   bool `json:"markD,omitzero"`
}

// TaskEOut are the output arguments of TaskE.
type TaskEOut struct { // MARKER: TaskE
	Recovered bool `json:"recovered,omitzero"`
}

// FanOutErrorIn are the input arguments of FanOutError.
type FanOutErrorIn struct { // MARKER: FanOutError
}

// FanOutErrorOut are the output arguments of FanOutError.
type FanOutErrorOut struct { // MARKER: FanOutError
	Recovered bool `json:"recovered,omitzero"`
}

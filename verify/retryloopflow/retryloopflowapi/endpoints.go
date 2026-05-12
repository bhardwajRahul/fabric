package retryloopflowapi

import (
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "retryloopflow.verify"

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
	TaskA     = Def{Method: "POST", Route: ":428/task-a"}      // MARKER: TaskA
	TaskB     = Def{Method: "POST", Route: ":428/task-b"}      // MARKER: TaskB
	Handler   = Def{Method: "POST", Route: ":428/handler"}     // MARKER: Handler
	TaskC     = Def{Method: "POST", Route: ":428/task-c"}      // MARKER: TaskC
	RetryLoop = Def{Method: "GET", Route: ":428/retry-loop"}   // MARKER: RetryLoop
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Target int `json:"target,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	TargetOut int `json:"target,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Attempts int `json:"attempts,omitzero"`
	Target   int `json:"target,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	Succeeded bool `json:"succeeded,omitzero"`
}

// HandlerIn are the input arguments of Handler.
type HandlerIn struct { // MARKER: Handler
	OnErr    *errors.TracedError `json:"onErr,omitzero"`
	Attempts int                 `json:"attempts,omitzero"`
}

// HandlerOut are the output arguments of Handler.
type HandlerOut struct { // MARKER: Handler
	AttemptsOut int `json:"attempts,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Attempts int `json:"attempts,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	FinalAttempts int `json:"finalAttempts,omitzero"`
}

// RetryLoopIn are the input arguments of RetryLoop.
type RetryLoopIn struct { // MARKER: RetryLoop
	Target int `json:"target,omitzero"`
}

// RetryLoopOut are the output arguments of RetryLoop.
type RetryLoopOut struct { // MARKER: RetryLoop
	FinalAttempts int `json:"finalAttempts,omitzero"`
}

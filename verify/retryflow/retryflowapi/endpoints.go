package retryflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "retryflow.verify"

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
	TaskA = Def{Method: "POST", Route: ":428/task-a"} // MARKER: TaskA
	Flaky = Def{Method: "POST", Route: ":428/flaky"}  // MARKER: Flaky
	TaskB = Def{Method: "POST", Route: ":428/task-b"} // MARKER: TaskB
	Retry = Def{Method: "GET", Route: ":428/retry"}   // MARKER: Retry
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Target int `json:"target,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	TargetOut int `json:"target,omitzero"`
}

// FlakyIn are the input arguments of Flaky.
type FlakyIn struct { // MARKER: Flaky
	Attempts int `json:"attempts,omitzero"`
	Target   int `json:"target,omitzero"`
}

// FlakyOut are the output arguments of Flaky.
type FlakyOut struct { // MARKER: Flaky
	AttemptsOut int `json:"attempts,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Attempts int `json:"attempts,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	FinalAttempts int `json:"finalAttempts,omitzero"`
}

// RetryIn are the input arguments of Retry.
type RetryIn struct { // MARKER: Retry
	Target int `json:"target,omitzero"`
}

// RetryOut are the output arguments of Retry.
type RetryOut struct { // MARKER: Retry
	FinalAttempts int `json:"finalAttempts,omitzero"`
}

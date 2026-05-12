package sleepflowapi

import (
	"time"

	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "sleepflow.verify"

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
	TaskB = Def{Method: "POST", Route: ":428/task-b"} // MARKER: TaskB
	TaskC = Def{Method: "POST", Route: ":428/task-c"} // MARKER: TaskC
	// Workflow named Delay to avoid colliding with connector.Sleep.
	Delay = Def{Method: "GET", Route: ":428/delay"} // MARKER: Delay
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	SleepFor time.Duration `json:"sleepFor,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	SleepForOut time.Duration `json:"sleepFor,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	SleepFor time.Duration `json:"sleepFor,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	Marked bool `json:"marked,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Marked bool `json:"marked,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	Completed bool `json:"completed,omitzero"`
}

// DelayIn are the input arguments of Delay.
type DelayIn struct { // MARKER: Delay
	SleepFor time.Duration `json:"sleepFor,omitzero"`
}

// DelayOut are the output arguments of Delay.
type DelayOut struct { // MARKER: Delay
	Completed bool `json:"completed,omitzero"`
}

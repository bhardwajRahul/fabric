package reducerflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "reducerflow.verify"

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
	TaskA   = Def{Method: "POST", Route: ":428/task-a"}  // MARKER: TaskA
	TaskB   = Def{Method: "POST", Route: ":428/task-b"}  // MARKER: TaskB
	TaskC   = Def{Method: "POST", Route: ":428/task-c"}  // MARKER: TaskC
	TaskD   = Def{Method: "POST", Route: ":428/task-d"}  // MARKER: TaskD
	TaskE   = Def{Method: "POST", Route: ":428/task-e"}  // MARKER: TaskE
	Reducer = Def{Method: "GET", Route: ":428/reducer"}  // MARKER: Reducer
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
}

// TaskBOut are the output arguments of TaskB. B contributes deltas to all three reducer fields.
type TaskBOut struct { // MARKER: TaskB
	SumTotalOut int      `json:"sumTotal,omitzero"`
	ListTagsOut []string `json:"listTags,omitzero"`
	SetSeenOut  []string `json:"setSeen,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	SumTotalOut int      `json:"sumTotal,omitzero"`
	ListTagsOut []string `json:"listTags,omitzero"`
	SetSeenOut  []string `json:"setSeen,omitzero"`
}

// TaskDIn are the input arguments of TaskD.
type TaskDIn struct { // MARKER: TaskD
}

// TaskDOut are the output arguments of TaskD.
type TaskDOut struct { // MARKER: TaskD
	SumTotalOut int      `json:"sumTotal,omitzero"`
	ListTagsOut []string `json:"listTags,omitzero"`
	SetSeenOut  []string `json:"setSeen,omitzero"`
}

// TaskEIn are the input arguments of TaskE.
type TaskEIn struct { // MARKER: TaskE
	SumTotal int      `json:"sumTotal,omitzero"`
	ListTags []string `json:"listTags,omitzero"`
	SetSeen  []string `json:"setSeen,omitzero"`
}

// TaskEOut are the output arguments of TaskE.
type TaskEOut struct { // MARKER: TaskE
	FinalSum  int      `json:"finalSum,omitzero"`
	FinalList []string `json:"finalList,omitzero"`
	FinalSet  []string `json:"finalSet,omitzero"`
}

// ReducerIn are the input arguments of Reducer.
type ReducerIn struct { // MARKER: Reducer
}

// ReducerOut are the output arguments of Reducer.
type ReducerOut struct { // MARKER: Reducer
	FinalSum  int      `json:"finalSum,omitzero"`
	FinalList []string `json:"finalList,omitzero"`
	FinalSet  []string `json:"finalSet,omitzero"`
}

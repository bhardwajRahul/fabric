/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package conditionalflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "conditionalflow.verify"

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
	TaskA       = Def{Method: "POST", Route: ":428/task-a"}        // MARKER: TaskA
	TaskHigh    = Def{Method: "POST", Route: ":428/task-high"}     // MARKER: TaskHigh
	TaskLow     = Def{Method: "POST", Route: ":428/task-low"}     // MARKER: TaskLow
	TaskC       = Def{Method: "POST", Route: ":428/task-c"}        // MARKER: TaskC
	Conditional = Def{Method: "GET", Route: ":428/conditional"}    // MARKER: Conditional
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Score int `json:"score,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	ScoreOut int `json:"score,omitzero"`
}

// TaskHighIn are the input arguments of TaskHigh.
type TaskHighIn struct { // MARKER: TaskHigh
	Score int `json:"score,omitzero"`
}

// TaskHighOut are the output arguments of TaskHigh.
type TaskHighOut struct { // MARKER: TaskHigh
	Branch string `json:"branch,omitzero"`
}

// TaskLowIn are the input arguments of TaskLow.
type TaskLowIn struct { // MARKER: TaskLow
	Score int `json:"score,omitzero"`
}

// TaskLowOut are the output arguments of TaskLow.
type TaskLowOut struct { // MARKER: TaskLow
	Branch string `json:"branch,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Branch string `json:"branch,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	FinalBranch string `json:"finalBranch,omitzero"`
}

// ConditionalIn are the input arguments of Conditional.
type ConditionalIn struct { // MARKER: Conditional
	Score int `json:"score,omitzero"`
}

// ConditionalOut are the output arguments of Conditional.
type ConditionalOut struct { // MARKER: Conditional
	FinalBranch string `json:"finalBranch,omitzero"`
}

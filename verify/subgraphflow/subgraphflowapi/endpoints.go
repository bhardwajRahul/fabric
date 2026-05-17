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

package subgraphflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "subgraphflow.verify"

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
	TaskA  = Def{Method: "POST", Route: ":428/task-a"} // MARKER: TaskA
	TaskX  = Def{Method: "POST", Route: ":428/task-x"} // MARKER: TaskX
	TaskY  = Def{Method: "POST", Route: ":428/task-y"} // MARKER: TaskY
	TaskZ  = Def{Method: "POST", Route: ":428/task-z"} // MARKER: TaskZ
	Inner  = Def{Method: "GET", Route: ":428/inner"}   // MARKER: Inner
	Parent = Def{Method: "GET", Route: ":428/parent"}  // MARKER: Parent
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Seed string `json:"seed,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	SeedOut string `json:"seed,omitzero"`
}

// TaskXIn are the input arguments of TaskX. Reads `seed` from parent state via DeclareInputs.
type TaskXIn struct { // MARKER: TaskX
	Seed string `json:"seed,omitzero"`
}

// TaskXOut are the output arguments of TaskX.
type TaskXOut struct { // MARKER: TaskX
	InnerStage string `json:"innerStage,omitzero"`
}

// TaskYIn are the input arguments of TaskY.
type TaskYIn struct { // MARKER: TaskY
	InnerStage string `json:"innerStage,omitzero"`
}

// TaskYOut are the output arguments of TaskY.
type TaskYOut struct { // MARKER: TaskY
	InnerResult string `json:"innerResult,omitzero"`
}

// TaskZIn are the input arguments of TaskZ. Reads `innerResult` from the subgraph's output.
type TaskZIn struct { // MARKER: TaskZ
	InnerResult string `json:"innerResult,omitzero"`
}

// TaskZOut are the output arguments of TaskZ.
type TaskZOut struct { // MARKER: TaskZ
	FinalResult string `json:"finalResult,omitzero"`
}

// InnerIn are the input arguments of Inner. Declared inputs: `seed`.
type InnerIn struct { // MARKER: Inner
	Seed string `json:"seed,omitzero"`
}

// InnerOut are the output arguments of Inner. Declared outputs: `innerResult`.
type InnerOut struct { // MARKER: Inner
	InnerResult string `json:"innerResult,omitzero"`
}

// ParentIn are the input arguments of Parent.
type ParentIn struct { // MARKER: Parent
	Seed string `json:"seed,omitzero"`
}

// ParentOut are the output arguments of Parent.
type ParentOut struct { // MARKER: Parent
	FinalResult string `json:"finalResult,omitzero"`
}

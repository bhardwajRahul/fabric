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

package nestedfailfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "nestedfailfanoutflow.verify"

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
	TaskA  = Def{Method: "POST", Route: ":428/task-a"}  // MARKER: TaskA
	TaskO  = Def{Method: "POST", Route: ":428/task-o"}  // MARKER: TaskO
	TaskI  = Def{Method: "POST", Route: ":428/task-i"}  // MARKER: TaskI
	JoinI  = Def{Method: "POST", Route: ":428/join-i"}  // MARKER: JoinI
	JoinO  = Def{Method: "POST", Route: ":428/join-o"}  // MARKER: JoinO
	Nested = Def{Method: "GET", Route: ":428/nested"}   // MARKER: Nested
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	Outers []int `json:"outers,omitzero"`
}

// TaskOIn are the input arguments of TaskO.
type TaskOIn struct { // MARKER: TaskO
	OuterItem int `json:"outerItem,omitzero"`
}

// TaskOOut are the output arguments of TaskO.
type TaskOOut struct { // MARKER: TaskO
	Inners       []int `json:"inners,omitzero"`
	CurrentOuter int   `json:"currentOuter,omitzero"`
}

// TaskIIn are the input arguments of TaskI.
type TaskIIn struct { // MARKER: TaskI
	CurrentOuter int `json:"currentOuter,omitzero"`
	InnerItem    int `json:"innerItem,omitzero"`
}

// TaskIOut are the output arguments of TaskI.
type TaskIOut struct { // MARKER: TaskI
}

// JoinIIn are the input arguments of JoinI.
type JoinIIn struct { // MARKER: JoinI
}

// JoinIOut are the output arguments of JoinI.
type JoinIOut struct { // MARKER: JoinI
}

// JoinOIn are the input arguments of JoinO.
type JoinOIn struct { // MARKER: JoinO
}

// JoinOOut are the output arguments of JoinO.
type JoinOOut struct { // MARKER: JoinO
	Done bool `json:"done,omitzero"`
}

// NestedIn are the input arguments of Nested.
type NestedIn struct { // MARKER: Nested
}

// NestedOut are the output arguments of Nested.
type NestedOut struct { // MARKER: Nested
	Done bool `json:"done,omitzero"`
}

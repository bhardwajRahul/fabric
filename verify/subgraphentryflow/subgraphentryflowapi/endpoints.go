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

package subgraphentryflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "subgraphentryflow.verify"

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
	TaskInner = Def{Method: "POST", Route: ":428/task-inner"} // MARKER: TaskInner
	TaskTail  = Def{Method: "POST", Route: ":428/task-tail"}  // MARKER: TaskTail
	RunInner  = Def{Method: "POST", Route: ":428/run-inner"}  // MARKER: RunInner
	RunTail   = Def{Method: "POST", Route: ":428/run-tail"}   // MARKER: RunTail
	Inner     = Def{Method: "GET", Route: ":428/inner"}       // MARKER: Inner
	Tail      = Def{Method: "GET", Route: ":428/tail"}        // MARKER: Tail
	Outer     = Def{Method: "GET", Route: ":428/outer"}       // MARKER: Outer
)

// TaskInnerIn are the input arguments of TaskInner.
type TaskInnerIn struct { // MARKER: TaskInner
}

// TaskInnerOut are the output arguments of TaskInner.
type TaskInnerOut struct { // MARKER: TaskInner
	InnerResult string `json:"innerResult,omitzero"`
}

// TaskTailIn are the input arguments of TaskTail.
type TaskTailIn struct { // MARKER: TaskTail
	InnerResult string `json:"innerResult,omitzero"`
}

// TaskTailOut are the output arguments of TaskTail.
type TaskTailOut struct { // MARKER: TaskTail
	FinalResult string `json:"finalResult,omitzero"`
}

// RunInnerIn are the input arguments of RunInner.
type RunInnerIn struct { // MARKER: RunInner
}

// RunInnerOut are the output arguments of RunInner.
type RunInnerOut struct { // MARKER: RunInner
	InnerResult string `json:"innerResult,omitzero"`
}

// RunTailIn are the input arguments of RunTail.
type RunTailIn struct { // MARKER: RunTail
	InnerResult string `json:"innerResult,omitzero"`
}

// RunTailOut are the output arguments of RunTail.
type RunTailOut struct { // MARKER: RunTail
	FinalResult string `json:"finalResult,omitzero"`
}

// InnerIn are the input arguments of Inner.
type InnerIn struct { // MARKER: Inner
}

// InnerOut are the output arguments of Inner.
type InnerOut struct { // MARKER: Inner
	InnerResult string `json:"innerResult,omitzero"`
}

// TailIn are the input arguments of Tail.
type TailIn struct { // MARKER: Tail
	InnerResult string `json:"innerResult,omitzero"`
}

// TailOut are the output arguments of Tail.
type TailOut struct { // MARKER: Tail
	FinalResult string `json:"finalResult,omitzero"`
}

// OuterIn are the input arguments of Outer.
type OuterIn struct { // MARKER: Outer
}

// OuterOut are the output arguments of Outer.
type OuterOut struct { // MARKER: Outer
	FinalResult string `json:"finalResult,omitzero"`
}

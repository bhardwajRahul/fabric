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

package nestedfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "nestedfanoutflow.verify"

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
	TaskA   = Def{Method: "POST", Route: ":428/task-a"}    // MARKER: TaskA
	NormalB = Def{Method: "POST", Route: ":428/normal-b"}  // MARKER: NormalB
	TaskX   = Def{Method: "POST", Route: ":428/task-x"}    // MARKER: TaskX
	TaskY   = Def{Method: "POST", Route: ":428/task-y"}    // MARKER: TaskY
	TaskZ   = Def{Method: "POST", Route: ":428/task-z"}    // MARKER: TaskZ
	TaskW   = Def{Method: "POST", Route: ":428/task-w"}    // MARKER: TaskW
	TaskJ    = Def{Method: "POST", Route: ":428/task-j"}     // MARKER: TaskJ
	RunInner = Def{Method: "POST", Route: ":428/run-inner"}  // MARKER: RunInner
	Inner    = Def{Method: "GET", Route: ":428/inner"}       // MARKER: Inner
	Nested   = Def{Method: "GET", Route: ":428/nested"}      // MARKER: Nested
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	Started bool `json:"started,omitzero"`
}

// NormalBIn are the input arguments of NormalB.
type NormalBIn struct { // MARKER: NormalB
}

// NormalBOut are the output arguments of NormalB.
type NormalBOut struct { // MARKER: NormalB
	NormalResult string `json:"normalResult,omitzero"`
}

// RunInnerIn are the input arguments of RunInner.
type RunInnerIn struct { // MARKER: RunInner
}

// RunInnerOut are the output arguments of RunInner.
type RunInnerOut struct { // MARKER: RunInner
	InnerResult int `json:"innerResult,omitzero"`
}

// TaskXIn are the input arguments of TaskX.
type TaskXIn struct { // MARKER: TaskX
}

// TaskXOut are the output arguments of TaskX.
type TaskXOut struct { // MARKER: TaskX
	InnerStarted bool `json:"innerStarted,omitzero"`
}

// TaskYIn are the input arguments of TaskY.
type TaskYIn struct { // MARKER: TaskY
}

// TaskYOut are the output arguments of TaskY. Contributes a delta to the sum reducer.
type TaskYOut struct { // MARKER: TaskY
	InnerOut int `json:"inner,omitzero"`
}

// TaskZIn are the input arguments of TaskZ.
type TaskZIn struct { // MARKER: TaskZ
}

// TaskZOut are the output arguments of TaskZ. Contributes a delta to the sum reducer.
type TaskZOut struct { // MARKER: TaskZ
	InnerOut int `json:"inner,omitzero"`
}

// TaskWIn are the input arguments of TaskW. Reads the merged `inner` field.
type TaskWIn struct { // MARKER: TaskW
	Inner int `json:"inner,omitzero"`
}

// TaskWOut are the output arguments of TaskW.
type TaskWOut struct { // MARKER: TaskW
	InnerResult int `json:"innerResult,omitzero"`
}

// TaskJIn are the input arguments of TaskJ. Reads both branches' results.
type TaskJIn struct { // MARKER: TaskJ
	NormalResult string `json:"normalResult,omitzero"`
	InnerResult  int    `json:"innerResult,omitzero"`
}

// TaskJOut are the output arguments of TaskJ.
type TaskJOut struct { // MARKER: TaskJ
	FinalResult string `json:"finalResult,omitzero"`
}

// InnerIn are the input arguments of Inner.
type InnerIn struct { // MARKER: Inner
}

// InnerOut are the output arguments of Inner.
type InnerOut struct { // MARKER: Inner
	InnerResult int `json:"innerResult,omitzero"`
}

// NestedIn are the input arguments of Nested.
type NestedIn struct { // MARKER: Nested
}

// NestedOut are the output arguments of Nested.
type NestedOut struct { // MARKER: Nested
	FinalResult string `json:"finalResult,omitzero"`
}

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

package forkflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "forkflow.verify"

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
	Pipe  = Def{Method: "GET", Route: ":428/pipe"}    // MARKER: Pipe
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Value int `json:"value,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	ValueOut int `json:"value,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Value int `json:"value,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	ValueOut int `json:"value,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Value int `json:"value,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	ValueOut int `json:"value,omitzero"`
}

// PipeIn are the input arguments of Pipe.
type PipeIn struct { // MARKER: Pipe
	Value int `json:"value,omitzero"`
}

// PipeOut are the output arguments of Pipe.
type PipeOut struct { // MARKER: Pipe
	ValueOut int `json:"value,omitzero"`
}

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

package basicflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "basicflow.verify"

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
	Basic = Def{Method: "GET", Route: ":428/basic"}   // MARKER: Basic
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	Path string `json:"path,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Path string `json:"path,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	PathOut string `json:"path,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Path string `json:"path,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	PathOut string `json:"path,omitzero"`
}

// BasicIn are the input arguments of Basic.
type BasicIn struct { // MARKER: Basic
}

// BasicOut are the output arguments of Basic.
type BasicOut struct { // MARKER: Basic
	Path string `json:"path,omitzero"`
}

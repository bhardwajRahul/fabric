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

package aliasflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "aliasflow.verify"

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
	TaskS = Def{Method: "POST", Route: ":428/task-s"} // MARKER: TaskS
	TaskA = Def{Method: "POST", Route: ":428/task-a"} // MARKER: TaskA
	TaskB = Def{Method: "POST", Route: ":428/task-b"} // MARKER: TaskB
	TaskC = Def{Method: "POST", Route: ":428/task-c"} // MARKER: TaskC
	TaskD = Def{Method: "POST", Route: ":428/task-d"} // MARKER: TaskD
	Alias = Def{Method: "GET", Route: ":428/alias"}   // MARKER: Alias
)

// TaskSIn are the input arguments of TaskS.
type TaskSIn struct { // MARKER: TaskS
	Branch string `json:"branch,omitzero"`
}

// TaskSOut are the output arguments of TaskS.
type TaskSOut struct { // MARKER: TaskS
	BranchOut string `json:"branch,omitzero"`
}

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Path string `json:"path,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	PathOut string `json:"path,omitzero"`
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

// TaskDIn are the input arguments of TaskD.
type TaskDIn struct { // MARKER: TaskD
	Path string `json:"path,omitzero"`
}

// TaskDOut are the output arguments of TaskD.
type TaskDOut struct { // MARKER: TaskD
	PathOut string `json:"path,omitzero"`
}

// AliasIn are the input arguments of Alias.
type AliasIn struct { // MARKER: Alias
	Branch string `json:"branch,omitzero"`
}

// AliasOut are the output arguments of Alias.
type AliasOut struct { // MARKER: Alias
	Path string `json:"path,omitzero"`
}

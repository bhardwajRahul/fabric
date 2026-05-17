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

package errorflowapi

import (
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "errorflow.verify"

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
	TaskA   = Def{Method: "POST", Route: ":428/task-a"}   // MARKER: TaskA
	TaskB   = Def{Method: "POST", Route: ":428/task-b"}   // MARKER: TaskB
	Handler = Def{Method: "POST", Route: ":428/handler"}  // MARKER: Handler
	TaskC   = Def{Method: "POST", Route: ":428/task-c"}   // MARKER: TaskC
	Error   = Def{Method: "GET", Route: ":428/error"}     // MARKER: Error
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Trigger string `json:"trigger,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	TriggerOut string `json:"trigger,omitzero"`
}

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct { // MARKER: TaskB
	Trigger string `json:"trigger,omitzero"`
}

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct { // MARKER: TaskB
	Result string `json:"result,omitzero"`
}

// HandlerIn are the input arguments of Handler.
type HandlerIn struct { // MARKER: Handler
	OnErr *errors.TracedError `json:"onErr,omitzero"`
}

// HandlerOut are the output arguments of Handler.
type HandlerOut struct { // MARKER: Handler
	Result string `json:"result,omitzero"`
}

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct { // MARKER: TaskC
	Result string `json:"result,omitzero"`
}

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct { // MARKER: TaskC
	FinalResult string `json:"finalResult,omitzero"`
}

// ErrorIn are the input arguments of Error.
type ErrorIn struct { // MARKER: Error
	Trigger string `json:"trigger,omitzero"`
}

// ErrorOut are the output arguments of Error.
type ErrorOut struct { // MARKER: Error
	FinalResult string `json:"finalResult,omitzero"`
}

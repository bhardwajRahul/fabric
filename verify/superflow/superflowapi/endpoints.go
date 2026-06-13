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

package superflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "superflow.verify"

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
	TaskA        = Def{Method: "POST", Route: ":428/task-a"}        // MARKER: TaskA
	TaskB        = Def{Method: "POST", Route: ":428/task-b"}        // MARKER: TaskB
	TaskC        = Def{Method: "POST", Route: ":428/task-c"}        // MARKER: TaskC
	TaskD        = Def{Method: "POST", Route: ":428/task-d"}        // MARKER: TaskD
	TaskE        = Def{Method: "POST", Route: ":428/task-e"}        // MARKER: TaskE
	TaskZ        = Def{Method: "POST", Route: ":428/task-z"}        // MARKER: TaskZ
	ErrorHandler = Def{Method: "POST", Route: ":428/error-handler"} // MARKER: ErrorHandler
	SubTaskA     = Def{Method: "POST", Route: ":428/sub-task-a"}    // MARKER: SubTaskA
	SubTaskB     = Def{Method: "POST", Route: ":428/sub-task-b"}    // MARKER: SubTaskB
	RunSuperSub  = Def{Method: "POST", Route: ":428/run-super-sub"} // MARKER: RunSuperSub
	Super        = Def{Method: "GET", Route: ":428/super"}          // MARKER: Super
	SuperSub     = Def{Method: "GET", Route: ":428/super-sub"}      // MARKER: SuperSub
)

// TaskBehavior parameterizes one task's runtime behavior. Tests set
// behaviors per task to drive specific control paths (sleep, error, interrupt,
// goto, retry) without changing the graph.
type TaskBehavior struct {
	SleepMs     int    `json:"sleepMs,omitzero" jsonschema:"description=SleepMs delays the task by this many milliseconds before returning"`
	ErrorStatus int    `json:"errorStatus,omitzero" jsonschema:"description=ErrorStatus returns an error with this HTTP status code"`
	Interrupt   bool   `json:"interrupt,omitzero" jsonschema:"description=Interrupt calls flow.Interrupt(payload) instead of returning normally"`
	Goto        string `json:"goto,omitzero" jsonschema:"description=Goto calls flow.Goto(target) before returning"`
	Retry       bool   `json:"retry,omitzero" jsonschema:"description=Retry calls flow.Retry with an unlimited cap and returns nil"`
}

// SuperflowState is the shape of the Super workflow's shared state.
// Tasks read it via flow.ParseState to decide their behavior.
type SuperflowState struct {
	Items       []string                `json:"items,omitzero" jsonschema:"description=Items drives TaskB's forEach fan-out into TaskC"`
	UseSubgraph bool                    `json:"useSubgraph,omitzero" jsonschema:"description=UseSubgraph routes TaskD through SuperSub when true"`
	Behaviors   map[string]TaskBehavior `json:"behaviors,omitzero" jsonschema:"description=Behaviors maps a task name to a per-task behavior knob"`
}

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct{} // MARKER: TaskA

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct{} // MARKER: TaskA

// TaskBIn are the input arguments of TaskB.
type TaskBIn struct{} // MARKER: TaskB

// TaskBOut are the output arguments of TaskB.
type TaskBOut struct{} // MARKER: TaskB

// TaskCIn are the input arguments of TaskC.
type TaskCIn struct{} // MARKER: TaskC

// TaskCOut are the output arguments of TaskC.
type TaskCOut struct{} // MARKER: TaskC

// TaskDIn are the input arguments of TaskD.
type TaskDIn struct{} // MARKER: TaskD

// TaskDOut are the output arguments of TaskD.
type TaskDOut struct{} // MARKER: TaskD

// TaskEIn are the input arguments of TaskE.
type TaskEIn struct{} // MARKER: TaskE

// TaskEOut are the output arguments of TaskE.
type TaskEOut struct{} // MARKER: TaskE

// TaskZIn are the input arguments of TaskZ.
type TaskZIn struct{} // MARKER: TaskZ

// TaskZOut are the output arguments of TaskZ.
type TaskZOut struct{} // MARKER: TaskZ

// ErrorHandlerIn are the input arguments of ErrorHandler.
type ErrorHandlerIn struct{} // MARKER: ErrorHandler

// ErrorHandlerOut are the output arguments of ErrorHandler.
type ErrorHandlerOut struct{} // MARKER: ErrorHandler

// SubTaskAIn are the input arguments of SubTaskA.
type SubTaskAIn struct{} // MARKER: SubTaskA

// SubTaskAOut are the output arguments of SubTaskA.
type SubTaskAOut struct{} // MARKER: SubTaskA

// SubTaskBIn are the input arguments of SubTaskB.
type SubTaskBIn struct{} // MARKER: SubTaskB

// SubTaskBOut are the output arguments of SubTaskB.
type SubTaskBOut struct{} // MARKER: SubTaskB

// RunSuperSubIn are the input arguments of RunSuperSub.
type RunSuperSubIn struct{} // MARKER: RunSuperSub

// RunSuperSubOut are the output arguments of RunSuperSub.
type RunSuperSubOut struct{} // MARKER: RunSuperSub

// SuperIn are the input arguments of the Super workflow.
type SuperIn struct { // MARKER: Super
	Items       []string                `json:"items,omitzero"`
	UseSubgraph bool                    `json:"useSubgraph,omitzero"`
	Behaviors   map[string]TaskBehavior `json:"behaviors,omitzero"`
}

// SuperOut are the output arguments of the Super workflow.
type SuperOut struct{} // MARKER: Super

// SuperSubIn are the input arguments of the SuperSub workflow.
type SuperSubIn struct { // MARKER: SuperSub
	Behaviors map[string]TaskBehavior `json:"behaviors,omitzero"`
}

// SuperSubOut are the output arguments of the SuperSub workflow.
type SuperSubOut struct{} // MARKER: SuperSub

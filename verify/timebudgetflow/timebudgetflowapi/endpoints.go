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

package timebudgetflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "timebudgetflow.verify"

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
	TaskA      = Def{Method: "POST", Route: ":428/task-a"}      // MARKER: TaskA
	Slow       = Def{Method: "POST", Route: ":428/slow"}        // MARKER: Slow
	TimeBudget = Def{Method: "GET", Route: ":428/time-budget"}  // MARKER: TimeBudget
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	Started bool `json:"started,omitzero"`
}

// SlowIn are the input arguments of Slow.
type SlowIn struct { // MARKER: Slow
}

// SlowOut are the output arguments of Slow.
type SlowOut struct { // MARKER: Slow
	Done bool `json:"done,omitzero"`
}

// TimeBudgetIn are the input arguments of TimeBudget.
type TimeBudgetIn struct { // MARKER: TimeBudget
}

// TimeBudgetOut are the output arguments of TimeBudget.
type TimeBudgetOut struct { // MARKER: TimeBudget
	Done bool `json:"done,omitzero"`
}

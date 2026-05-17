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

package subgraphfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "subgraphfanoutflow.verify"

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
	TaskA       = Def{Method: "POST", Route: ":428/task-a"}        // MARKER: TaskA
	NormalB     = Def{Method: "POST", Route: ":428/normal-b"}      // MARKER: NormalB
	TaskX       = Def{Method: "POST", Route: ":428/task-x"}        // MARKER: TaskX
	TaskY       = Def{Method: "POST", Route: ":428/task-y"}        // MARKER: TaskY
	NormalD     = Def{Method: "POST", Route: ":428/normal-d"}      // MARKER: NormalD
	TaskE       = Def{Method: "POST", Route: ":428/task-e"}        // MARKER: TaskE
	Sub         = Def{Method: "GET", Route: ":428/sub"}            // MARKER: Sub
	SubFanOut   = Def{Method: "GET", Route: ":428/sub-fan-out"}    // MARKER: SubFanOut
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
	ResultB string `json:"resultB,omitzero"`
}

// TaskXIn are the input arguments of TaskX.
type TaskXIn struct { // MARKER: TaskX
}

// TaskXOut are the output arguments of TaskX.
type TaskXOut struct { // MARKER: TaskX
	XPassed bool `json:"xPassed,omitzero"`
}

// TaskYIn are the input arguments of TaskY.
type TaskYIn struct { // MARKER: TaskY
	XPassed bool `json:"xPassed,omitzero"`
}

// TaskYOut are the output arguments of TaskY.
type TaskYOut struct { // MARKER: TaskY
	SubResult string `json:"subResult,omitzero"`
}

// NormalDIn are the input arguments of NormalD.
type NormalDIn struct { // MARKER: NormalD
}

// NormalDOut are the output arguments of NormalD.
type NormalDOut struct { // MARKER: NormalD
	ResultD string `json:"resultD,omitzero"`
}

// TaskEIn are the input arguments of TaskE.
type TaskEIn struct { // MARKER: TaskE
	ResultB   string `json:"resultB,omitzero"`
	SubResult string `json:"subResult,omitzero"`
	ResultD   string `json:"resultD,omitzero"`
}

// TaskEOut are the output arguments of TaskE.
type TaskEOut struct { // MARKER: TaskE
	FinalResult string `json:"finalResult,omitzero"`
}

// SubIn are the input arguments of Sub.
type SubIn struct { // MARKER: Sub
}

// SubOut are the output arguments of Sub.
type SubOut struct { // MARKER: Sub
	SubResult string `json:"subResult,omitzero"`
}

// SubFanOutIn are the input arguments of SubFanOut.
type SubFanOutIn struct { // MARKER: SubFanOut
}

// SubFanOutOut are the output arguments of SubFanOut.
type SubFanOutOut struct { // MARKER: SubFanOut
	FinalResult string `json:"finalResult,omitzero"`
}

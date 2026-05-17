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

package interruptedfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "interruptedfanoutflow.verify"

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
	Src               = Def{Method: "POST", Route: ":428/src"}                    // MARKER: Src
	A                 = Def{Method: "POST", Route: ":428/a"}                      // MARKER: A
	B                 = Def{Method: "POST", Route: ":428/b"}                      // MARKER: B
	C                 = Def{Method: "POST", Route: ":428/c"}                      // MARKER: C
	J                 = Def{Method: "POST", Route: ":428/j"}                      // MARKER: J
	InterruptedFanOut = Def{Method: "GET", Route: ":428/interrupted-fan-out"}     // MARKER: InterruptedFanOut
)

// SrcIn are the input arguments of Src.
type SrcIn struct { // MARKER: Src
}

// SrcOut are the output arguments of Src.
type SrcOut struct { // MARKER: Src
	Started bool `json:"started,omitzero"`
}

// AIn are the input arguments of A.
type AIn struct { // MARKER: A
}

// AOut are the output arguments of A.
type AOut struct { // MARKER: A
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// BIn are the input arguments of B.
type BIn struct { // MARKER: B
	Resumed bool `json:"resumed,omitzero"`
}

// BOut are the output arguments of B.
type BOut struct { // MARKER: B
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// CIn are the input arguments of C.
type CIn struct { // MARKER: C
}

// COut are the output arguments of C.
type COut struct { // MARKER: C
	SumExecutedOut int `json:"sumExecuted,omitzero"`
}

// JIn are the input arguments of J.
type JIn struct { // MARKER: J
	SumExecuted int `json:"sumExecuted,omitzero"`
}

// JOut are the output arguments of J.
type JOut struct { // MARKER: J
	TotalExecuted int `json:"totalExecuted,omitzero"`
}

// InterruptedFanOutIn are the input arguments of InterruptedFanOut.
type InterruptedFanOutIn struct { // MARKER: InterruptedFanOut
}

// InterruptedFanOutOut are the output arguments of InterruptedFanOut.
type InterruptedFanOutOut struct { // MARKER: InterruptedFanOut
	SumExecuted   int `json:"sumExecuted,omitzero"`
	TotalExecuted int `json:"totalExecuted,omitzero"`
}

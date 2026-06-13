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

package cancelledfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "cancelledfanoutflow.verify"

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
	Source         = Def{Method: "POST", Route: ":428/source"}              // MARKER: Source
	A              = Def{Method: "POST", Route: ":428/a"}                   // MARKER: A
	B              = Def{Method: "POST", Route: ":428/b"}                   // MARKER: B
	C              = Def{Method: "POST", Route: ":428/c"}                   // MARKER: C
	J              = Def{Method: "POST", Route: ":428/j"}                   // MARKER: J
	CancelledFanOut = Def{Method: "GET", Route: ":428/cancelled-fan-out"}   // MARKER: CancelledFanOut
)

// SourceIn are the input arguments of Source.
type SourceIn struct { // MARKER: Source
}

// SourceOut are the output arguments of Source.
type SourceOut struct { // MARKER: Source
	Started bool `json:"started,omitzero"`
}

// AIn are the input arguments of A.
type AIn struct { // MARKER: A
}

// AOut are the output arguments of A.
type AOut struct { // MARKER: A
	ExecutedOut int `json:"executed,omitzero"`
}

// BIn are the input arguments of B.
type BIn struct { // MARKER: B
}

// BOut are the output arguments of B.
type BOut struct { // MARKER: B
	ExecutedOut int `json:"executed,omitzero"`
}

// CIn are the input arguments of C.
type CIn struct { // MARKER: C
}

// COut are the output arguments of C.
type COut struct { // MARKER: C
	ExecutedOut int `json:"executed,omitzero"`
}

// JIn are the input arguments of J.
type JIn struct { // MARKER: J
	Executed int `json:"executed,omitzero"`
}

// JOut are the output arguments of J.
type JOut struct { // MARKER: J
	TotalExecuted int `json:"totalExecuted,omitzero"`
}

// CancelledFanOutIn are the input arguments of CancelledFanOut.
type CancelledFanOutIn struct { // MARKER: CancelledFanOut
}

// CancelledFanOutOut are the output arguments of CancelledFanOut.
type CancelledFanOutOut struct { // MARKER: CancelledFanOut
	Executed      int `json:"executed,omitzero"`
	TotalExecuted int `json:"totalExecuted,omitzero"`
}

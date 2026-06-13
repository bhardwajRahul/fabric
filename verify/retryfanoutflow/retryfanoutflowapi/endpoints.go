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

package retryfanoutflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "retryfanoutflow.verify"

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
	Enter       = Def{Method: "POST", Route: ":428/enter"}            // MARKER: Enter
	Increment   = Def{Method: "POST", Route: ":428/increment"}        // MARKER: Increment
	Join        = Def{Method: "POST", Route: ":428/join"}             // MARKER: Join
	RetryFanOut = Def{Method: "GET", Route: ":428/retry-fan-out"}     // MARKER: RetryFanOut
)

// EnterIn are the input arguments of Enter.
type EnterIn struct { // MARKER: Enter
	Elements []int `json:"elements,omitzero"`
}

// EnterOut are the output arguments of Enter.
type EnterOut struct { // MARKER: Enter
	ElementsOut []int `json:"elements,omitzero"`
}

// IncrementIn are the input arguments of Increment.
type IncrementIn struct { // MARKER: Increment
	Element int `json:"element,omitzero"`
}

// IncrementOut are the output arguments of Increment.
type IncrementOut struct { // MARKER: Increment
	ResultsOut []int `json:"results,omitzero"`
}

// JoinIn are the input arguments of Join.
type JoinIn struct { // MARKER: Join
	Results []int `json:"results,omitzero"`
}

// JoinOut are the output arguments of Join.
type JoinOut struct { // MARKER: Join
	ResultsOut []int `json:"results,omitzero"`
}

// RetryFanOutIn are the input arguments of RetryFanOut.
type RetryFanOutIn struct { // MARKER: RetryFanOut
	Elements []int `json:"elements,omitzero"`
}

// RetryFanOutOut are the output arguments of RetryFanOut.
type RetryFanOutOut struct { // MARKER: RetryFanOut
	Results []int `json:"results,omitzero"`
}

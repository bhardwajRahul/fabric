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

package cancelbackpressureflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "cancelbackpressureflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// BounceAndCancelIn are the input arguments of BounceAndCancel.
type BounceAndCancelIn struct { // MARKER: BounceAndCancel
	Tag string `json:"tag,omitzero"`
}

// BounceAndCancelOut are the output arguments of BounceAndCancel.
type BounceAndCancelOut struct { // MARKER: BounceAndCancel
	Tallied bool `json:"tallied,omitzero"`
}

// CancelBackpressureIn are the input arguments of CancelBackpressure.
type CancelBackpressureIn struct { // MARKER: CancelBackpressure
	Tag string `json:"tag,omitzero"`
}

// CancelBackpressureOut are the output arguments of CancelBackpressure.
type CancelBackpressureOut struct { // MARKER: CancelBackpressure
	Tallied bool `json:"tallied,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	BounceAndCancel            = Def{Method: "POST", Route: ":428/bounce-and-cancel"}             // MARKER: BounceAndCancel
	CancelBackpressure = Def{Method: "GET", Route: ":428/cancel-backpressure"}            // MARKER: CancelBackpressure
)

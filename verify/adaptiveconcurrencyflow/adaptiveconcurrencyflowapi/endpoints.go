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

package adaptiveconcurrencyflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "adaptiveconcurrencyflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// AdaptiveIn are the input arguments of Adaptive.
type AdaptiveIn struct { // MARKER: Adaptive
	Tag string `json:"tag,omitzero"`
}

// AdaptiveOut are the output arguments of Adaptive.
type AdaptiveOut struct { // MARKER: Adaptive
	Tallied bool `json:"tallied,omitzero"`
}

// AdaptiveConcurrencyIn are the input arguments of AdaptiveConcurrency.
type AdaptiveConcurrencyIn struct { // MARKER: AdaptiveConcurrency
	Tag string `json:"tag,omitzero"`
}

// AdaptiveConcurrencyOut are the output arguments of AdaptiveConcurrency.
type AdaptiveConcurrencyOut struct { // MARKER: AdaptiveConcurrency
	Tallied bool `json:"tallied,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Adaptive            = Def{Method: "POST", Route: ":428/adaptive"}             // MARKER: Adaptive
	AdaptiveConcurrency = Def{Method: "GET", Route: ":428/adaptive-concurrency"}  // MARKER: AdaptiveConcurrency
)

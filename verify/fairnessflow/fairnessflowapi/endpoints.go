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

package fairnessflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "fairnessflow.verify"

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
	Tally    = Def{Method: "POST", Route: ":428/tally"}   // MARKER: Tally
	Fairness = Def{Method: "GET", Route: ":428/fairness"} // MARKER: Fairness
)

// TallyIn are the input arguments of Tally.
type TallyIn struct { // MARKER: Tally
	Tag     string `json:"tag,omitzero"`
	DelayMs int    `json:"delayMs,omitzero"`
}

// TallyOut are the output arguments of Tally.
type TallyOut struct { // MARKER: Tally
	Tallied bool `json:"tallied,omitzero"`
}

// FairnessIn are the input arguments of Fairness.
type FairnessIn struct { // MARKER: Fairness
	Tag     string `json:"tag,omitzero"`
	DelayMs int    `json:"delayMs,omitzero"`
}

// FairnessOut are the output arguments of Fairness.
type FairnessOut struct { // MARKER: Fairness
	Tallied bool `json:"tallied,omitzero"`
}

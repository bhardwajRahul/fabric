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

package saturatedbandflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "saturatedbandflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// BoundedIn are the input arguments of Bounded.
type BoundedIn struct { // MARKER: Bounded
	Tag string `json:"tag,omitzero"`
}

// BoundedOut are the output arguments of Bounded.
type BoundedOut struct { // MARKER: Bounded
	Tallied bool `json:"tallied,omitzero"`
}

// OpenIn are the input arguments of Open.
type OpenIn struct { // MARKER: Open
	Tag string `json:"tag,omitzero"`
}

// OpenOut are the output arguments of Open.
type OpenOut struct { // MARKER: Open
	Tallied bool `json:"tallied,omitzero"`
}

// SaturatedBandIn are the input arguments of SaturatedBand.
type SaturatedBandIn struct { // MARKER: SaturatedBand
	Tag string `json:"tag,omitzero"`
}

// SaturatedBandOut are the output arguments of SaturatedBand.
type SaturatedBandOut struct { // MARKER: SaturatedBand
	Tallied bool `json:"tallied,omitzero"`
}

// OpenBandIn are the input arguments of OpenBand.
type OpenBandIn struct { // MARKER: OpenBand
	Tag string `json:"tag,omitzero"`
}

// OpenBandOut are the output arguments of OpenBand.
type OpenBandOut struct { // MARKER: OpenBand
	Tallied bool `json:"tallied,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Bounded       = Def{Method: "POST", Route: ":428/bounded"}        // MARKER: Bounded
	Open          = Def{Method: "POST", Route: ":428/open"}           // MARKER: Open
	SaturatedBand = Def{Method: "GET", Route: ":428/saturated-band"}  // MARKER: SaturatedBand
	OpenBand      = Def{Method: "GET", Route: ":428/open-band"}       // MARKER: OpenBand
)

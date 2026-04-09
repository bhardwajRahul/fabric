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

package configuratorapi

import (
	"time"

	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "configurator.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// ValuesIn are the input arguments of Values.
type ValuesIn struct { // MARKER: Values
	Names []string `json:"names,omitzero"`
}

// ValuesOut are the output arguments of Values.
type ValuesOut struct { // MARKER: Values
	Values map[string]string `json:"values,omitzero"`
}

// RefreshIn are the input arguments of Refresh.
type RefreshIn struct { // MARKER: Refresh
}

// RefreshOut are the output arguments of Refresh.
type RefreshOut struct { // MARKER: Refresh
}

// SyncRepoIn are the input arguments of SyncRepo.
type SyncRepoIn struct { // MARKER: SyncRepo
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// SyncRepoOut are the output arguments of SyncRepo.
type SyncRepoOut struct { // MARKER: SyncRepo
}

// Values443In are the input arguments of Values443.
type Values443In struct { // MARKER: Values443
	Names []string `json:"names,omitzero"`
}

// Values443Out are the output arguments of Values443.
type Values443Out struct { // MARKER: Values443
	Values map[string]string `json:"values,omitzero"`
}

// Refresh443In are the input arguments of Refresh443.
type Refresh443In struct { // MARKER: Refresh443
}

// Refresh443Out are the output arguments of Refresh443.
type Refresh443Out struct { // MARKER: Refresh443
}

// Sync443In are the input arguments of Sync443.
type Sync443In struct { // MARKER: Sync443
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// Sync443Out are the output arguments of Sync443.
type Sync443Out struct { // MARKER: Sync443
}

var (
	// HINT: Insert endpoint definitions here
	Values     = Def{Method: "ANY", Route: ":888/values"}    // MARKER: Values
	Refresh    = Def{Method: "ANY", Route: ":444/refresh"}   // MARKER: Refresh
	SyncRepo   = Def{Method: "ANY", Route: ":888/sync-repo"} // MARKER: SyncRepo
	Values443  = Def{Method: "ANY", Route: ":443/values"}    // MARKER: Values443
	Refresh443 = Def{Method: "ANY", Route: ":443/refresh"}   // MARKER: Refresh443
	Sync443    = Def{Method: "ANY", Route: ":443/sync"}      // MARKER: Sync443
)

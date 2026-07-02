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
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "configurator.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "Configurator"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 254

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Configurator is a core microservice that centralizes the dissemination of configuration values to other microservices.`

// Values returns the values associated with the specified config property names for the caller microservice.
var Values = define.Function{ // MARKER: Values
	Host: Hostname, Method: "ANY", Route: ":888/values",
	In: ValuesIn{}, Out: ValuesOut{},
}

// ValuesIn are the input arguments of Values.
type ValuesIn struct { // MARKER: Values
	Names []string `json:"names,omitzero"`
}

// ValuesOut are the output arguments of Values.
type ValuesOut struct { // MARKER: Values
	Values map[string]string `json:"values,omitzero"`
}

// Refresh tells all microservices to contact the configurator and refresh their configs.
// An error is returned if any of the values sent to the microservices fails validation.
var Refresh = define.Function{ // MARKER: Refresh
	Host: Hostname, Method: "ANY", Route: ":444/refresh",
	In: RefreshIn{}, Out: RefreshOut{},
}

// RefreshIn are the input arguments of Refresh.
type RefreshIn struct { // MARKER: Refresh
}

// RefreshOut are the output arguments of Refresh.
type RefreshOut struct { // MARKER: Refresh
}

// SyncRepo is used to synchronize values among replica peers of the configurator.
var SyncRepo = define.Function{ // MARKER: SyncRepo
	Host: Hostname, Method: "ANY", Route: ":888/sync-repo",
	LoadBalancing: define.None,
	In:            SyncRepoIn{}, Out: SyncRepoOut{},
}

// SyncRepoIn are the input arguments of SyncRepo.
type SyncRepoIn struct { // MARKER: SyncRepo
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// SyncRepoOut are the output arguments of SyncRepo.
type SyncRepoOut struct { // MARKER: SyncRepo
}

// Deprecated.
var Values443 = define.Function{ // MARKER: Values443
	Host: Hostname, Method: "ANY", Route: ":443/values",
	In: Values443In{}, Out: Values443Out{},
}

// Values443In are the input arguments of Values443.
type Values443In struct { // MARKER: Values443
	Names []string `json:"names,omitzero"`
}

// Values443Out are the output arguments of Values443.
type Values443Out struct { // MARKER: Values443
	Values map[string]string `json:"values,omitzero"`
}

// Deprecated.
var Refresh443 = define.Function{ // MARKER: Refresh443
	Host: Hostname, Method: "ANY", Route: ":443/refresh",
	In: Refresh443In{}, Out: Refresh443Out{},
}

// Refresh443In are the input arguments of Refresh443.
type Refresh443In struct { // MARKER: Refresh443
}

// Refresh443Out are the output arguments of Refresh443.
type Refresh443Out struct { // MARKER: Refresh443
}

// Deprecated.
var Sync443 = define.Function{ // MARKER: Sync443
	Host: Hostname, Method: "ANY", Route: ":443/sync",
	In: Sync443In{}, Out: Sync443Out{},
}

// Sync443In are the input arguments of Sync443.
type Sync443In struct { // MARKER: Sync443
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// Sync443Out are the output arguments of Sync443.
type Sync443Out struct { // MARKER: Sync443
}

// PeriodicRefresh tells all microservices to contact the configurator and refresh their configs. An error
// is returned if any of the values sent to the microservices fails validation.
var PeriodicRefresh = define.Ticker{ // MARKER: PeriodicRefresh
	Interval: 20 * time.Minute,
}

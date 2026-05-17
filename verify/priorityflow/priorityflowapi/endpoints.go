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

package priorityflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "priorityflow.verify"

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
	Record   = Def{Method: "POST", Route: ":428/record"}  // MARKER: Record
	Priority = Def{Method: "GET", Route: ":428/priority"} // MARKER: Priority
)

// RecordIn are the input arguments of Record.
type RecordIn struct { // MARKER: Record
	Tag     string `json:"tag,omitzero"`
	DelayMs int    `json:"delayMs,omitzero"`
}

// RecordOut are the output arguments of Record.
type RecordOut struct { // MARKER: Record
	Recorded bool `json:"recorded,omitzero"`
}

// PriorityIn are the input arguments of Priority.
type PriorityIn struct { // MARKER: Priority
	Tag     string `json:"tag,omitzero"`
	DelayMs int    `json:"delayMs,omitzero"`
}

// PriorityOut are the output arguments of Priority.
type PriorityOut struct { // MARKER: Priority
	Recorded bool `json:"recorded,omitzero"`
}

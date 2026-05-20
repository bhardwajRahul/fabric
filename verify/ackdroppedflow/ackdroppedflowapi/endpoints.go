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

package ackdroppedflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "ackdroppedflow.verify"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// ParkIn are the input arguments of Park.
type ParkIn struct { // MARKER: Park
	Tag string `json:"tag,omitzero"`
}

// ParkOut are the output arguments of Park.
type ParkOut struct { // MARKER: Park
	Parked bool `json:"parked,omitzero"`
}

// PingIn are the input arguments of Ping.
type PingIn struct { // MARKER: Ping
	Tag string `json:"tag,omitzero"`
}

// PingOut are the output arguments of Ping.
type PingOut struct { // MARKER: Ping
	Pinged bool `json:"pinged,omitzero"`
}

// AckDroppedIn are the input arguments of AckDropped.
type AckDroppedIn struct { // MARKER: AckDropped
	Tag string `json:"tag,omitzero"`
}

// AckDroppedOut are the output arguments of AckDropped.
type AckDroppedOut struct { // MARKER: AckDropped
	Parked bool `json:"parked,omitzero"`
}

// EchoIn are the input arguments of Echo.
type EchoIn struct { // MARKER: Echo
	Tag string `json:"tag,omitzero"`
}

// EchoOut are the output arguments of Echo.
type EchoOut struct { // MARKER: Echo
	Pinged bool `json:"pinged,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Park       = Def{Method: "POST", Route: ":428/park"}        // MARKER: Park
	Ping       = Def{Method: "POST", Route: ":428/ping"}        // MARKER: Ping
	AckDropped = Def{Method: "GET", Route: ":428/ack-dropped"}  // MARKER: AckDropped
	Echo       = Def{Method: "GET", Route: ":428/echo"}         // MARKER: Echo
)

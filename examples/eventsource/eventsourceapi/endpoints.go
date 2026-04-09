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

package eventsourceapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "eventsource.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// RegisterIn are the input arguments of Register.
type RegisterIn struct { // MARKER: Register
	Email string `json:"email,omitzero"`
}

// RegisterOut are the output arguments of Register.
type RegisterOut struct { // MARKER: Register
	Allowed bool `json:"allowed,omitzero"`
}

// OnAllowRegisterIn are the input arguments of OnAllowRegister.
type OnAllowRegisterIn struct { // MARKER: OnAllowRegister
	Email string `json:"email,omitzero"`
}

// OnAllowRegisterOut are the output arguments of OnAllowRegister.
type OnAllowRegisterOut struct { // MARKER: OnAllowRegister
	Allow bool `json:"allow,omitzero"`
}

// OnRegisteredIn are the input arguments of OnRegistered.
type OnRegisteredIn struct { // MARKER: OnRegistered
	Email string `json:"email,omitzero"`
}

// OnRegisteredOut are the output arguments of OnRegistered.
type OnRegisteredOut struct { // MARKER: OnRegistered
}

var (
	// HINT: Insert endpoint definitions here
	Register        = Def{Method: "ANY", Route: ":443/register"}           // MARKER: Register
	OnAllowRegister = Def{Method: "POST", Route: ":417/on-allow-register"} // MARKER: OnAllowRegister
	OnRegistered    = Def{Method: "POST", Route: ":417/on-registered"}     // MARKER: OnRegistered
)

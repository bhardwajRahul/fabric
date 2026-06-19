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
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "eventsource.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "EventSource"

// Version is the major version of the microservice's public API.
const Version = 270

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The event source microservice fires events that are caught by the event sink microservice.`

// OnAllowRegister is triggered before registration to check if any sink blocks it.
var OnAllowRegister = define.OutboundEvent{ // MARKER: OnAllowRegister
	Host: Hostname, Method: "POST", Route: ":417/on-allow-register",
	In: OnAllowRegisterIn{}, Out: OnAllowRegisterOut{},
}

// OnAllowRegisterIn are the input arguments of OnAllowRegister.
type OnAllowRegisterIn struct { // MARKER: OnAllowRegister
	Email string `json:"email,omitzero"`
}

// OnAllowRegisterOut are the output arguments of OnAllowRegister.
type OnAllowRegisterOut struct { // MARKER: OnAllowRegister
	Allow bool `json:"allow,omitzero"`
}

// OnRegistered is triggered after successful registration.
var OnRegistered = define.OutboundEvent{ // MARKER: OnRegistered
	Host: Hostname, Method: "POST", Route: ":417/on-registered",
	In: OnRegisteredIn{}, Out: OnRegisteredOut{},
}

// OnRegisteredIn are the input arguments of OnRegistered.
type OnRegisteredIn struct { // MARKER: OnRegistered
	Email string `json:"email,omitzero"`
}

// OnRegisteredOut are the output arguments of OnRegistered.
type OnRegisteredOut struct { // MARKER: OnRegistered
}

// Register attempts to register a new user.
var Register = define.Function{ // MARKER: Register
	Host: Hostname, Method: "ANY", Route: ":443/register",
	In: RegisterIn{}, Out: RegisterOut{},
}

// RegisterIn are the input arguments of Register.
type RegisterIn struct { // MARKER: Register
	Email string `json:"email,omitzero"`
}

// RegisterOut are the output arguments of Register.
type RegisterOut struct { // MARKER: Register
	Allowed bool `json:"allowed,omitzero"`
}

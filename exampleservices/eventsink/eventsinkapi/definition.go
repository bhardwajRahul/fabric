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

package eventsinkapi

import (
	"github.com/microbus-io/fabric/define"
	"github.com/microbus-io/fabric/exampleservices/eventsource/eventsourceapi"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "eventsink.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "EventSink"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 261

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The event sink microservice handles events that are fired by the event source microservice.`

// Registered returns the list of registered users.
var Registered = define.Function{ // MARKER: Registered
	Host: Hostname, Method: "ANY", Route: ":443/registered",
	In: RegisteredIn{}, Out: RegisteredOut{},
}

// RegisteredIn are the input arguments of Registered.
type RegisteredIn struct { // MARKER: Registered
}

// RegisteredOut are the output arguments of Registered.
type RegisteredOut struct { // MARKER: Registered
	Emails []string `json:"emails,omitzero"`
}

// OnAllowRegister blocks registrations from gmail and hotmail domains.
var OnAllowRegister = define.InboundEvent{ // MARKER: OnAllowRegister
	Source: eventsourceapi.OnAllowRegister,
}

// OnRegistered keeps track of registrations.
var OnRegistered = define.InboundEvent{ // MARKER: OnRegistered
	Source: eventsourceapi.OnRegistered,
}

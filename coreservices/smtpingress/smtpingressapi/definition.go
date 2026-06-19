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

package smtpingressapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "smtp.ingress.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "SMTPIngress"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 182

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The SMTP ingress microservice listens for incoming emails and fires corresponding events.`

// Port is the TCP port to listen to.
var Port = define.Config{ // MARKER: Port
	Value:      int(0),
	Default:    "25",
	Validation: "int [1,65535]",
	Callback:   true,
}

// Enabled determines whether the email server is started.
var Enabled = define.Config{ // MARKER: Enabled
	Value:      bool(false),
	Default:    "true",
	Validation: "bool",
	Callback:   true,
}

// MaxSize is the maximum size of messages that will be accepted, in megabytes. Defaults to 10 megabytes.
var MaxSize = define.Config{ // MARKER: MaxSize
	Value:      int(0),
	Default:    "10",
	Validation: "int [0,1024]",
	Callback:   true,
}

// MaxClients controls how many client connections can be opened in parallel. Defaults to 128.
var MaxClients = define.Config{ // MARKER: MaxClients
	Value:      int(0),
	Default:    "128",
	Validation: "int [1,1024]",
	Callback:   true,
}

// Workers controls how many workers process incoming mail. Defaults to 8.
var Workers = define.Config{ // MARKER: Workers
	Value:      int(0),
	Default:    "8",
	Validation: "int [1,1024]",
	Callback:   true,
}

// OnIncomingEmail is triggered when a new email message is received.
var OnIncomingEmail = define.OutboundEvent{ // MARKER: OnIncomingEmail
	Host: Hostname, Method: "POST", Route: ":417/on-incoming-email",
	In: OnIncomingEmailIn{}, Out: OnIncomingEmailOut{},
}

// OnIncomingEmailIn are the input arguments of OnIncomingEmail.
type OnIncomingEmailIn struct { // MARKER: OnIncomingEmail
	Email *Email `json:"email,omitzero"`
}

// OnIncomingEmailOut are the output arguments of OnIncomingEmail.
type OnIncomingEmailOut struct { // MARKER: OnIncomingEmail
}

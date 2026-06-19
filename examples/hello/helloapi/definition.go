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

package helloapi

import (
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "hello.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Hello"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 326

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Hello microservice demonstrates the various capabilities of a microservice.`

// Greeting to use.
var Greeting = define.Config{ // MARKER: Greeting
	Value:   string(""),
	Default: "Hello",
}

// Repeat indicates how many times to display the greeting.
var Repeat = define.Config{ // MARKER: Repeat
	Value:      int(0),
	Default:    "1",
	Validation: "int [0,100]",
}

// Hello prints a greeting.
var Hello = define.Web{ // MARKER: Hello
	Host: Hostname, Method: "ANY", Route: "/hello",
}

// Echo back the incoming request in wire format.
var Echo = define.Web{ // MARKER: Echo
	Host: Hostname, Method: "ANY", Route: "/echo",
}

// Ping all microservices and list them.
var Ping = define.Web{ // MARKER: Ping
	Host: Hostname, Method: "ANY", Route: "/ping",
}

// Calculator renders a UI for a calculator. The calculation operation is delegated to another microservice in order to demonstrate a call from one microservice to another.
var Calculator = define.Web{ // MARKER: Calculator
	Host: Hostname, Method: "ANY", Route: "/calculator",
}

// BusPNG serves an image from the embedded resources.
var BusPNG = define.Web{ // MARKER: BusPNG
	Host: Hostname, Method: "GET", Route: "/bus.png",
}

// Localization prints hello in the language best matching the request's Accept-Language header.
var Localization = define.Web{ // MARKER: Localization
	Host: Hostname, Method: "ANY", Route: "/localization",
}

// Root is the top-most root page.
var Root = define.Web{ // MARKER: Root
	Host: Hostname, Method: "ANY", Route: "//root",
}

// TickTock is executed every 10 seconds.
var TickTock = define.Ticker{ // MARKER: TickTock
	Interval: 10 * time.Second,
}

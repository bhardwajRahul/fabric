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

// Hostname is the default hostname of the microservice.
const Hostname = "hello.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Hello"

// Version is the major version of the microservice's public API.
const Version = 326

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Hello microservice demonstrates the various capabilities of a microservice.`

// Greeting to use.
var Greeting = define.Config{
	Value:   string(""),
	Default: "Hello",
}

// Repeat indicates how many times to display the greeting.
var Repeat = define.Config{
	Value:      int(0),
	Default:    "1",
	Validation: "int [0,100]",
}

// Hello prints a greeting.
var Hello = define.Web{
	Host: Hostname, Method: "ANY", Route: "/hello",
}

// Echo back the incoming request in wire format.
var Echo = define.Web{
	Host: Hostname, Method: "ANY", Route: "/echo",
}

// Ping all microservices and list them.
var Ping = define.Web{
	Host: Hostname, Method: "ANY", Route: "/ping",
}

// Calculator renders a UI for a calculator. The calculation operation is delegated to another microservice in order to demonstrate a call from one microservice to another.
var Calculator = define.Web{
	Host: Hostname, Method: "ANY", Route: "/calculator",
}

// BusPNG serves an image from the embedded resources.
var BusPNG = define.Web{
	Host: Hostname, Method: "GET", Route: "/bus.png",
}

// Localization prints hello in the language best matching the request's Accept-Language header.
var Localization = define.Web{
	Host: Hostname, Method: "ANY", Route: "/localization",
}

// Root is the top-most root page.
var Root = define.Web{
	Host: Hostname, Method: "ANY", Route: "//root",
}

// TickTock is executed every 10 seconds.
var TickTock = define.Ticker{
	Interval: 10 * time.Second,
}

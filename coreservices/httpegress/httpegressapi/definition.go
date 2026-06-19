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

package httpegressapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "http.egress.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "HTTPEgress"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 158

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The HTTP egress microservice relays HTTP requests to the internet.`

// MakeRequest proxies a request to a URL and returns the HTTP response, respecting the timeout set in the context.
// The proxied request is expected to be posted in the body of the request in binary form (RFC7231).
var MakeRequest = define.Web{ // MARKER: MakeRequest
	Host: Hostname, Method: "POST", Route: ":444/make-request",
}

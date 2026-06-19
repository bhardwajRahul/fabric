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

package openapiportalapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "openapi.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "OpenAPIPortal"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 166

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The OpenAPI portal serves an aggregated OpenAPI 3.1 document covering every microservice on the bus,
plus per-service docs proxied from each microservice's :888/openapi.json. Endpoints are filtered by the caller's claims
(per-service) and the request's port (consumer-side).`

// Document returns the OpenAPI 3.1 document as JSON. Without a hostname query arg, returns an aggregate covering every microservice on the bus. With ?hostname=<host>, proxies to that single service's :888/openapi.json. Filtered by the caller's claims; the aggregate is also filtered by the request's port.
var Document = define.Web{ // MARKER: Document
	Host: Hostname, Method: "GET", Route: "//openapi.json:0",
}

// Explorer renders a human-friendly HTML browser for the OpenAPI documents on the bus. Without ?hostname=<host>, lists every service. With ?hostname=<host>, shows that service's endpoints.
var Explorer = define.Web{ // MARKER: Explorer
	Host: Hostname, Method: "GET", Route: "//openapi:0",
}

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

package controlapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "control.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "Control"

// Version is the major version of the microservice's public API.
const Version = 237

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `This microservice is created for the sake of generating the client API for the :888 control subscriptions.
The microservice itself does nothing and should not be included in applications.`

// OnNewSubs informs of new subscriptions.
var OnNewSubs = define.OutboundEvent{ // MARKER: OnNewSubs
	Host: Hostname, Method: "POST", Route: ":888/on-new-subs",
	In: OnNewSubsIn{}, Out: OnNewSubsOut{},
}

// OnNewSubsIn are the input arguments of OnNewSubs.
type OnNewSubsIn struct { // MARKER: OnNewSubs
}

// OnNewSubsOut are the output arguments of OnNewSubs.
type OnNewSubsOut struct { // MARKER: OnNewSubs
}

// Ping responds to the message with a pong.
var Ping = define.Function{ // MARKER: Ping
	Host: Hostname, Method: "ANY", Route: ":888/ping",
	LoadBalancing: define.None,
	In:            PingIn{}, Out: PingOut{},
}

// PingIn are the input arguments of Ping.
type PingIn struct { // MARKER: Ping
}

// PingOut are the output arguments of Ping.
type PingOut struct { // MARKER: Ping
	Pong int `json:"pong,omitzero"`
}

// ConfigRefresh pulls the latest config values from the configurator microservice.
var ConfigRefresh = define.Function{ // MARKER: ConfigRefresh
	Host: Hostname, Method: "ANY", Route: ":888/config-refresh",
	LoadBalancing: define.None,
	In:            ConfigRefreshIn{}, Out: ConfigRefreshOut{},
}

// ConfigRefreshIn are the input arguments of ConfigRefresh.
type ConfigRefreshIn struct { // MARKER: ConfigRefresh
}

// ConfigRefreshOut are the output arguments of ConfigRefresh.
type ConfigRefreshOut struct { // MARKER: ConfigRefresh
}

// Trace forces exporting the indicated tracing span.
var Trace = define.Function{ // MARKER: Trace
	Host: Hostname, Method: "ANY", Route: ":888/trace",
	LoadBalancing: define.None,
	In:            TraceIn{}, Out: TraceOut{},
}

// TraceIn are the input arguments of Trace.
type TraceIn struct { // MARKER: Trace
	ID string `json:"id,omitzero"`
}

// TraceOut are the output arguments of Trace.
type TraceOut struct { // MARKER: Trace
}

// OpenAPI returns the OpenAPI 3.1 document of the microservice. Returns endpoints across all ports filtered by the caller's claims; consumers (portal/MCP) apply any port-based filtering at their ingress boundary.
var OpenAPI = define.Function{ // MARKER: OpenAPI
	Host: Hostname, Method: "GET", Route: ":888/openapi.json",
	In: OpenAPIIn{}, Out: OpenAPIOut{},
}

// OpenAPIIn are the input arguments of OpenAPI.
type OpenAPIIn struct { // MARKER: OpenAPI
}

// OpenAPIOut are the output arguments of OpenAPI.
type OpenAPIOut struct { // MARKER: OpenAPI
	HTTPResponseBody *Document `json:"-"`
	HTTPStatusCode   int       `json:"-"`
}

// Metrics returns the Prometheus metrics collected by the microservice.
var Metrics = define.Web{ // MARKER: Metrics
	Host: Hostname, Method: "ANY", Route: ":888/metrics",
	LoadBalancing: define.None,
}

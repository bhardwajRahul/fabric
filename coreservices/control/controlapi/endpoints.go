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
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "control.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// PingIn are the input arguments of Ping.
type PingIn struct { // MARKER: Ping
}

// PingOut are the output arguments of Ping.
type PingOut struct { // MARKER: Ping
	Pong int `json:"pong,omitzero"`
}

// ConfigRefreshIn are the input arguments of ConfigRefresh.
type ConfigRefreshIn struct { // MARKER: ConfigRefresh
}

// ConfigRefreshOut are the output arguments of ConfigRefresh.
type ConfigRefreshOut struct { // MARKER: ConfigRefresh
}

// TraceIn are the input arguments of Trace.
type TraceIn struct { // MARKER: Trace
	ID string `json:"id,omitzero"`
}

// TraceOut are the output arguments of Trace.
type TraceOut struct { // MARKER: Trace
}

// OnNewSubsIn are the input arguments of OnNewSubs.
type OnNewSubsIn struct { // MARKER: OnNewSubs
}

// OnNewSubsOut are the output arguments of OnNewSubs.
type OnNewSubsOut struct { // MARKER: OnNewSubs
}

// OpenAPIIn are the input arguments of OpenAPI.
type OpenAPIIn struct { // MARKER: OpenAPI
}

// OpenAPIOut are the output arguments of OpenAPI.
type OpenAPIOut struct { // MARKER: OpenAPI
	HTTPResponseBody *Document `json:"-"`
	HTTPStatusCode   int       `json:"-"`
}

var (
	// HINT: Insert endpoint definitions here
	Ping          = Def{Method: "ANY", Route: ":888/ping"}           // MARKER: Ping
	ConfigRefresh = Def{Method: "ANY", Route: ":888/config-refresh"} // MARKER: ConfigRefresh
	Trace         = Def{Method: "ANY", Route: ":888/trace"}          // MARKER: Trace
	Metrics       = Def{Method: "ANY", Route: ":888/metrics"}        // MARKER: Metrics
	OnNewSubs     = Def{Method: "POST", Route: ":888/on-new-subs"}   // MARKER: OnNewSubs
	OpenAPI       = Def{Method: "GET", Route: ":888/openapi.json"}   // MARKER: OpenAPI
)

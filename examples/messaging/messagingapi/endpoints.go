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

package messagingapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "messaging.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	Home         = Def{Method: "GET", Route: "/home"}          // MARKER: Home
	NoQueue      = Def{Method: "GET", Route: "/no-queue"}      // MARKER: NoQueue
	DefaultQueue = Def{Method: "GET", Route: "/default-queue"} // MARKER: DefaultQueue
	CacheLoad    = Def{Method: "GET", Route: "/cache-load"}    // MARKER: CacheLoad
	CacheStore   = Def{Method: "GET", Route: "/cache-store"}   // MARKER: CacheStore
)

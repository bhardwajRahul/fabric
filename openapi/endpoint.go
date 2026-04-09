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

package openapi

import (
	"net/url"
	"strings"

	"github.com/microbus-io/fabric/httpx"
)

// Endpoint describes a single endpoint of a microservice, such as an RPC function.
type Endpoint struct {
	Hostname       string
	Type           string
	Name           string
	Method         string
	Route          string
	Summary        string
	Description    string
	InputArgs      any
	OutputArgs     any
	RequiredClaims string
}

// URL is the full URL to the endpoint, derived from the endpoint's [Hostname] and [Route].
func (e *Endpoint) URL() string {
	return httpx.JoinHostAndPath(e.Hostname, e.Route)
}

// Port returns the port that the endpoint listens on, as a string. The default is "443".
//
// The port is parsed from the [Route] in any of these forms:
//   - ":NNN" or ":NNN/path" - the explicit port prefix
//   - "//host:NNN/path" - a protocol-relative absolute route
//   - "https://host:NNN/path" or "http://host:NNN/path" - a full URL
//   - any other form (including "/path", "//host/path", or empty) - defaults to "443"
//     ("80" for an "http://" URL with no port)
func (e *Endpoint) Port() string {
	route := e.Route
	if strings.HasPrefix(route, ":") {
		rest := route[1:]
		if i := strings.Index(rest, "/"); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	if strings.HasPrefix(route, "//") || strings.Contains(route, "://") {
		u, err := url.Parse(route)
		if err == nil {
			if p := u.Port(); p != "" {
				return p
			}
			if u.Scheme == "http" {
				return "80"
			}
		}
	}
	return "443"
}

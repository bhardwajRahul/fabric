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

package bearertokenapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "bearer.token.core"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// MintIn are the input arguments of Mint.
type MintIn struct { // MARKER: Mint
	Claims any `json:"claims,omitzero"`
}

// MintOut are the output arguments of Mint.
type MintOut struct { // MARKER: Mint
	Token string `json:"token,omitzero"`
}

// JWKSIn are the input arguments of JWKS.
type JWKSIn struct { // MARKER: JWKS
}

// JWKSOut are the output arguments of JWKS.
type JWKSOut struct { // MARKER: JWKS
	Keys []JWK `json:"keys,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	Mint = Def{Method: "ANY", Route: ":444/mint"} // MARKER: Mint
	JWKS = Def{Method: "ANY", Route: ":888/jwks"} // MARKER: JWKS
)

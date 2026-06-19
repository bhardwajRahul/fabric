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
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "bearer.token.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "BearerToken"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 2

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `BearerToken signs long-lived JWTs with Ed25519 keys for external actor authentication.`

// AuthTokenTTL sets the TTL of the JWT.
var AuthTokenTTL = define.Config{ // MARKER: AuthTokenTTL
	Value:      time.Duration(0),
	Default:    "720h",
	Validation: "dur [1m,]",
}

// PrivateKey is the Ed25519 private key used to sign JWTs, in PEM or raw base64 format.
var PrivateKey = define.Config{ // MARKER: PrivateKey
	Value:    string(""),
	Secret:   true,
	Callback: true,
}

// AltPrivateKey is an alternative Ed25519 private key used during key rotation, in PEM or raw base64 format.
var AltPrivateKey = define.Config{ // MARKER: AltPrivateKey
	Value:    string(""),
	Secret:   true,
	Callback: true,
}

// Mint signs a JWT with the given claims.
var Mint = define.Function{ // MARKER: Mint
	Host: Hostname, Method: "ANY", Route: ":666/mint",
	In: MintIn{}, Out: MintOut{},
}

// MintIn are the input arguments of Mint.
type MintIn struct { // MARKER: Mint
	Claims any `json:"claims,omitzero"`
}

// MintOut are the output arguments of Mint.
type MintOut struct { // MARKER: Mint
	Token string `json:"token,omitzero"`
}

// JWKS returns the public keys of the token issuer in JWKS format.
var JWKS = define.Function{ // MARKER: JWKS
	Host: Hostname, Method: "ANY", Route: ":888/jwks",
	In: JWKSIn{}, Out: JWKSOut{},
}

// JWKSIn are the input arguments of JWKS.
type JWKSIn struct { // MARKER: JWKS
}

// JWKSOut are the output arguments of JWKS.
type JWKSOut struct { // MARKER: JWKS
	Keys []JWK `json:"keys,omitzero"`
}

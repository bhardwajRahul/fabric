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

package accesstokenapi

import (
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "access.token.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "AccessToken"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 3

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `AccessToken generates short-lived JWTs signed with ephemeral Ed25519 keys for internal actor propagation.`

// KeyRotationInterval is the duration between Ed25519 key rotations.
var KeyRotationInterval = define.Config{ // MARKER: KeyRotationInterval
	Value:      time.Duration(0),
	Default:    "6h",
	Validation: "dur [2h,]",
}

// DefaultTokenLifetime is the token lifetime used when no time budget is present in the request.
var DefaultTokenLifetime = define.Config{ // MARKER: DefaultTokenLifetime
	Value:      time.Duration(0),
	Default:    "20s",
	Validation: "dur [1s,15m]",
}

// MaxTokenLifetime is the maximum token lifetime regardless of the request's time budget.
var MaxTokenLifetime = define.Config{ // MARKER: MaxTokenLifetime
	Value:      time.Duration(0),
	Default:    "15m",
	Validation: "dur [1s,15m]",
}

// Mint signs a JWT with the given claims. The token's lifetime is derived from the request's time budget,
// falling back to DefaultTokenLifetime if no budget is set, and capped at MaxTokenLifetime.
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

// JWKS aggregates public keys from all replicas and returns them in JWKS format.
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

// LocalKeys returns this replica's current and previous public keys in JWKS format.
var LocalKeys = define.Function{ // MARKER: LocalKeys
	Host: Hostname, Method: "ANY", Route: ":888/local-keys",
	LoadBalancing: define.None,
	In:            LocalKeysIn{}, Out: LocalKeysOut{},
}

// LocalKeysIn are the input arguments of LocalKeys.
type LocalKeysIn struct { // MARKER: LocalKeys
}

// LocalKeysOut are the output arguments of LocalKeys.
type LocalKeysOut struct { // MARKER: LocalKeys
	Keys []JWK `json:"keys,omitzero"`
}

// RotateKey checks if the current key has exceeded the rotation interval and generates a new key pair if so.
var RotateKey = define.Ticker{ // MARKER: RotateKey
	Interval: 10 * time.Minute,
}

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

// JWK represents a JSON Web Key.
type JWK struct {
	KTY string `json:"kty"`          // "OKP"
	CRV string `json:"crv"`          // "Ed25519"
	X   string `json:"x"`            // base64url public key
	KID string `json:"kid"`          // key ID
	Use string `json:"use,omitzero"` // "sig"
	ALG string `json:"alg,omitzero"` // "EdDSA"
}

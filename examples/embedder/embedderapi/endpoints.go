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

package embedderapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "embedder.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// EmbedIn are the input arguments of Embed.
type EmbedIn struct { // MARKER: Embed
	Text string `json:"text,omitzero" jsonschema:"description=Text is the input string to embed"`
}

// EmbedOut are the output arguments of Embed.
type EmbedOut struct { // MARKER: Embed
	Vector []float64 `json:"vector,omitzero" jsonschema:"description=Vector is the embedding produced by the model"`
}

// SimilarityIn are the input arguments of Similarity.
type SimilarityIn struct { // MARKER: Similarity
	A string `json:"a,omitzero" jsonschema:"description=A is the first input string"`
	B string `json:"b,omitzero" jsonschema:"description=B is the second input string"`
}

// SimilarityOut are the output arguments of Similarity.
type SimilarityOut struct { // MARKER: Similarity
	Score float64 `json:"score,omitzero" jsonschema:"description=Score is the cosine similarity in [-1.0, 1.0]"`
}

var (
	// HINT: Insert endpoint definitions here
	Embed      = Def{Method: "GET", Route: ":443/embed"}        // MARKER: Embed
	Similarity = Def{Method: "GET", Route: ":443/similarity"}   // MARKER: Similarity
	Demo       = Def{Method: "ANY", Route: ":443/demo"}         // MARKER: Demo
	DemoInit   = Def{Method: "POST", Route: ":443/demo/init"}   // MARKER: DemoInit
	DemoStatus = Def{Method: "GET", Route: ":443/demo/status"}  // MARKER: DemoStatus
)

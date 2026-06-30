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
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "embedder.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Embedder"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 2

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Embedder is a sentence-embedding microservice backed by sentence-transformers running in an in-process Python virtual environment via github.com/microbus-io/pyvenv.`

// MaxWorkers caps how many calls into the Python venv may run concurrently.
var MaxWorkers = define.Config{ // MARKER: MaxWorkers
	Value:      int(0),
	Default:    "2",
	Validation: "int [1,]",
}

// Embed returns the sentence-embedding vector for the input text.
var Embed = define.Function{ // MARKER: Embed
	Host: Hostname, Method: "GET", Route: ":443/embed",
	Manual: true, Tags: []string{"python"},
	In: EmbedIn{}, Out: EmbedOut{},
}

// EmbedIn are the input arguments of Embed.
type EmbedIn struct { // MARKER: Embed
	Text string `json:"text,omitzero" jsonschema_description:"Text is the input string to embed"`
}

// EmbedOut are the output arguments of Embed.
type EmbedOut struct { // MARKER: Embed
	Vector []float64 `json:"vector,omitzero" jsonschema_description:"Vector is the embedding produced by the model"`
}

// Similarity returns the cosine similarity between the embeddings of strings a and b.
var Similarity = define.Function{ // MARKER: Similarity
	Host: Hostname, Method: "GET", Route: ":443/similarity",
	Manual: true, Tags: []string{"python"},
	In: SimilarityIn{}, Out: SimilarityOut{},
}

// SimilarityIn are the input arguments of Similarity.
type SimilarityIn struct { // MARKER: Similarity
	A string `json:"a,omitzero" jsonschema_description:"A is the first input string"`
	B string `json:"b,omitzero" jsonschema_description:"B is the second input string"`
}

// SimilarityOut are the output arguments of Similarity.
type SimilarityOut struct { // MARKER: Similarity
	Score float64 `json:"score,omitzero" jsonschema_description:"Score is the cosine similarity in [-1.0, 1.0]"`
}

// Demo serves the interactive demo page for the embedder.
var Demo = define.Web{ // MARKER: Demo
	Host: Hostname, Method: "ANY", Route: ":443/demo",
}

// DemoInit kicks off Python venv allocation in the background.
var DemoInit = define.Web{ // MARKER: DemoInit
	Host: Hostname, Method: "POST", Route: ":443/demo/init",
}

// DemoStatus returns the current venv initialization status and tailed logs.
var DemoStatus = define.Web{ // MARKER: DemoStatus
	Host: Hostname, Method: "GET", Route: ":443/demo/status",
}

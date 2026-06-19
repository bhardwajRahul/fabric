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

package geminillmapi

import (
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "gemini.llm.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "GeminiLLM"

// Version is the major version of the microservice's public API.
const Version = 6

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Gemini LLM provider microservice implements the Turn endpoint for the Google Gemini API.`

// CompletionURL is the base URL of the Gemini models endpoint; the model and generateContent action are appended per request.
var CompletionURL = define.Config{ // MARKER: CompletionURL
	Value:      string(""),
	Default:    "https://generativelanguage.googleapis.com/v1beta/models",
	Validation: "url",
}

// APIKey is the API key for the Gemini API.
var APIKey = define.Config{ // MARKER: APIKey
	Value:  string(""),
	Secret: true,
}

// Turn executes a single LLM turn using the Gemini provider.
var Turn = define.Function{ // MARKER: Turn
	Host: Hostname, Method: "POST", Route: ":444/turn",
	In: TurnIn{}, Out: TurnOut{},
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Model    string              `json:"model,omitzero"`
	Messages []llmapi.Message    `json:"messages,omitzero"`
	Tools    []llmapi.Tool       `json:"tools,omitzero"`
	Options  *llmapi.TurnOptions `json:"options,omitzero"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	Content    string            `json:"content,omitzero"`
	ToolCalls  []llmapi.ToolCall `json:"toolCalls,omitzero"`
	StopReason string            `json:"stopReason,omitzero"`
	Usage      llmapi.Usage      `json:"usage,omitzero"`
}

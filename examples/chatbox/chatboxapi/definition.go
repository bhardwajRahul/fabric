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

package chatboxapi

import (
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "chatbox.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Chatbox"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 4

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Chatbox is a demo LLM provider that pattern-matches user messages to demonstrate the tool-calling flow.`

// Turn executes a single LLM turn using the chatbox demo provider.
// It pattern-matches math questions and generates tool calls to the calculator.
var Turn = define.Function{ // MARKER: Turn
	Host: Hostname, Method: "POST", Route: ":444/turn",
	In: TurnIn{}, Out: TurnOut{},
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Model    string              `json:"model,omitzero" jsonschema:"description=Model is the model identifier"`
	Messages []llmapi.Message    `json:"messages,omitzero" jsonschema:"description=Messages is the conversation history"`
	Tools    []llmapi.Tool       `json:"tools,omitzero" jsonschema:"description=Tools is the list of tools available to the LLM"`
	Options  *llmapi.TurnOptions `json:"options,omitzero" jsonschema:"description=Options configures the turn"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	Content    string            `json:"content,omitzero" jsonschema:"description=Content is the LLM text response"`
	ToolCalls  []llmapi.ToolCall `json:"toolCalls,omitzero" jsonschema:"description=ToolCalls is the list of tool calls"`
	StopReason string            `json:"stopReason,omitzero" jsonschema:"description=StopReason is the normalized reason the turn ended"`
	Usage      llmapi.Usage      `json:"usage,omitzero" jsonschema:"description=Usage is the token consumption"`
}

// Demo serves the interactive demo page for the chatbox.
var Demo = define.Web{ // MARKER: Demo
	Host: Hostname, Method: "ANY", Route: "//chatbox.example/demo",
}

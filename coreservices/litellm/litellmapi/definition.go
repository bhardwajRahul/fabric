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

package litellmapi

import (
	"time"

	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "lite.llm.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "LiteLLM"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 3

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The LiteLLM provider microservice implements the Turn endpoint for a LiteLLM proxy using the OpenAI Responses wire format.`

// ResponsesURL is the URL of the LiteLLM proxy responses endpoint.
var ResponsesURL = define.Config{ // MARKER: ResponsesURL
	Value:      string(""),
	Default:    "http://localhost:4000/v1/responses",
	Validation: "url",
}

// ModelsURL is the URL of the LiteLLM proxy models-list endpoint, whose model_name entries are this provider's aliases.
var ModelsURL = define.Config{ // MARKER: ModelsURL
	Value:      string(""),
	Default:    "http://localhost:4000/v1/models",
	Validation: "url",
}

// APIKey is the virtual key for the LiteLLM proxy.
var APIKey = define.Config{ // MARKER: APIKey
	Value:  string(""),
	Secret: true,
}

// OnResolveProvider is fired by llm.core to resolve which provider serves a given model alias or name. The LiteLLM proxy fronts an operator-defined model_list, so this provider answers ok=true for any model_name the proxy exposes (fetched from its models-list API), including tiers like smart when the operator names an entry so.
var OnResolveProvider = define.InboundEvent{ // MARKER: OnResolveProvider
	Source: llmapi.OnResolveProvider,
}

// RefreshModels periodically repopulates the model_name set from the LiteLLM proxy's models-list API.
var RefreshModels = define.Ticker{ // MARKER: RefreshModels
	Interval: 6 * time.Hour,
}

// Turn executes a single LLM turn through the LiteLLM proxy.
var Turn = define.Function{ // MARKER: Turn
	Host: Hostname, Method: "POST", Route: ":444/turn",
	In: TurnIn{}, Out: TurnOut{},
}

// TurnIn are the input arguments of Turn.
type TurnIn struct { // MARKER: Turn
	Model   string              `json:"model,omitzero"`
	Items   []llmapi.Item       `json:"items,omitzero"`
	Tools   []llmapi.Tool       `json:"tools,omitzero"`
	Options *llmapi.TurnOptions `json:"options,omitzero"`
}

// TurnOut are the output arguments of Turn.
type TurnOut struct { // MARKER: Turn
	ItemsOut   []llmapi.Item `json:"items,omitzero"`
	StopReason string        `json:"stopReason,omitzero"`
	Usage      llmapi.Usage  `json:"usage,omitzero"`
}

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

package llmapi

import "encoding/json"

// ToolDef is a tool definition derived from an OpenAPI endpoint schema.
type ToolDef struct {
	Name        string          `json:"name,omitzero" jsonschema:"description=Name is the tool name derived from the OpenAPI operation"`
	Description string          `json:"description,omitzero" jsonschema:"description=Description is the tool description from the OpenAPI spec"`
	InputSchema json.RawMessage `json:"inputSchema,omitzero" jsonschema:"description=InputSchema is the JSON Schema for the tool's input parameters"`
	URL         string          `json:"url,omitzero" jsonschema:"description=URL is the Microbus endpoint URL to invoke"`
	Method      string          `json:"method,omitzero" jsonschema:"description=Method is the HTTP method from the OpenAPI spec"`
	FeatureType string          `json:"featureType,omitzero" jsonschema:"description=FeatureType is the x-feature-type from the OpenAPI spec"`
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	ID        string          `json:"id,omitzero" jsonschema:"description=ID is a unique identifier for this tool call"`
	Name      string          `json:"name,omitzero" jsonschema:"description=Name is the tool name the LLM wants to invoke"`
	Arguments json.RawMessage `json:"arguments,omitzero" jsonschema:"description=Arguments is the JSON-encoded arguments for the tool call"`
}

// TurnCompletion is the response from a single LLM turn.
type TurnCompletion struct {
	Content   string     `json:"content,omitzero" jsonschema:"description=Content is the text content of the response"`
	ToolCalls []ToolCall `json:"toolCalls,omitzero" jsonschema:"description=ToolCalls is the list of tool calls requested by the LLM"`
}

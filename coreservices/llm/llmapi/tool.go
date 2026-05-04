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

// Tool is a Microbus endpoint resolved into the shape an LLM consumes as a callable tool. It is
// produced internally by the LLM service from the OpenAPI documents of each requested host;
// callers of [Client.Chat] supply endpoint URLs, not Tool values.
type Tool struct {
	Name        string          `json:"name,omitzero" jsonschema:"description=Name is the tool name presented to the LLM"`
	Description string          `json:"description,omitzero" jsonschema:"description=Description is the natural-language description shown to the LLM"`
	InputSchema json.RawMessage `json:"inputSchema,omitzero" jsonschema:"description=InputSchema is the JSON Schema for the tool's input parameters"`
	URL         string          `json:"url,omitzero" jsonschema:"description=URL is the Microbus endpoint URL invoked when the LLM calls this tool"`
	Method      string          `json:"method,omitzero" jsonschema:"description=Method is the HTTP method used to invoke the endpoint"`
	Type        string          `json:"type,omitzero" jsonschema:"description=Type is the endpoint kind (function/web/workflow). Workflow tools are dispatched as dynamic subgraphs"`
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	ID        string          `json:"id,omitzero" jsonschema:"description=ID is a unique identifier for this tool call"`
	Name      string          `json:"name,omitzero" jsonschema:"description=Name is the tool name the LLM wants to invoke"`
	Arguments json.RawMessage `json:"arguments,omitzero" jsonschema:"description=Arguments is the JSON-encoded arguments for the tool call"`
}

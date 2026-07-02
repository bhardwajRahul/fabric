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
	Name        string          `json:"name,omitzero" jsonschema_description:"Name is the tool name presented to the LLM"`
	Description string          `json:"description,omitzero" jsonschema_description:"Description is the natural-language description shown to the LLM"`
	InputSchema json.RawMessage `json:"inputSchema,omitzero" jsonschema_description:"InputSchema is the JSON Schema for the tool's input parameters"`
	URL         string          `json:"url,omitzero" jsonschema_description:"URL is the Microbus endpoint URL invoked when the LLM calls this tool"`
	Method      string          `json:"method,omitzero" jsonschema_description:"Method is the HTTP method used to invoke the endpoint"`
	Type        string          `json:"type,omitzero" jsonschema_description:"Type is the endpoint kind (function/web/workflow). Workflow tools are dispatched as dynamic subgraphs"`
}

// ToolCall represents an LLM's request to invoke a tool. It is the tool_call variant of an Item; its
// result is a separate ToolResult item correlated by ID. Any reasoning bound to this call travels as a
// reasoning Item positioned immediately before it.
type ToolCall struct {
	ID        string          `json:"id,omitzero" jsonschema_description:"ID is a unique identifier for this tool call"`
	Name      string          `json:"name,omitzero" jsonschema_description:"Name is the tool name the LLM wants to invoke"`
	Arguments json.RawMessage `json:"arguments,omitzero" jsonschema_description:"Arguments is the JSON-encoded arguments for the tool call"`
}

// ToolResult is the outcome of a tool invocation, correlated to its ToolCall by CallID. It is roleless:
// providers that nest tool results inside a message (Anthropic, Gemini) supply the enclosing role.
type ToolResult struct {
	CallID string `json:"callId,omitzero" jsonschema_description:"CallID correlates this result with the ToolCall.ID that requested it"`
	Output string `json:"output,omitzero" jsonschema_description:"Output is the serialized result returned to the model"`
}

// NewToolResult builds a tool result, for use with AppendItems or an Item literal.
func NewToolResult(callID, output string) *ToolResult {
	return &ToolResult{CallID: callID, Output: output}
}

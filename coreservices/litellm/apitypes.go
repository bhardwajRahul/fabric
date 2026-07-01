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

package litellm

import "encoding/json"

// openaiRequest is the body of a POST to the Responses API (/v1/responses) as spoken by the LiteLLM proxy.
type openaiRequest struct {
	Model           string            `json:"model"`
	Input           []openaiInputItem `json:"input"`
	Instructions    string            `json:"instructions,omitzero"`
	Tools           []openaiTool      `json:"tools,omitzero"`
	MaxOutputTokens int               `json:"max_output_tokens,omitzero"`
	Temperature     float64           `json:"temperature,omitzero"`
	// NumRetries is always sent as 0 (no omitzero) to disable LiteLLM's internal retries: its retry decision is
	// status-code-only and cannot detect the poison case, and retries belong in exactly one place (CallLLM).
	NumRetries int `json:"num_retries"`
}

// openaiInputItem is one item in the Responses request input array. The Responses API represents an
// assistant tool call and its result as distinct items (function_call, function_call_output), rather
// than as tool_calls/tool_call_id fields on chat messages.
type openaiInputItem struct {
	Type    string          `json:"type"`               // message, function_call, function_call_output
	Role    string          `json:"role,omitzero"`      // message items only
	Content []openaiContent `json:"content,omitzero"`   // message items only
	CallID  string          `json:"call_id,omitzero"`   // function_call, function_call_output
	Name    string          `json:"name,omitzero"`      // function_call
	Args    string          `json:"arguments,omitzero"` // function_call
	Output  string          `json:"output,omitzero"`    // function_call_output
}

// openaiContent is a typed content part of a message item. Input text uses input_text; assistant text
// echoed back uses output_text.
type openaiContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openaiTool struct {
	Type        string          `json:"type"` // function
	Name        string          `json:"name"`
	Description string          `json:"description,omitzero"`
	Parameters  json.RawMessage `json:"parameters"`
}

// openaiResponse is the body of a Responses API completion.
type openaiResponse struct {
	Output            []openaiOutputItem `json:"output"`
	Status            string             `json:"status"` // completed, incomplete
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Model string      `json:"model"`
	Usage openaiUsage `json:"usage"`
}

// openaiOutputItem is one item in the Responses output array. Text lives in message items (content
// parts of type output_text); tool calls are function_call items correlated by call_id.
type openaiOutputItem struct {
	Type    string          `json:"type"` // message, function_call, reasoning
	Content []openaiContent `json:"content,omitzero"`
	CallID  string          `json:"call_id,omitzero"`
	Name    string          `json:"name,omitzero"`
	Args    string          `json:"arguments,omitzero"`
}

type openaiUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

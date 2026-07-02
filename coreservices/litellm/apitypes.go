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

// openaiRequest is the body of a POST to the Responses API (/v1/responses) as spoken by the LiteLLM
// proxy. Store is always false (the conversation is stateless; not retaining is the privacy-preferring
// default). Include is set only once a model is observed to reason (see service.go), since a
// non-reasoning model rejects the encrypted-reasoning request.
type openaiRequest struct {
	Model           string           `json:"model"`
	Input           []openaiItem     `json:"input"`
	Instructions    string           `json:"instructions,omitzero"`
	Tools           []openaiTool     `json:"tools,omitzero"`
	MaxOutputTokens int              `json:"max_output_tokens,omitzero"`
	Temperature     float64          `json:"temperature,omitzero"`
	Store           bool             `json:"store"`
	Include         []string         `json:"include,omitzero"`
	Reasoning       *openaiReasoning `json:"reasoning,omitzero"`
	// NumRetries is always sent as 0 (no omitzero) to disable LiteLLM's internal retries: its retry decision is
	// status-code-only and cannot detect the poison case, and retries belong in exactly one place (CallLLM).
	NumRetries int `json:"num_retries"`
}

// openaiReasoning carries the reasoning-effort level and requests a summary for display.
type openaiReasoning struct {
	Effort  string `json:"effort,omitzero"`
	Summary string `json:"summary,omitzero"`
}

// openaiItem is one item in a Responses input or output array. The Responses API is item-native: an
// assistant text message, a tool call, a tool result, and a reasoning step are each a distinct item.
// A single struct covers all variants (the unused fields stay zero); the Type discriminates.
type openaiItem struct {
	Type    string          `json:"type"`               // message, function_call, function_call_output, reasoning
	Role    string          `json:"role,omitzero"`      // message
	Content []openaiContent `json:"content,omitzero"`   // message
	CallID  string          `json:"call_id,omitzero"`   // function_call, function_call_output
	Name    string          `json:"name,omitzero"`      // function_call
	Args    string          `json:"arguments,omitzero"` // function_call
	Output  string          `json:"output,omitzero"`    // function_call_output
	// reasoning
	ID               string          `json:"id,omitzero"`                // reasoning item id (rs_...)
	Summary          []openaiSummary `json:"summary,omitzero"`           // reasoning summary parts
	EncryptedContent string          `json:"encrypted_content,omitzero"` // opaque reasoning payload for replay
}

// openaiContent is a typed content part of a message item. Input text uses input_text; assistant text
// echoed back uses output_text.
type openaiContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// openaiSummary is a summarized-reasoning part of a reasoning item.
type openaiSummary struct {
	Type string `json:"type"` // summary_text
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
	Output            []openaiItem `json:"output"`
	Status            string       `json:"status"` // completed, incomplete
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Model string      `json:"model"`
	Usage openaiUsage `json:"usage"`
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

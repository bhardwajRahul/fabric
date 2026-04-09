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

package chatgptllm

import "encoding/json"

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitzero"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitzero"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitzero"`
	ToolCallID string           `json:"tool_call_id,omitzero"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitzero"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openaiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openaiCallFunc `json:"function"`
}

type openaiCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiResponse struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
}

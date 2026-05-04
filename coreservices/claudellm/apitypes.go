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

package claudellm

import "encoding/json"

type claudeRequest struct {
	Model     string               `json:"model"`
	MaxTokens int                  `json:"max_tokens"`
	System    []claudeContentBlock `json:"system,omitzero"`
	Messages  []claudeMessage      `json:"messages"`
	Tools     []claudeTool         `json:"tools,omitzero"`
}

type claudeMessage struct {
	Role    string               `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text,omitzero"`
	ID           string              `json:"id,omitzero"`
	Name         string              `json:"name,omitzero"`
	Input        json.RawMessage     `json:"input,omitzero"`
	ToolUseID    string              `json:"tool_use_id,omitzero"`
	Content      string              `json:"content,omitzero"`
	CacheControl *claudeCacheControl `json:"cache_control,omitzero"`
}

type claudeTool struct {
	Name         string              `json:"name"`
	Description  string              `json:"description,omitzero"`
	InputSchema  json.RawMessage     `json:"input_schema"`
	CacheControl *claudeCacheControl `json:"cache_control,omitzero"`
}

type claudeCacheControl struct {
	Type string `json:"type"`
}

type claudeResponse struct {
	Content    []claudeContentBlock `json:"content"`
	StopReason string               `json:"stop_reason"`
	Model      string               `json:"model"`
	Usage      claudeUsage          `json:"usage"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

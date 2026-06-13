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

// Message is a single message in a conversation with an LLM.
type Message struct {
	Role       string `json:"role,omitzero" jsonschema:"description=Role is the message role: user, assistant, system, or tool"`
	Content    string `json:"content,omitzero" jsonschema:"description=Content is the text content of the message"`
	ToolCallID string `json:"toolCallId,omitzero" jsonschema:"description=ToolCallID pairs a tool result with its tool call (role=tool only)"`
	ToolCalls  string `json:"toolCalls,omitzero" jsonschema:"description=ToolCalls is the JSON-serialized tool calls from the assistant (role=assistant only)"`
	// ThoughtSignature is an opaque, provider-issued token attached to the assistant's text part
	// that must be echoed back on subsequent turns to preserve reasoning continuity. Currently used
	// only by Gemini 2.5 thinking models; other providers leave it empty. Per-tool-call signatures
	// live on ToolCall.ThoughtSignature instead.
	ThoughtSignature string `json:"thoughtSignature,omitzero" jsonschema:"description=ThoughtSignature is an opaque provider token (Gemini 2.5 thinking models) that must be round-tripped to preserve reasoning continuity"`
	// Attachments are non-text parts (images, audio, video, documents) attached to the message.
	// Order is preserved on the wire; providers that don't support multi-modal input silently
	// drop them. See the Attachment type for inline vs URI semantics.
	Attachments []Attachment `json:"attachments,omitzero" jsonschema:"description=Attachments are non-text parts (images/audio/video/documents) attached to this message; ignored by providers without multi-modal support"`
}

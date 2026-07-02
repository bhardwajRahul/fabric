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

// Message is a text message from a single role within a conversation. It is the message variant of an
// Item; tool calls, tool results, and reasoning are separate Item variants rather than fields here.
type Message struct {
	Role    string `json:"role,omitzero" jsonschema_description:"Role is the message role: user, assistant, or system"`
	Content string `json:"content,omitzero" jsonschema_description:"Content is the text content of the message"`
	// Attachments are non-text parts (images, audio, video, documents) attached to the message.
	// Order is preserved on the wire; providers that don't support multi-modal input silently
	// drop them. See the Attachment type for inline vs URI semantics.
	Attachments []Attachment `json:"attachments,omitzero" jsonschema_description:"Attachments are non-text parts (images/audio/video/documents) attached to this message; ignored by providers without multi-modal support"`
}

// NewMessage builds a message, for use with AppendItems or an Item literal.
func NewMessage(role, content string) *Message {
	return &Message{Role: role, Content: content}
}

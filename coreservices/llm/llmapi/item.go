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

// ItemType names the kind of an Item, returned by Item.Type.
type ItemType string

const (
	ItemMessage    ItemType = "message"     // a text message from user/assistant/system
	ItemToolCall   ItemType = "tool_call"   // the model's request to invoke a tool
	ItemToolResult ItemType = "tool_result" // the result of a tool invocation
	ItemReasoning  ItemType = "reasoning"   // opaque provider reasoning, round-tripped for continuity
	ItemUnknown    ItemType = ""            // no variant is populated
)

// Item is one entry in a conversation. A conversation is an ordered, append-only list of Items, the
// provider-neutral shape all provider microservices translate to and from their native wire format
// (OpenAI Responses items, Anthropic content blocks, Gemini parts). Item is a discriminated union:
// exactly one of the pointer fields is populated, and Type reports which.
type Item struct {
	Message    *Message    `json:"message,omitzero" jsonschema_description:"Message is a user/assistant/system text message"`
	ToolCall   *ToolCall   `json:"toolCall,omitzero" jsonschema_description:"ToolCall is the model's request to invoke a tool"`
	ToolResult *ToolResult `json:"toolResult,omitzero" jsonschema_description:"ToolResult is the result of a tool invocation"`
	Reasoning  *Reasoning  `json:"reasoning,omitzero" jsonschema_description:"Reasoning is opaque provider reasoning round-tripped for continuity"`
}

// Type reports which variant the Item carries, derived from the populated pointer field. A malformed
// Item with zero or more than one pointer set is ItemUnknown, so the "exactly one populated" invariant
// is reported rather than silently resolved to the first-set field.
func (it Item) Type() ItemType {
	t := ItemUnknown
	n := 0
	if it.Message != nil {
		t, n = ItemMessage, n+1
	}
	if it.ToolCall != nil {
		t, n = ItemToolCall, n+1
	}
	if it.ToolResult != nil {
		t, n = ItemToolResult, n+1
	}
	if it.Reasoning != nil {
		t, n = ItemReasoning, n+1
	}
	if n != 1 {
		return ItemUnknown
	}
	return t
}

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

// Reasoning is opaque, provider-issued reasoning carried across turns to preserve the model's internal
// continuity. The three payload kinds are distinct and disjoint per provider: Summary is human-readable
// text, Signature authenticates the item, EncryptedContent is the encrypted reasoning itself.
//   - OpenAI: ID + Summary (summary_text parts) + EncryptedContent
//   - Anthropic thinking: Summary (thinking text, one element) + Signature; redacted: EncryptedContent
//   - Gemini: Signature (thought signature)
//
// Providers that don't recognize a Reasoning item drop it.
type Reasoning struct {
	ID               string   `json:"id,omitzero" jsonschema_description:"ID is the provider reasoning-item identifier (OpenAI rs_...); empty for others"`
	Summary          []string `json:"summary,omitzero" jsonschema_description:"Summary is human-readable reasoning text parts (OpenAI summary parts, Anthropic thinking text); kept as an array, not joined"`
	Signature        string   `json:"signature,omitzero" jsonschema_description:"Signature is an opaque token authenticating this item for replay (Anthropic thinking signature, Gemini thought signature)"`
	EncryptedContent string   `json:"encryptedContent,omitzero" jsonschema_description:"EncryptedContent is the opaque encrypted reasoning payload for replay (OpenAI encrypted_content, Anthropic redacted_thinking data)"`
}

// AsItem wraps the reasoning as an Item, for building a conversation slice.
func (r Reasoning) AsItem() Item {
	return Item{Reasoning: &r}
}

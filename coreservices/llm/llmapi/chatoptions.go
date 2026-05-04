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

// ChatOptions configures a Chat invocation. Zero-valued fields fall back to defaults.
type ChatOptions struct {
	MaxToolRounds int     `json:"maxToolRounds,omitzero" jsonschema:"description=MaxToolRounds caps tool-call round-trips for this chat (overrides MaxToolRounds config)"`
	MaxTokens     int     `json:"maxTokens,omitzero" jsonschema:"description=MaxTokens caps the response length per turn"`
	Temperature   float64 `json:"temperature,omitzero" jsonschema:"description=Temperature controls sampling randomness"`
}

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

// Usage reports token consumption for an LLM invocation.
// Providers populate per-turn usage with Turns=1; Chat aggregates across turns.
type Usage struct {
	InputTokens      int    `json:"inputTokens,omitzero" jsonschema:"description=InputTokens is the number of prompt tokens charged"`
	OutputTokens     int    `json:"outputTokens,omitzero" jsonschema:"description=OutputTokens is the number of completion tokens generated"`
	CacheReadTokens  int    `json:"cacheReadTokens,omitzero" jsonschema:"description=CacheReadTokens is the number of tokens served from the provider's prompt cache"`
	CacheWriteTokens int    `json:"cacheWriteTokens,omitzero" jsonschema:"description=CacheWriteTokens is the number of tokens written to the provider's prompt cache"`
	Model            string `json:"model,omitzero" jsonschema:"description=Model is the provider's model identifier that produced this completion"`
	Turns            int    `json:"turns,omitzero" jsonschema:"description=Turns is the number of LLM turns aggregated in this Usage"`
}

// Add accumulates other into u. Model is overwritten by the latest non-empty value.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.CacheWriteTokens += other.CacheWriteTokens
	u.Turns += other.Turns
	if other.Model != "" {
		u.Model = other.Model
	}
}

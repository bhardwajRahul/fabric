## Design Rationale

This microservice implements the `Turn` endpoint for the ChatGPT LLM provider. It translates between the Microbus LLM message format and the OpenAI Chat Completions API format, handling tool_calls on assistant messages and tool_call_id on tool result messages.

The `model` argument is required per call (no `Model` config); use the typed constants in `chatgptllmapi` (e.g. `chatgptllmapi.ModelGPT4o`) for compile-time checking.

`Turn` populates `llmapi.Usage` from the OpenAI `usage` block. OpenAI reports `prompt_tokens` and `completion_tokens` plus `prompt_tokens_details.cached_tokens`. We map the cached portion to `CacheReadTokens` and report the remainder as `InputTokens`. OpenAI does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

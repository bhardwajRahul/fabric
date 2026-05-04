## Design Rationale

This microservice implements the `Turn` endpoint for the Google Gemini LLM provider. It translates between the Microbus LLM message format and the Gemini generateContent API format, handling functionCall/functionResponse parts and role mapping (assistant→model).

The `model` argument is required per call (no `Model` config); use the typed constants in `geminillmapi` (e.g. `geminillmapi.ModelGemini20Flash`) for compile-time checking.

`Turn` populates `llmapi.Usage` from the Gemini `usageMetadata` block. Gemini reports `promptTokenCount`, `candidatesTokenCount`, and `cachedContentTokenCount`. We map the cached portion to `CacheReadTokens` and report the remainder as `InputTokens`. Gemini does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

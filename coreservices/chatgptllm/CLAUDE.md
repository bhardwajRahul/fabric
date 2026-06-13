## Design Rationale

This microservice implements the `Turn` endpoint for the ChatGPT LLM provider. It translates between the Microbus LLM message format and the OpenAI Chat Completions API format, handling tool_calls on assistant messages and tool_call_id on tool result messages.

The `model` argument is required per call (no `Model` config); use the typed constants in `chatgptllmapi` (e.g. `chatgptllmapi.ModelGPT4o`) for compile-time checking.

`Turn` populates `llmapi.Usage` from the OpenAI `usage` block. OpenAI reports `prompt_tokens` and `completion_tokens` plus `prompt_tokens_details.cached_tokens`. We map the cached portion to `CacheReadTokens` and report the remainder as `InputTokens`. OpenAI does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## Empty Response Diagnostics

When a turn returns no text and no tool calls, the provider emits a `Debug` log with the model,
finishReason, normalized stopReason, and the raw response body (enable via `MICROBUS_LOG_DEBUG=1`).
The common cause is OpenAI returning HTTP 200 with an empty `choices` array - a prompt-level
content-filter block. Without this, the empty response falls through to `stopReason == ""`
(`StopReasonUnknown`) and surfaces only as an opaque `502 unknown stop reason` at `llm.core`,
masking the real cause. The raw body is the load-bearing field: a content-filter block carries its
reason there. This mirrors the same diagnostic in the Gemini provider so the three providers behave
alike on the no-content shape. The response body is read into a buffer (`io.ReadAll`) before
decoding so it is available to log.

## Stop-Reason Mapping

`Turn` returns a normalized `stopReason` (constants in `llmapi/stopreason.go`). `mapFinishReason`
in `service.go` translates OpenAI's `finish_reason` field:

| OpenAI                        | Normalized                  |
|-------------------------------|-----------------------------|
| `stop`                        | `StopReasonEndTurn`         |
| `tool_calls` / `function_call`| `StopReasonToolUse`         |
| `length`                      | `StopReasonMaxTokens`       |
| `content_filter`              | `StopReasonRefusal`         |
| `""` / anything else          | `StopReasonUnknown`         |

OpenAI doesn't carry a separate `stop_sequence` finish reason — both natural end and hitting a
configured stop string surface as `stop`, so both map to `StopReasonEndTurn`. `function_call` is
the legacy alias for `tool_calls`; we map it to the same value for the older models that still
emit it.

`llm.core` interprets these — see `coreservices/llm/CLAUDE.md` "Stop-Reason Branching". An empty or
unrecognized value reaches `llm.core` as `StopReasonUnknown` and surfaces as a `502` rather than as
a silent completion.

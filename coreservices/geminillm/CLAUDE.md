## Design Rationale

This microservice implements the `Turn` endpoint for the Google Gemini LLM provider. It translates between the Microbus LLM message format and the Gemini generateContent API format, handling functionCall/functionResponse parts and role mapping (assistant→model).

The `model` argument is required per call (no `Model` config); use the typed constants in `geminillmapi` (e.g. `geminillmapi.ModelGemini20Flash`) for compile-time checking.

`Turn` populates `llmapi.Usage` from the Gemini `usageMetadata` block. Gemini reports `promptTokenCount`, `candidatesTokenCount`, `cachedContentTokenCount`, and (on 2.5 thinking models) `thoughtsTokenCount`. We map the cached portion to `CacheReadTokens` and report the remainder as `InputTokens`; `OutputTokens` is `candidatesTokenCount + thoughtsTokenCount` (so it reflects total billed completion, the cross-provider invariant for `llmapi.Usage`) and `ThinkingTokens` breaks out the thoughts portion for observability. Gemini does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## System Instructions

Gemini has a dedicated `systemInstruction` field on the request, separate from the `contents` array. The provider hoists every `role: "system"` message out of the message list and concatenates them (blank-line separated) into a single `systemInstruction.parts[0].text`. They are not also emitted as `contents` entries — doing so would create user/user adjacency at the start (system-as-user followed by the real user turn) and isn't the contract Gemini documents. If callers rely on system messages appearing in the visible transcript, that's the LLM-core layer's responsibility, not ours.

## Thinking and Thought Signatures

Gemini 2.5 models (`gemini-2.5-flash`, `gemini-2.5-pro`) ship with thinking enabled and add two new shapes the provider has to handle:

- **Thought parts** — `{"text": "...", "thought": true}` parts in the response that carry the model's internal reasoning, not the answer. The parser sets `Thought` on `geminiPart` and skips thought parts when accumulating `content`. Letting them through would leak reasoning into the assistant message body and confuse downstream parsers expecting a clean final answer.
- **Thought signatures** — opaque base64-encoded tokens attached to parts (text or functionCall) that the caller is expected to echo back on subsequent turns to preserve reasoning continuity across a multi-turn tool-calling conversation. Without round-trip, 2.5 models can lose the thread after a few rounds of tool use and return an empty STOP response (i.e. give up).

We carry the signature out via two new `llmapi` fields:
- `llmapi.Message.ThoughtSignature` — for the assistant's text part (last-wins if multiple text parts have signatures; in practice Gemini only attaches it to the last visible text).
- `llmapi.ToolCall.ThoughtSignature` — per function-call part.

Both are surfaced with the `omitzero` JSON tag and are silently ignored by every other provider, so adding them is non-breaking. On the next `Turn` invocation, the provider re-emits them on the corresponding parts of the model-role content it reconstructs.

If you're touching the message-conversion code, the invariant to preserve is: **every part the model sent us with a thoughtSignature must come back to it on the next turn with that same signature attached**. Dropping a signature is silently corrupting — it doesn't fail, but the model's quality on subsequent turns degrades.

## Multimodal Attachments

The provider supports outbound `llmapi.Message.Attachments` (images, audio, video, documents) sent to Gemini, but does **not** yet surface inbound multimodal parts back through `Turn`. The asymmetry is intentional:

- **Outbound** (common case — send a screenshot to Gemini for analysis): each `llmapi.Attachment` is converted to a `geminiPart`. If `Data` is set, the bytes go into `inlineData` (Go's `encoding/json` handles base64 transparently). If `URI` is set, the reference goes into `fileData` (works with Gemini File API URIs or public HTTPS URLs). Order is preserved within the message's parts.
- **Inbound** (image-generation models like `gemini-2.5-flash-image-preview` that return `inlineData` parts containing produced media): the response decoder counts such parts and logs a `Debug` message indicating they were dropped. Surfacing them requires widening the `Turn` return signature across all four providers and the client stub, which is a deliberate future refactor — when an agent needs to consume produced media, do that change as one intentional cross-provider PR rather than ad-hoc here.

Messages with neither `Content` nor convertible attachments are silently skipped to avoid hitting Gemini's INVALID_ARGUMENT for empty `parts` arrays.

## Empty Response Diagnostics

When a turn returns no text and no tool calls (the smoking-gun shape for downstream "LLM returned no final assistant content" errors), the provider emits a `Debug` log with the model, finishReason, normalized stopReason, and the raw response body. An empty response with `finishReason: STOP` legitimately maps to `end_turn`, and the caller decides what to do — this isn't a failure, just a diagnostic. Enable via `MICROBUS_LOG_DEBUG=1` when investigating. The raw body is the load-bearing field: if Gemini ever starts returning a field we don't decode, it shows up here without needing a separate capture step.

## Stop-Reason Mapping

`Turn` returns a normalized `stopReason` (constants in `llmapi/stopreason.go`). `mapFinishReason`
in `service.go` translates Gemini's `finishReason` field:

| Gemini                                                              | Normalized               |
|---------------------------------------------------------------------|--------------------------|
| `STOP` + tool calls present                                         | `StopReasonToolUse`      |
| `STOP` + no tool calls                                              | `StopReasonEndTurn`      |
| `MAX_TOKENS`                                                        | `StopReasonMaxTokens`    |
| `SAFETY` / `RECITATION` / `BLOCKLIST` / `PROHIBITED_CONTENT` / `SPII` | `StopReasonRefusal`    |
| `""` / anything else (`OTHER`, `MALFORMED_FUNCTION_CALL`, `LANGUAGE`, ...) | `StopReasonUnknown` |

Gemini is unique in that `STOP` covers both natural end and tool-call turns — the API doesn't have
a `tool_use`-equivalent finish reason. `mapFinishReason` therefore takes a `hasToolCalls bool` and
disambiguates from the parsed response. The safety-family reasons all collapse to `refusal`
because the caller-facing meaning is the same: the model declined to respond, and the orchestrator
should fail rather than emit a possibly-policy-flagged partial.

`llm.core` interprets these — see `coreservices/llm/CLAUDE.md` "Stop-Reason Branching". An empty or
unrecognized value reaches `llm.core` as `StopReasonUnknown` and surfaces as a `502` rather than as
a silent completion. `MALFORMED_FUNCTION_CALL` deliberately falls into `Unknown` rather than being
mapped to a completion or a refusal — a malformed call is a provider/data bug, not a routine
outcome, and surfacing it lets the operator notice.

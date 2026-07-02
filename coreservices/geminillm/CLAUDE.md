## Design Rationale

This microservice implements the `Turn` endpoint for the Google Gemini LLM provider. It translates between the Microbus LLM message format and the Gemini generateContent API format, handling functionCall/functionResponse parts and role mapping (assistant→model).

The provider ships no typed model catalog: provider model IDs rotate every quarter, so a maintained `Model*` const list would always be stale and removing entries would break downstream compilation. Instead `resolveModel` (in `service.go`) maps a capability tier (`fast`/`default`/`smart`) or a Gemini family alias (`flash`/`pro`/`flash-lite`) to a current concrete model, passes through any `gemini-` prefixed name as-is (so a newly released model works before it is listed), and returns `""` for anything else. `Turn` calls it at the top: a recognized alias/name resolves, an unrecognized string passes through unchanged so an explicit-provider call to a brand-new model still reaches the API. Every tier points at a Gemini floating `-latest` alias (`gemini-flash-lite-latest`, `gemini-flash-latest`, `gemini-pro-latest`), so all self-update to the current model with no code change. The alias table is held in `svc.modelAliases` (a small runtime map, not a hand-maintained public catalog); the Phase 2 live `/v1/models` lookup repopulates it if an override is ever needed.

`OnResolveProvider` is this provider's sink for the `llm.core` resolve event: it answers `ok = APIKey configured && resolveModel(model) != ""`, so `llm.core` selects this provider under an empty/`"any"` request only when Gemini holds a key and recognizes the model. See `coreservices/llm/CLAUDE.md` "Provider and Model Resolution".

`Turn` populates `llmapi.Usage` from the Gemini `usageMetadata` block. Gemini reports `promptTokenCount`, `candidatesTokenCount`, `cachedContentTokenCount`, and (on 2.5 thinking models) `thoughtsTokenCount`. We map the cached portion to `CacheReadTokens` and report the remainder as `InputTokens`; `OutputTokens` is `candidatesTokenCount + thoughtsTokenCount` (so it reflects total billed completion, the cross-provider invariant for `llmapi.Usage`) and `ThinkingTokens` breaks out the thoughts portion for observability. Gemini does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## System Instructions

Gemini has a dedicated `systemInstruction` field on the request, separate from the `contents` array. The provider hoists every `role: "system"` message out of the message list and concatenates them (blank-line separated) into a single `systemInstruction.parts[0].text`. They are not also emitted as `contents` entries — doing so would create user/user adjacency at the start (system-as-user followed by the real user turn) and isn't the contract Gemini documents. If callers rely on system messages appearing in the visible transcript, that's the LLM-core layer's responsibility, not ours.

## Thinking and Thought Signatures

Gemini 2.5 models (`gemini-2.5-flash`, `gemini-2.5-pro`) ship with thinking enabled and add two new shapes the provider has to handle:

- **Thought parts** — `{"text": "...", "thought": true}` parts in the response that carry the model's internal reasoning, not the answer. The parser sets `Thought` on `geminiPart` and skips thought parts when accumulating `content`. Letting them through would leak reasoning into the assistant message body and confuse downstream parsers expecting a clean final answer.
- **Thought signatures** — opaque base64-encoded tokens attached to parts (text or functionCall) that the caller is expected to echo back on subsequent turns to preserve reasoning continuity across a multi-turn tool-calling conversation. Without round-trip, 2.5 models can lose the thread after a few rounds of tool use and return an empty STOP response (i.e. give up).

We carry the signature via `llmapi.Reasoning` items (`Reasoning.Signature`). In the item model there is
no per-part signature field; instead a signature is a standalone reasoning item positioned immediately
before the item it binds to:
- A response part carrying a `thoughtSignature` is emitted as a `Reasoning{Signature}` item just before
  the message or tool_call item it belonged to.
- A `thought:true` part (the model's internal reasoning text) becomes a `Reasoning{Summary}` item.

On the next `Turn`, the input converter holds each reasoning item's signature in a `pendingSig` and
attaches it to the next text/functionCall part it emits, re-gluing what the split separated. This is the
same positional-adjacency contract OpenAI's Responses API uses for reasoning items, and the conversation
slice preserves order across the bus and the workflow append-reducer, so adjacency is reliable.

If you're touching the conversion code, the invariant to preserve is: **every part the model sent us with
a thoughtSignature must come back to it on the next turn with that same signature attached** (as a
reasoning item immediately before its part). Dropping a signature is silently corrupting — it doesn't
fail, but the model's quality on subsequent turns degrades.

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

## Error Classification

The provider-to-ChatLoop contract is a single attribute, **`retryAfter`**: present => retryable (and that is the
wait), absent => permanent. ChatLoop holds no policy and never inspects the status code; the provider makes the call.
The upstream status code is left **authentic** (never remapped) - it is for HTTP semantics and observability.

On a non-OK response the provider parses Gemini's `{error:{message,status,details[]}}` body and constitutes a
`TracedError` via `errors.New`: the text is the body's `error.message`; the attributes are the authentic upstream
status code, `status` (e.g. `RESOURCE_EXHAUSTED`), `quotaId` and `quotaValue` (from `details[].google.rpc.QuotaFailure`),
`retryDelay` (the raw, second-truncated value from `details[].google.rpc.RetryInfo`), and `retryAfter` (the retry
signal, only when retryable). Gemini returns **no rate-limit headers** - the wait and the quota detail live only in
the body's `details[]` array, a heterogeneous list of typed objects keyed by `@type` (`google.rpc.Help`,
`google.rpc.QuotaFailure`, `google.rpc.RetryInfo`).

### Poison vs transient: the request's token count decides whether to attach retryAfter

This is the hard case. **Gemini returns the same `429 RESOURCE_EXHAUSTED` whether the request is unfixable or merely
transiently throttled**, and unlike OpenAI the body carries no requested-token count to compare against the limit. So
the provider obtains the request's token count and compares it to `QuotaFailure.quotaValue`.

**Cheap short-circuit first.** A token spans at least one byte, so the marshaled request's byte length is a hard
ceiling on its token count (`tokens <= bytes`, true even for byte-level tokenizers - rune count is *not* a safe
ceiling, since a rare multi-byte character can tokenize to several tokens). When that ceiling already fits the quota
(`len(body) < quotaValue`), the request provably cannot be poison, so it is classified retryable without any further
call. This covers the common case - most requests are far under a 1M-tokens/minute quota - and the byte length of the
full request also folds in the system instruction and tools, so nothing sent is missed.

**Exact count only when it might matter.** When the byte ceiling reaches the quota (a genuinely huge request, the only
place poison is possible), the count comes from Gemini's **`countTokens` endpoint** (`countInputTokens`), which
returns the exact input token count for the same request without generating. Exact (not a heuristic) because
tokens-per-word varies too much for a never-retry decision: ~1.3 for English prose but 2-3 for code/JSON, higher for
non-Latin scripts, dictionary varying by model. `countTokens` runs on a separate, free quota, so it still answers
while generation is rate-limited, and it is wrapped in a `generateContentRequest` so the system instruction and tools
are counted, not just `contents`.

If `countTokens` itself fails, the provider does not estimate the count itself - it treats the request as
non-retryable (fail closed). The byte ceiling already showed the request might exceed the quota, and the only unsafe
mistake is retrying a permanently-too-large request forever, so when the exact count is unavailable we decline to
retry rather than guess. The sound `bytes >= tokens` short-circuit is the only token arithmetic the provider does
itself.

The comparison:

- count > quota (poison): a single request larger than the entire per-minute budget, can never succeed. **No**
  `retryAfter`; ChatLoop will not retry.
- a `quotaId` ending in `PerDay` (daily-quota exhaustion): a give-up, not a per-minute throttle. **No** `retryAfter`.
- otherwise (count <= quota, per-minute quota): a genuine transient overflow - an earlier request in the same
  minute filled the window. `retryAfter` is attached.

Because Google returns `429` in both poison and transient cases, the authentic status is `429` either way; the only
difference is whether `retryAfter` is present, and that verdict is ours, not Google's. The raw `quotaValue`,
`quotaId`, and `retryDelay` are always attached so a caller of `Chat` can see the full picture regardless.

### Why `retryAfter` adds a 1s margin

`RetryInfo.retryDelay` is **truncated to whole seconds**. Verified against live responses: when the precise wait was
`42.921954507s` (stated in the prose `message`), the structured `retryDelay` read `42s`; an earlier capture showed
`1.466…s` reported as `1s`. Following the structured value as-is therefore retries up to ~1s early and risks
immediately re-tripping the same 429. The provider parses `retryDelay` and adds a 1s margin before storing it as
`retryAfter`. The structured field is used (not the precise prose value) because it is stable; the prose format is
not a contract. The 1s margin is the cheap, robust correction for its truncation.

### Example A: single oversized request (poison, no retryAfter)

A ~1.5M-token request against the 1,000,000/min quota. The single request alone exceeds the whole budget, so no wait
helps. `countTokens` returns ~1.5M > `quotaValue`, so **no** `retryAfter` is attached and ChatLoop will not retry.

```
status 429   (authentic; no retryAfter attached because countTokens > quotaValue)
headers: (no rate-limit headers; X-Gemini-Service-Tier: standard only)
body:
  {"error":{"code":429,"status":"RESOURCE_EXHAUSTED",
    "message":"You exceeded your current quota ... limit: 1000000, model: gemini-2.5-flash\nPlease retry in 1.466542669s.",
    "details":[
      {"@type":"type.googleapis.com/google.rpc.QuotaFailure",
       "violations":[{"quotaId":"GenerateContentPaidTierInputTokensPerModelPerMinute","quotaValue":"1000000"}]},
      {"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"1s"}]}}
```

### Example B: burst overflow (transient, retryAfter attached)

Several ~360k-token requests back-to-back in the same minute. The first ones succeed; the overflowing one is rejected
because they together exceed the window, even though no single request exceeds the quota. `countTokens` returns
~360k <= `quotaValue`, so `retryAfter` is attached (raw `retryDelay` + 1s margin) and ChatLoop retries.

```
status 429   (authentic; retryAfter attached)
body:
  {"error":{"code":429,"status":"RESOURCE_EXHAUSTED",
    "message":"You exceeded your current quota ... limit: 1000000, model: gemini-2.5-flash\nPlease retry in 42.921954507s.",
    "details":[
      {"@type":"type.googleapis.com/google.rpc.QuotaFailure",
       "violations":[{"quotaId":"GenerateContentPaidTierInputTokensPerModelPerMinute","quotaValue":"1000000"}]},
      {"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"42s"}]}}
```

A `quotaId` ending in `PerDay` (daily-quota exhaustion) should be treated as a give-up rather than retried; that
case is not yet special-cased because the probes only reached the per-minute quota.

### 503 UNAVAILABLE

A `503 UNAVAILABLE` is a transient overload (the model is momentarily swamped), not a permanent failure, and Gemini
attaches no `retryDelay` to it. The provider therefore retries it with a modest default wait (5s) rather than the
body-derived value, arming the same preemption gate. This keeps Gemini symmetric with Anthropic's retryable `529`
overload, where the `429` poison/transient distinction above does not apply.

## Rate-Limit Preemption

The provider keeps an in-memory `map[model]blockedUntil` (mutex-guarded). On a `retryAfter`-bearing throttle it
records `blockedUntil[model] = now + retryAfter`, and at the top of `Turn` it preempts a call to a still-blocked model
with a synthetic `429` (carrying the remaining wait as `retryAfter`) *without* dialing Gemini. This stops every
in-flight caller from each eating its own real `429` before backing off.

The gate lives in the provider, not llm.core, because the provider holds the API key - it unambiguously *is* one
account and can gate before dialing, and a gate in `Turn` covers every caller (`Chat`, `CallLLM`, and direct
`ForHost`), not just llm.core-routed traffic. It is keyed by **model** because Gemini's quota is per-model
(`quotaId` is `...PerModelPerMinute`); an account-wide gate would over-block sibling models. Only `retryAfter`-bearing
errors arm it (poison and `PerDay` give-ups never do). Per-replica, no cross-replica gossip.

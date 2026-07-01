## Design Rationale

This microservice implements the `Turn` endpoint for the ChatGPT LLM provider. It translates between the Microbus LLM message format and the OpenAI Chat Completions API format, handling tool_calls on assistant messages and tool_call_id on tool result messages.

The `model` argument is required per call (no `Model` config) and is a passthrough string (e.g. `"gpt-5.4-mini"`). The provider deliberately ships no typed model catalog: provider model IDs rotate every quarter, so a maintained `Model*` const list would always be stale and removing entries would break downstream compilation. The planned ergonomic replacement is alias resolution (a family/tier name like `mini` or `smart` resolved to a current concrete model at runtime), not a hand-maintained catalog.

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

## Error Classification

The provider-to-ChatLoop contract is a single attribute, **`retryAfter`**: present => retryable (and that is the
wait), absent => permanent. ChatLoop holds no policy and never inspects the status code; the provider makes the call.
The upstream status code is left **authentic** (never remapped) - it is for HTTP semantics and observability.

On a non-OK response the provider parses OpenAI's `{error:{message,type,code,param}}` body and constitutes a
`TracedError` via `errors.New`: the text is the body's `error.message`; the attributes are the authentic upstream
status code, `type`, `code`, `param` (only when non-null), `organization` (extracted from the message with the regex
`org-[A-Za-z0-9]+` - OpenAI puts the org id in the prose, not a header), `X-Request-Id`, and the **entire
`X-Ratelimit-*` header family** (`HeaderAttrs`: limit/remaining/reset for both requests and tokens) so a caller of
`Chat` can make its own retry decision from the raw values.

**OpenAI disguises an unfixable request as a 429.** A request whose token count exceeds the entire per-minute token
budget (TPM) comes back as `429` with `code:"rate_limit_exceeded"` - on the wire identical to a transient throttle,
but it can never succeed by waiting. The provider classifies it by *not* attaching `retryAfter`:

- A `429 rate_limit_exceeded` whose request fits the limit (no `Requested > Limit` in the message) is a genuine,
  waitable throttle: `retryAfter` is set to the longest positive `X-Ratelimit-Reset-*` (tokens or requests), or a
  60s fallback.
- The poison case - `Requested > Limit` (regex `Limit (\d+), Requested (\d+)`), a single request larger than the
  whole budget - and `insufficient_quota` (billing) get **no** `retryAfter`. ChatLoop will not retry them. The status
  stays an authentic `429`; the `requested > limit` arithmetic in the message is the human-readable proof it is
  permanent.

### Example: request exceeds the per-minute token budget (live capture)

A ~250k-token request against an account whose TPM limit is 30000. The request is larger than the entire minute's
budget, so no wait can help: a `429` with no `retryAfter` attached.

```
status 429   (authentic; no retryAfter attached because Requested > Limit)
headers (all captured as attributes):
  X-Ratelimit-Limit-Tokens:     30000
  X-Ratelimit-Remaining-Tokens: 30000
  X-Ratelimit-Reset-Tokens:     0s
  X-Ratelimit-Reset-Requests:   120ms
  X-Request-Id:                 req_fca0e758794340b397e34649c09e3c41
body:
  {"error":{
     "message":"Request too large for gpt-4o in organization org-QYrP4ECVZ7NglfryGTiPWUWE on tokens per min (TPM): Limit 30000, Requested 250011. The input or output tokens must be reduced in order to run successfully. ...",
     "type":"tokens","param":null,"code":"rate_limit_exceeded"}}
```

A genuinely transient throttle (request fits the budget but the minute's quota is momentarily spent) arrives as the
same `429`/`rate_limit_exceeded` but with `Requested <= Limit`; that case gets a `retryAfter` from the reset headers
and ChatLoop retries it. (Not yet reproduced live - this account walls on the poison case first.)

## Rate-Limit Preemption

The provider keeps an in-memory `map[model]blockedUntil` (mutex-guarded). On a `retryAfter`-bearing throttle it
records `blockedUntil[model] = now + retryAfter`, and at the top of `Turn` it preempts a call to a still-blocked model
with a synthetic `429` (carrying the remaining wait as `retryAfter`) *without* dialing OpenAI. This stops every
in-flight caller from each eating its own real `429` before backing off.

The gate lives in the provider, not llm.core, because the provider holds the API key - it unambiguously *is* one
account and can gate before dialing, and a gate in `Turn` covers every caller (`Chat`, `CallLLM`, and direct
`ForHost`), not just llm.core-routed traffic. It is keyed by **model** because OpenAI's limits are per-model; an
account-wide gate would over-block sibling models. Only `retryAfter`-bearing errors arm it (the poison case and
`insufficient_quota` never do). Per-replica, no cross-replica gossip.

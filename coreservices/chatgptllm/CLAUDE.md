## Design Rationale

This microservice implements the `Turn` endpoint for the ChatGPT LLM provider. It translates between the Microbus LLM message format and the OpenAI Responses API (`/v1/responses`) format.

### Why the Responses API

The provider targets the Responses API rather than the older Chat Completions API. The two are wire-incompatible in three
places that the translation layer bridges, all internal to this microservice - the `Turn` contract (the `llmapi` types
crossing the bus) is unchanged, so nothing downstream is affected:

- **Conversation shape.** Chat Completions carries everything in a flat `messages` array where an assistant tool call is a
  `tool_calls` field and its result is a `role:"tool"` message keyed by `tool_call_id`. Responses uses an `input` array of
  typed items: a text turn is a `message` item (with `content` parts of type `input_text` for user text, `output_text` for
  assistant text echoed back), a tool call is a `function_call` item, and a tool result is a `function_call_output` item.
  Calls and results are correlated by `call_id` rather than by message adjacency. This is a near one-to-one mapping onto the
  Microbus `[]llmapi.Item` model (the neutral conversation shape), which `Turn` translates in order: a `system` `Message`
  item folds into the top-level `instructions` string, a `message` item maps to a Responses `message`, a `tool_call` item to
  a `function_call`, a `tool_result` item to a `function_call_output`, and a `reasoning` item to a `reasoning` item (see
  Reasoning Items and Replay below).
- **Tools.** Responses drops the `{type:"function", function:{...}}` wrapper - the function's `name`, `description`, and
  `parameters` sit flat on the tool object.
- **Response walk.** There are no `choices`; the response is an `output` array of typed items, appended to the returned
  `[]Item` in order. Text is gathered from `output_text` content parts of `message` items; tool calls come from
  `function_call` items (their `call_id` becomes `ToolCall.ID`); `reasoning` items are captured for replay, not discarded.

`max_tokens` is renamed `max_output_tokens`.

The `model` argument is required per call (no `Model` config) and is a passthrough string (e.g. `"gpt-5.4-mini"`). The provider deliberately ships no typed model catalog: provider model IDs rotate every quarter, so a maintained `Model*` const list would always be stale and removing entries would break downstream compilation. The planned ergonomic replacement is alias resolution (a family/tier name like `mini` or `smart` resolved to a current concrete model at runtime), not a hand-maintained catalog.

## Reasoning Items and Replay

`Turn` speaks the `[]llmapi.Item` conversation model. The item log maps one-to-one onto Responses input
items and preserves order, so a reasoning item stays adjacent to the function_call it belongs to. On the
response, `reasoning` output items are captured into `llmapi.Reasoning` items (`ID` + `Summary` +
`EncryptedContent`) interleaved in order with the message and tool_call items.

Reasoning continuity is **manual replay, not `previous_response_id`**: the conversation is the full item
log passed on every turn (stateless), so `previous_response_id` (server-retained state) is the wrong
mechanism. Instead we request the encrypted reasoning payload and echo reasoning items back next turn.
That requires `include:["reasoning.encrypted_content"]` on the request, and `store` is always `false`
(a plain bool - we never rely on OpenAI retaining responses, and not retaining is the privacy default).

**Whether to send `include` is detected at runtime, not from a model-name list.** Two live-API
constraints shape it: a non-reasoning model (e.g. `gpt-4o-mini`) rejects
`include=reasoning.encrypted_content` with a `400`, and an echoed reasoning item must carry a `summary`
array (empty is fine; omitting it is a `400`). So the provider keeps a per-replica
`map[model]bool reasoningSeen`: a model is added when a response bills reasoning tokens
(`usage.output_tokens_details.reasoning_tokens > 0`), and `include` is sent only for models already in
that set. This replaces a hardcoded reasoning-family prefix list, which would rot as models rotate.

**Consequence - first-encounter turn has no replay.** Because the model is added to `reasoningSeen`
only *after* its first response, the very first call to a model on a replica goes out without `include`,
so that turn's reasoning items come back with no encrypted payload and can't be replayed (the input
converter skips reasoning items whose `EncryptedContent` is empty). Every turn after the first replays
normally. This first-turn gap is the price of not hardcoding model names; the eventual fix is an upfront
capability signal from the provider's models-list API (the provider-portability work), not a name list.

`Turn` populates `llmapi.Usage` from the Responses `usage` block. Responses reports `input_tokens` and `output_tokens` plus `input_tokens_details.cached_tokens` and `output_tokens_details.reasoning_tokens`. The cached portion maps to `CacheReadTokens` with the remainder reported as `InputTokens`; `reasoning_tokens` maps to `ThinkingTokens` (the reasoning subset of `OutputTokens`, populated for GPT-5-class reasoning models and zero otherwise). Responses does not expose write counts so `CacheWriteTokens` is left at zero.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## Empty Response Diagnostics

When a turn returns no text and no tool calls, the provider emits a `Debug` log with the model,
response `status`, `incomplete_details.reason`, normalized stopReason, and the raw response body
(enable via `MICROBUS_LOG_DEBUG=1`). The common cause is an `incomplete` response with a
content-filter reason, or a completed turn whose `output` array carried only a `reasoning` item.
Without this, the empty response falls through to `stopReason == ""` (`StopReasonUnknown`) and
surfaces only as an opaque `502 unknown stop reason` at `llm.core`, masking the real cause. The raw
body is the load-bearing field. This mirrors the same diagnostic in the Gemini provider so the three
providers behave alike on the no-content shape. The response body is read into a buffer (`io.ReadAll`)
before decoding so it is available to log.

## Stop-Reason Mapping

`Turn` returns a normalized `stopReason` (constants in `llmapi/stopreason.go`). The Responses API has
no single `finish_reason` field, so `mapStopReason` in `service.go` derives it from the top-level
`status`, `incomplete_details.reason`, and whether any `function_call` items were emitted:

| Responses `status` / `incomplete_details.reason` | Normalized            |
|--------------------------------------------------|-----------------------|
| `completed` with `function_call` items           | `StopReasonToolUse`   |
| `completed`, no tool calls                        | `StopReasonEndTurn`   |
| `incomplete` / `max_output_tokens`               | `StopReasonMaxTokens` |
| `incomplete` / `content_filter`                   | `StopReasonRefusal`   |
| anything else                                     | `StopReasonUnknown`   |

Tool use is inferred from the presence of `function_call` items on an otherwise-completed turn rather
than from a distinct status. Responses has no separate `stop_sequence` reason - a natural end and a
configured stop string both surface as `completed`, so both map to `StopReasonEndTurn`.

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

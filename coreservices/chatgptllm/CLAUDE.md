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

The provider ships no typed model catalog and **no shipped default alias table**: provider model IDs rotate every quarter, so any maintained list would go stale. Instead `resolveModel` (in `service.go`) resolves a capability tier (`fast`/`default`/`smart`) or a synthesized floating alias (`gpt-latest`/`gpt-pro-latest`/`gpt-mini-latest`/`gpt-nano-latest`) through a `svc.modelAliases` table that is populated lazily from the live models-list API on first use. The `gpt-*-latest` aliases stand in for the floating pointer OpenAI does not publish; they are namespaced with the `gpt-` prefix on purpose - a bare generic family word like `mini`/`nano` is **not** a global alias, since it would collide across providers under `"any"` resolution. Only the neutral tiers are global. A concrete model id passes through *without* the table: `isConcreteOpenAIModel` (a `gpt-` prefix followed by a version digit, or an `o1-`/`o3-`/`o4-` prefix) is returned verbatim, so a concrete `Turn` never depends on the models-list fetch and a newly released model works before it is listed. The digit test is what separates a concrete `gpt-5.5` from a synthesized `gpt-latest` alias (letter after the dash), which must go through the table. Anything unrecognized returns `""`. **Consequence of dropping defaults:** until a fetch succeeds, only concrete prefixed names resolve - a tier/family alias returns `""` (and `OnResolveProvider` answers `ok=false`) rather than a stale default. This is an accepted hard dependency: resolving an alias needs the same API and key as the chat call itself.

## Model Alias Refresh

The alias table is populated by `RefreshModels`, which fetches the OpenAI models-list API (`ModelsURL`, default `https://api.openai.com/v1/models`) and rebuilds `svc.modelAliases`. It runs three ways: an eager warm at startup (launched from `OnStartup` via `svc.Go`, non-blocking), a 6h ticker, and a lazy fetch on the first alias resolve. The lazy path is `ensureAliases`: a **retryable "once"** - concurrent resolves are serialized by `fetchMu` and a populated table short-circuits, but a *failed* fetch leaves the table empty so the next call retries. (A plain `sync.Once` is deliberately avoided: with no default fallback, a wedged first fetch would break alias resolution until the ticker.) The table is swapped under `aliasMu` (an `RWMutex`; `resolveModel` reads under `RLock`).

`buildChatgptAliases` (pure, unit-tested) builds the table from the live list: each tier and its `gpt-*-latest` alias points at the *latest* `gpt-5`-or-later model of that variant. "Latest" is the highest version, tie-broken by the newer `created` timestamp - version leads so a re-published older model never outranks a higher version. Mapping: base `gpt-5.x` -> `default` + `gpt-latest`, `-pro` -> `smart` + `gpt-pro-latest`, `-mini` -> `fast` + `gpt-mini-latest`, `-nano` -> `gpt-nano-latest`. The variant regex only accepts a single all-letters suffix ending the id, so `-preview`/`-beta` markers, dated snapshots (`gpt-5.5-2025-04-01`), and multi-segment names (`gpt-5.5-pro-preview`) never match - a "latest" pointer tracks stable, dateless flagships only. Pre-5 families (`gpt-4o`, `gpt-4.1`), o-series, and non-tier suffixes (`gpt-5-codex`, `gpt-5.5-chat`) are excluded; a variant absent from the list simply has no alias.

A missing key is a no-op; a fetch failure is returned by `RefreshModels` (never swallowed) and logged by the framework - its `svc.Go` warm-up and the 6h ticker both log a returned error - while staying non-fatal, so the last-known table keeps serving. A successful fetch doubles as early API-key validation: a `401`/`403` surfaces in the log shortly after startup rather than only on first use. The OpenAI list is sparse (`id`, `created`, `owned_by` only - no token limits or reasoning flag), which is why reasoning is inferred by name; see Reasoning Items and Replay.

`OnResolveProvider` is this provider's sink for the `llm.core` resolve event: it answers `ok = APIKey configured && resolveModel(model) != ""`, so `llm.core` selects this provider under an empty/`"any"` request only when ChatGPT holds a key and recognizes the model. See `coreservices/llm/CLAUDE.md` "Provider and Model Resolution".

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

**Whether to send `include` is inferred from the model name.** A non-reasoning model (e.g. `gpt-4o-mini`)
rejects `include=reasoning.encrypted_content` with a `400`, so the flag is gated on `isReasoningModel`
(`service.go`), which reads the name. The rule is stated in the **negative** so future models default
correctly: the o-series (`o1`/`o3`/`o4-...`) reason, and a `gpt-` model reasons *unless* its version is
below 5 (`gpt-4o`/`gpt-4.1` are chat) or its name contains `-chat` (the `gpt-5.x-chat`/`-chat-latest`
variants are non-reasoning even at version >= 5). An unversioned or future `gpt` name (`gpt-6`, ...)
therefore defaults to reasoning, because new OpenAI flagships are reasoning models - the safe default is to
assume reasoning and only carve out the known pre-5 and `-chat` non-reasoning families. The models-list API carries no reasoning flag
for OpenAI (unlike Anthropic's `capabilities` and Gemini's `thinking` bool), so this name heuristic is the
only upfront signal.

Inferring from the name rather than observing billed reasoning tokens is a deliberate reversal of the
earlier runtime-detection scheme (a per-replica `reasoningSeen` set warmed from
`usage.output_tokens_details.reasoning_tokens`). That scheme had a **first-encounter gap**: the very first
call to a model went out without `include`, so its reasoning items came back with no encrypted payload and
could not be replayed. Because the current models are effectively all reasoning models, the safer default
is to assume reasoning from the name and send `include` on the first turn, so replay works immediately with
no gap. An echoed reasoning item must still carry a `summary` array (empty is fine; omitting it is a
`400`), which the input converter always initializes non-nil.

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

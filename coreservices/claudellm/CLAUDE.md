## Design Rationale

This microservice implements the `Turn` endpoint for the Claude (Anthropic) LLM provider. It translates between the Microbus `[]llmapi.Item` conversation log and the Claude Messages API format. The flat item log is regrouped into Claude's two-level shape: a `system` message folds into `system`, an assistant text item and the `tool_call` items after it coalesce into one assistant message (text then `tool_use` blocks), and `tool_result` items coalesce into one user message (Anthropic requires all tool_results for a turn to share a user message). Reasoning items are dropped on input: extended thinking is not enabled, so Claude produces no thinking blocks to round-trip, and a Claude conversation never contains reasoning items to replay.

The provider ships no typed model catalog: provider model IDs rotate every quarter, so a maintained `Model*` const list would always be stale and removing entries would break downstream compilation. Instead `resolveModel` (in `service.go`) maps a capability tier (`fast`/`default`/`smart`) or a Claude family alias (`haiku`/`sonnet`/`opus`) to a current concrete model, passes through any `claude-` prefixed name as-is (so a newly released model works before it is listed), and returns `""` for anything else. `Turn` calls it at the top: a recognized alias/name resolves, an unrecognized string passes through unchanged so an explicit-provider call to a brand-new model still reaches the API. Entries are pinned to the current release because the Anthropic API exposes no evergreen pointer - dateless IDs like `claude-opus-4-8` are pinned snapshots, and the `opus`/`sonnet` aliases that auto-migrate are a Claude Code CLI convenience, not API model IDs. The alias table is held in `svc.modelAliases` (a small runtime map, not a hand-maintained public catalog); the Phase 2 live `/v1/models` lookup repopulates it.

`OnResolveProvider` is this provider's sink for the `llm.core` resolve event: it answers `ok = APIKey configured && resolveModel(model) != ""`, so `llm.core` selects this provider under an empty/`"any"` request only when Claude holds a key and recognizes the model. See `coreservices/llm/CLAUDE.md` "Provider and Model Resolution".

`Turn` populates `llmapi.Usage` with input/output token counts, cache read/write tokens (from Claude's `cache_read_input_tokens` and `cache_creation_input_tokens` fields), the model identifier echoed by the response, and `Turns: 1`. `llm.core` aggregates these across turns and emits the `LLMTokens` metric.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## Stop-Reason Mapping

`Turn` returns a normalized `stopReason` (constants in `llmapi/stopreason.go`). `mapStopReason` in
`service.go` translates Anthropic's `stop_reason` field one-to-one:

| Anthropic            | Normalized                  |
|----------------------|-----------------------------|
| `end_turn`           | `StopReasonEndTurn`         |
| `tool_use`           | `StopReasonToolUse`         |
| `max_tokens`         | `StopReasonMaxTokens`       |
| `stop_sequence`      | `StopReasonStopSequence`    |
| `refusal`            | `StopReasonRefusal`         |
| `pause_turn`         | `StopReasonPauseTurn`       |
| `""` / anything else | `StopReasonUnknown`         |

`llm.core` interprets these — see `coreservices/llm/CLAUDE.md` "Stop-Reason Branching". An empty or
unrecognized value reaches `llm.core` as `StopReasonUnknown` and surfaces as a `502` rather than as
a silent completion, so a future Anthropic stop reason that we haven't taught the map about fails
loudly until someone adds the case.

## Prompt Caching

Anthropic's prompt cache is opt-in via `cache_control: {"type": "ephemeral"}` markers placed on specific request blocks. The cache is **prefix-based and byte-exact**: a marker tells Anthropic to remember everything from the start of the request up to and including that block, and a future request hits the cache only if the cached prefix matches byte-for-byte.

### What this provider does by default

Two breakpoints are set unconditionally on every request, using 2 of the 4 markers Anthropic allows:

1. **Last tool** if tools are present, else **last system block** if system is present - caches the stable preamble.
2. **Last content block of the last message** - caches the conversation history as a prefix that the next turn (which appends new messages) can reuse.

These are set unconditionally because Anthropic silently declines to cache content below the per-model size threshold (~1024 tokens for Sonnet/Opus, ~2048 for Haiku). For small requests the markers are a no-op; for large ones they're a free win. There's no need to gate by an estimated token count - server-side handles the threshold.

### Why mark the *last* block, not the second-to-last

In multi-turn conversations, marking the last message of turn N caches the longest stable prefix that turn N+1 will reuse:

```
Turn 1 sends [user1]                    → marks user1 → caches "system+tools+user1"
Turn 2 sends [..., assistant1, user2]   → reads "system+tools+user1" prefix
                                          marks user2  → caches the new prefix for turn 3
```

If you marked the second-to-last block instead, every subsequent turn would only hit a shorter prefix and pay full price on the most-recently-stable content. Marking the last block lets each turn's cache write set up the next turn's deeper cache read.

### What the request shape looks like (cache-relevant fields)

`apitypes.go` defines the wire format. Three things matter for caching:

- **`System` is `[]claudeContentBlock`** (not `string`). Anthropic accepts both forms; we always emit the array form so a `cache_control` can be attached uniformly.
- **`Message.Content` is `[]claudeContentBlock`** (not `json.RawMessage`). Same reason - uniform shape, consistent place to attach `cache_control`.
- **`claudeContentBlock` and `claudeTool` both carry `*claudeCacheControl`** with `omitzero`, so the marker is included only when non-nil.

### Byte-exact serialization

Cache hits depend on the request bytes being identical across calls. Three things keep them deterministic:

1. **Go struct field order is declaration order** (`encoding/json` guarantees this), so as long as the structs in `apitypes.go` aren't reordered, the JSON shape is stable.
2. **Tools are sorted by name** in `service.go` before being converted to `claudeTool`. This insulates the cache key from caller-side ordering variance - a downstream caller building tool URLs from a Go map iteration would otherwise produce different orderings between calls and defeat the cache.
3. **`json.RawMessage` for `InputSchema` and `Input`** is pass-through, no re-marshaling. The schema bytes come from the OpenAPI document fetched by `llm.core`; see `openapi/CLAUDE.md` for the schema-stability guarantees on that side.

### What is NOT exposed in `TurnOptions`

`TurnOptions` does not expose cache-marker placement controls (e.g. an `EnableCache bool` or per-section `CacheBreakpoints []string`). Reasons:

- Marking the last tool / last message is the canonical pattern that wins for >95% of workloads.
- Provider-specific cache mechanics (Anthropic's explicit markers vs OpenAI's automatic prefix vs Gemini's implicit caching) don't fit a shared abstraction cleanly.
- Adding the option speculatively is YAGNI - when a real use case emerges (deterministic-test cache bypass, very long history with a custom history breakpoint, per-tenant cache isolation), the option's shape will be informed by that workload rather than guessed.

If a caller explicitly needs to bypass caching, the cleanest workaround today is to set `cache_control` to nil at the call site by patching `claudellm/service.go` - we'll add the option when there's actual demand.

## Error Classification

The provider-to-ChatLoop contract is a single attribute, **`retryAfter`**: an error that carries a `retryAfter`
duration is retryable (and that is the wait); an error without one is permanent. ChatLoop holds no policy and never
inspects the status code - the provider, which alone sees the status, body, and headers, makes the decision and
expresses it as `retryAfter`. This mirrors the dwarf/foreman split where the task expresses backoff via
`flow.Sleep`+`flow.Retry` and the engine just executes it. The upstream status code is left **authentic** (never
remapped) - it is for HTTP semantics and observability, not the retry decision.

On a non-OK response the provider parses Anthropic's `{error:{type,message}}` body and constitutes a `TracedError`
via `errors.New`: the text is the body's `error.message`; the attributes are the authentic upstream status code,
`type` (body `error.type`), `Request-Id` and `Traceresponse` headers, and the **entire `Anthropic-*` header family**
(`HeaderAttrs`) - the organization id (`Anthropic-Organization-Id`, the per-org account identifier, returned on every
response) plus every `Anthropic-Ratelimit-*` dimension. Capturing the full header set lets a caller of `Chat` make
its own retry decision from the raw limit/remaining/reset values, not just the boolean `retryAfter`.

`retryAfter` is set from the `Retry-After` header (delta-seconds). **Anthropic sends `Retry-After` exactly on the
cases it wants retried** (429 `rate_limit_error`, 529 `overloaded_error`), and omits it on permanent failures - so
keying `retryAfter` off that header needs no per-type allow-list and naturally classifies the permanent cases
(billing `402`, oversized `413`, context-window `400`, auth `401/403`) as non-retryable.

### Example: oversized request (live capture)

Sending a prompt larger than the model's 200k-token context window. A clean permanent `400` with no `Retry-After`, so
no `retryAfter` attribute is attached and ChatLoop will not retry.

```
status 400   (authentic, not remapped)
headers:
  Anthropic-Organization-Id: 17a4db89-fded-4b3d-a5e9-c1f8ec02d31a
  Request-Id:                req_011CcFeQK8sU5EubttGUs31K
  Traceresponse:             00-bd61476a8eb29f552e60cf107e235d45-042bf9271da17926-01
  X-Should-Retry:            false      (Anthropic's own retryability hint; no Retry-After present)
  Content-Type:              application/json
body:
  {"type":"error","error":{"type":"invalid_request_error",
   "message":"prompt is too long: 200020 tokens > 200000 maximum"},
   "request_id":"req_011CcFeQK8sU5EubttGUs31K"}
```

The limits are stated inline in the message (`200020 tokens > 200000 maximum`). A retryable error (e.g. a `429`)
would instead carry a `Retry-After` header, which becomes the `retryAfter` attribute.

## Rate-Limit Preemption

The provider keeps an in-memory `map[model]blockedUntil` (mutex-guarded). On a `retryAfter`-bearing throttle it
records `blockedUntil[model] = now + retryAfter`, and at the top of `Turn` it preempts a call to a still-blocked model
with a synthetic `429` (carrying the remaining wait as `retryAfter`) *without* dialing Anthropic. This stops every
in-flight caller from each eating its own real `429` before backing off.

The gate lives in the provider, not llm.core, because the provider holds the API key - it unambiguously *is* one
account and can gate before dialing, and a gate in `Turn` covers every caller (`Chat`, `CallLLM`, and direct
`ForHost`), not just llm.core-routed traffic. It is keyed by **model** because Anthropic's limits are per-model; an
account-wide gate would over-block sibling models. Only `retryAfter`-bearing errors arm it (poison/permanent never
do). Per-replica, no cross-replica gossip.

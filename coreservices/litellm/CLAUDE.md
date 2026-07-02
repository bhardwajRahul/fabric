## Design Rationale

This microservice implements the `Turn` endpoint for a [LiteLLM](https://docs.litellm.ai) proxy. LiteLLM
exposes the OpenAI Responses wire format (`/v1/responses`) regardless of the backend model it routes to - it
bridges `/chat/completions` internally for providers that lack native Responses support - so this provider is
intentionally a near-clone of `chatgptllm`: the request/response structs in `apitypes.go` keep their `openai*`
names because the bytes on the wire genuinely are OpenAI's schema, not a LiteLLM-specific one. Keeping the names
identical documents that fact and keeps the two providers easy to diff.

Like `chatgptllm`, `Turn` speaks the `[]llmapi.Item` model and maps it onto Responses input/output items,
including reasoning capture and replay (`store` always false; `include:reasoning.encrypted_content` sent
only once a model is observed to reason). Reasoning support is detected at **runtime** - a per-replica
`reasoningSeen` set populated when a response bills reasoning tokens - not from a model-name list. This
runtime detection matters especially behind LiteLLM, where the model string is operator-defined and can't
be pattern-matched. Its `Turn` returns the same `(items, stopReason, usage)` shape `llm.core` dispatches against
(`llm.core` deserializes the provider response into `llmapi.TurnOut`, including `stopReason`; a missing
`stopReason` would read as `StopReasonUnknown` and 502 every turn). The only intended differences from
`chatgptllm` are the LiteLLM-specific `num_retries` field, the localhost default, and the absence of the
empty-response debug diagnostic.

`mapStopReason` derives the normalized reason from the Responses `status`, `incomplete_details.reason`, and whether
any `function_call` items were emitted (`completed` + tool calls -> `StopReasonToolUse`; `completed` alone ->
`StopReasonEndTurn`; `incomplete` / `max_output_tokens` -> `StopReasonMaxTokens`; `incomplete` / `content_filter` ->
`StopReasonRefusal`; anything else -> `StopReasonUnknown`). It is identical to the `chatgptllm` mapping.

`llm.core` does not maintain a provider registry - it routes by hostname (`ForHost(provider)`). This provider
is reachable as `lite.llm.core`; nothing in `llm.core` needed to change to add it.

### Why the default `ResponsesURL` is `http://localhost:4000/v1/responses`

The other three providers point at a fixed vendor URL (`api.openai.com`, `api.anthropic.com`,
`generativelanguage.googleapis.com`). LiteLLM has no canonical public endpoint - it is a self-hosted proxy.
`http://localhost:4000` is LiteLLM's own documented default proxy address, so it is the only sensible default;
operators running the proxy elsewhere set `ResponsesURL` outright (the whole URL, not a base + suffix, per the
full-URL config convention shared by all four providers).

### No typed model constants; the proxy's model_list is the alias table

No provider ships a typed model catalog anymore (model IDs rotate too fast to maintain, and removing stale consts
breaks downstream). For LiteLLM the point is sharper still: the valid model strings are whatever the operator put in
the proxy's `model_list`, so even a current catalog here would be wrong for most deployments. `model` is a
passthrough string; `Turn` sends it straight to the proxy, which is the authority on its own `model_list`.

Unlike the other three providers, LiteLLM has no fixed vendor families or tiers to synthesize aliases from. Instead
**its `model_list` is the alias table**: each entry's `model_name` is an operator-chosen public label (arbitrary - a
real id like `gpt-4o`, or a friendly name like `smart`) that the proxy maps to a real backend model. So this provider
fetches the proxy's OpenAI-compatible models-list API (`ModelsURL`, default `http://localhost:4000/v1/models`) and
keeps the returned `model_name` set. `resolveModel` is a pure membership test - a held `model_name` resolves to itself,
anything else to `""` - and `OnResolveProvider` answers `ok=true` for any held name. **The portable tiers work with no
extra Microbus config when the operator names `model_list` entries `fast`/`default`/`smart`**; there is no separate
operator alias-map because LiteLLM's own naming already is one.

The set is populated by the same lazy-fetch/ticker machinery as the other providers: an eager `OnStartup` warm via
`svc.Go`, a 6h `RefreshModels` ticker, and a lazy fetch on first resolve (`ensureAliases`, a retryable "once" guarded
by `fetchMu`). It is gated on a configured `APIKey` (the proxy's virtual/master key) - a keyless dev proxy is reached
by pinning `provider="lite.llm.core"` explicitly, since `Turn` passes the model through without consulting the set, so
the explicit-provider path never depends on the fetch. Reasoning detection stays **runtime** (`reasoningSeen`): the
`model_name` is operator-defined and cannot be name-inferred the way `chatgptllm` infers from `gpt-`/`o-` names.

### Token usage mapping

`Turn` maps the Responses `usage` block the same way `chatgptllm` does: `input_tokens` minus
`input_tokens_details.cached_tokens` to `InputTokens`, the cached portion to `CacheReadTokens`,
`output_tokens` to `OutputTokens`, `output_tokens_details.reasoning_tokens` to `ThinkingTokens`, no write count.
LiteLLM normalizes most backends into this shape, but for non-OpenAI backends some fields (notably cached- and
reasoning-token details) may be absent and surface as zero. This is a fidelity limit of the upstream proxy, not a
bug here.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform
interface across all provider microservices.

### Internal retries are disabled (`num_retries: 0`)

The request always sends `num_retries: 0` (the field has no `omitzero`, so the zero is on the wire). LiteLLM's own
retry decision, `litellm._should_retry(status_code)`, mirrors the OpenAI SDK: it retries on the `x-should-retry: true`
header, `408`, `409`, `429`, and `>= 500`, and declines on `x-should-retry: false`. It is **status-code-only** (plus
that header) - it never inspects the body. So for upstreams that bury a permanent failure inside a `429` (OpenAI
`insufficient_quota` / request-too-large, Gemini `RESOURCE_EXHAUSTED`), LiteLLM would retry the poison case until its
count ran out, because only the body distinguishes it and LiteLLM doesn't read the body. (Anthropic behind LiteLLM is
the exception: it sends `x-should-retry: false`, which LiteLLM honors.) Retries must live in exactly one place -
`CallLLM`, driven by the `retryAfter` contract - so we force LiteLLM not to compound them. We set `0` explicitly
rather than rely on a default because the proxy's router can be configured with retries, so the per-request `0` is the
safe override.

### Rate-limit handling: accept the poison risk (no per-upstream heuristic)

LiteLLM fronts many upstream accounts behind one key/hostname, so the body-parsing poison heuristic the other three
providers use is upstream-specific and not worth replicating here. Instead this provider treats **every `429` as
retryable**: it attaches a `retryAfter` (from `Retry-After`, LiteLLM's forwarded `llm_provider-retry-after`, the
OpenAI-style `X-Ratelimit-Reset-*`, or a 60s default - see `litellmRetryAfter`) and arms the same per-model
preemption gate as the other providers. A genuine throttle (the common case) is therefore retried; a poison request
(rare) is retried too, but **bounded** by `CallLLM`'s finite retry cap and surfaced via metrics, so an operator sees
it and raises quota or shrinks the prompt. This is only safe because that cap is finite - it must never regress to an
unbounded retry, or a poison request would loop forever.

The gate is keyed by the `model` string (the value sent to the proxy). Non-429 errors carry no `retryAfter` and are
permanent. Everything else (raw `body` truncated to 16KB + filtered `headers`) is attached for caller introspection,
identical to the other providers.

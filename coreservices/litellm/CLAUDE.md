## Design Rationale

This microservice implements the `Turn` endpoint for a [LiteLLM](https://docs.litellm.ai) proxy. LiteLLM
exposes the OpenAI Chat Completions wire format regardless of the backend model it routes to, so this
provider is intentionally a near-clone of `chatgptllm`: the request/response structs in `apitypes.go` keep
their `openai*` names because the bytes on the wire genuinely are OpenAI's schema, not a LiteLLM-specific one.
Keeping the names identical documents that fact and keeps the two providers easy to diff.

`llm.core` does not maintain a provider registry - it routes by hostname (`ForHost(provider)`). This provider
is reachable as `lite.llm.core`; nothing in `llm.core` needed to change to add it.

### Why the default `CompletionURL` is `http://localhost:4000/v1/chat/completions`

The other three providers point at a fixed vendor URL (`api.openai.com`, `api.anthropic.com`,
`generativelanguage.googleapis.com`). LiteLLM has no canonical public endpoint - it is a self-hosted proxy.
`http://localhost:4000` is LiteLLM's own documented default proxy address, so it is the only sensible default;
operators running the proxy elsewhere set `CompletionURL` outright (the whole URL, not a base + suffix, per the
`CompletionURL` convention shared by all four providers).

### No typed model constants

`chatgptllm` ships `ModelGPT4o` etc. in its api package. This provider deliberately does not: the valid model
strings are whatever the operator put in the LiteLLM proxy's `model_list`, so a hard-coded catalog here would be
wrong for most deployments. `model` is a passthrough string; callers pass whatever the proxy is configured to
accept.

### Token usage mapping

`Turn` maps the OpenAI `usage` block the same way `chatgptllm` does: `prompt_tokens` minus
`prompt_tokens_details.cached_tokens` to `InputTokens`, the cached portion to `CacheReadTokens`,
`completion_tokens` to `OutputTokens`, no write count. LiteLLM normalizes most backends into this shape, but for
non-OpenAI backends some fields (notably cached-token details) may be absent and surface as zero. This is a
fidelity limit of the upstream proxy, not a bug here.

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

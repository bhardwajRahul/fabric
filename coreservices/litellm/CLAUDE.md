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

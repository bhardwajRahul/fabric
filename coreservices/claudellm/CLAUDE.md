## Design Rationale

This microservice implements the `Turn` endpoint for the Claude (Anthropic) LLM provider. It translates between the Microbus LLM message format and the Claude Messages API format, handling system messages, tool_use/tool_result content blocks, and error parsing.

The `model` argument is required per call (no `Model` config); use the typed constants in `claudellmapi` (e.g. `claudellmapi.ModelHaiku45`) for compile-time checking.

`Turn` populates `llmapi.Usage` with input/output token counts, cache read/write tokens (from Claude's `cache_read_input_tokens` and `cache_creation_input_tokens` fields), the model identifier echoed by the response, and `Turns: 1`. `llm.core` aggregates these across turns and emits the `LLMTokens` metric.

Types (`Message`, `Tool`, `ToolCall`, `Usage`, `TurnOptions`) are imported from `llmapi` to ensure a uniform interface across all provider microservices.

## Prompt Caching

Anthropic's prompt cache is opt-in via `cache_control: {"type": "ephemeral"}` markers placed on specific request blocks. The cache is **prefix-based and byte-exact**: a marker tells Anthropic to remember everything from the start of the request up to and including that block, and a future request hits the cache only if the cached prefix matches byte-for-byte.

### What this provider does by default

Two breakpoints are set unconditionally on every request, using 2 of the 4 markers Anthropic allows:

1. **Last tool** if tools are present, else **last system block** if system is present — caches the stable preamble.
2. **Last content block of the last message** — caches the conversation history as a prefix that the next turn (which appends new messages) can reuse.

These are set unconditionally because Anthropic silently declines to cache content below the per-model size threshold (~1024 tokens for Sonnet/Opus, ~2048 for Haiku). For small requests the markers are a no-op; for large ones they're a free win. There's no need to gate by an estimated token count — server-side handles the threshold.

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
- **`Message.Content` is `[]claudeContentBlock`** (not `json.RawMessage`). Same reason — uniform shape, consistent place to attach `cache_control`.
- **`claudeContentBlock` and `claudeTool` both carry `*claudeCacheControl`** with `omitzero`, so the marker is included only when non-nil.

### Byte-exact serialization

Cache hits depend on the request bytes being identical across calls. Three things keep them deterministic:

1. **Go struct field order is declaration order** (`encoding/json` guarantees this), so as long as the structs in `apitypes.go` aren't reordered, the JSON shape is stable.
2. **Tools are sorted by name** in `service.go` before being converted to `claudeTool`. This insulates the cache key from caller-side ordering variance — a downstream caller building tool URLs from a Go map iteration would otherwise produce different orderings between calls and defeat the cache.
3. **`json.RawMessage` for `InputSchema` and `Input`** is pass-through, no re-marshaling. The schema bytes come from the OpenAPI document fetched by `llm.core`; see `openapi/CLAUDE.md` for the schema-stability guarantees on that side.

### What is NOT exposed in `TurnOptions`

`TurnOptions` does not expose cache-marker placement controls (e.g. an `EnableCache bool` or per-section `CacheBreakpoints []string`). Reasons:

- Marking the last tool / last message is the canonical pattern that wins for >95% of workloads.
- Provider-specific cache mechanics (Anthropic's explicit markers vs OpenAI's automatic prefix vs Gemini's implicit caching) don't fit a shared abstraction cleanly.
- Adding the option speculatively is YAGNI — when a real use case emerges (deterministic-test cache bypass, very long history with a custom history breakpoint, per-tenant cache isolation), the option's shape will be informed by that workload rather than guessed.

If a caller explicitly needs to bypass caching, the cleanest workaround today is to set `cache_control` to nil at the call site by patching `claudellm/service.go` — we'll add the option when there's actual demand.

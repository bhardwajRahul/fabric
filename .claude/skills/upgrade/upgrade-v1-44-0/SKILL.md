---
name: upgrade-v1-44-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.43.x to v1.44.0. Breaking changes to the LLM subsystem's public surface. (1) Provider endpoint configs are renamed: claude.llm.core CompletionURL -> MessagesURL, gemini.llm.core CompletionURL -> ModelsURL, chatgpt.llm.core and litellm CompletionURL -> ResponsesURL (config keys only; default URLs unchanged). (2) The per-provider model constants (claudellmapi.ModelHaiku45, chatgptllmapi.ModelGPT4o, geminillmapi.ModelGemini20Flash, ...) are removed - the model argument is now a passthrough string or an llmapi tier alias. (3) llmapi.Usage.ThinkingTokens is renamed ReasoningTokens. (4) The conversation model changes from a flat []llmapi.Message to an ordered []llmapi.Item: llmapi.AppendItems is removed in favor of append(items, x.AsItem()), Chat/ChatLoop take and return []Item, and Turn's return shape changes from (content, toolCalls, stopReason, usage) to (items, stopReason, usage). The config renames, const removals, and ThinkingTokens rename are pure-shell sed/perl edits; the []Item migration is a grep-guided manual rewrite because the correct new shape depends on how each call site used the conversation. No genupgrade tool is involved. All breaks are loud compile errors (or, for config keys, an unrecognized-property error) surfaced by the orchestrator's single go vet/test pass.
---

## What changed

v1.44.0 finishes the LLM subsystem's move to a provider-portable, ordered-item conversation model. Four breaking
changes touch the downstream-facing surface (`llmapi` and the provider `*api` config keys):

- **Provider endpoint configs renamed** (config keys only; the default URLs and wire formats are unchanged):
  | Provider hostname | Old config key | New config key |
  |---|---|---|
  | `claude.llm.core` | `CompletionURL` | `MessagesURL` |
  | `gemini.llm.core` | `CompletionURL` | `ModelsURL` |
  | `chatgpt.llm.core` | `CompletionURL` | `ResponsesURL` |
  | `lite.llm.core` | `CompletionURL` | `ResponsesURL` |
  Only a project that *overrode* one of these in `config.yaml`/`config.local.yaml` is affected; a project on the
  defaults has nothing to rename.
- **Per-provider model constants removed.** `claudellmapi.ModelHaiku45`, `chatgptllmapi.ModelGPT4o`,
  `geminillmapi.ModelGemini20Flash`, and their siblings are gone. Provider model IDs rotate every quarter, so the
  catalog is no longer a hand-maintained const list; the `model` argument is now a passthrough string (e.g.
  `"claude-haiku-4-5"`) or an `llmapi` tier alias (`llmapi.ModelDefault`/`ModelFast`/`ModelSmart`).
- **`llmapi.Usage.ThinkingTokens` renamed `ReasoningTokens`.** "Reasoning" is now the neutral-layer term across the
  LLM API.
- **The conversation is an ordered `[]llmapi.Item`, not a flat `[]llmapi.Message`.** An `Item` is a discriminated
  union (`message`, `tool_call`, `tool_result`, `reasoning`). `llmapi.AppendItems(items, x)` is removed - append the
  item form directly, `append(items, x.AsItem())`. `Chat`/`ChatLoop` now take `items []Item` and return
  `itemsOut []Item`; `Turn`'s return changes from `(content string, toolCalls []ToolCall, stopReason string, usage
  Usage, err error)` to `(itemsOut []Item, stopReason string, usage Usage, err error)`, so a caller reads the
  assistant text and tool calls out of the returned items instead of separate return values.

## Workflow

```
Upgrade a Microbus project to v1.44.0:
- [ ] Step 1: Rename provider endpoint configs (mechanical)
- [ ] Step 2: Replace removed model constants (mechanical)
- [ ] Step 3: Rename Usage.ThinkingTokens -> ReasoningTokens (mechanical)
- [ ] Step 4: Migrate the []Message conversation to []Item (grep-guided)
```

Regeneration and verification are **not** part of this skill - the `upgrade-microbus` orchestrator runs
`genservice` and `go mod tidy && go vet ./... && go test ./...` once, after every numbered skill has applied its
source transformation. The tree will not compile between steps; that is expected. The final `go vet` pass surfaces
any call site the grep-guided step missed.

#### Step 1: Rename Provider Endpoint Configs (Mechanical)

These are configuration *keys*, so they live in `config.yaml`, `config.local.yaml`, and any `env*.yaml` - not in Go
source. Only an overridden value needs renaming; the default is unchanged. Find every occurrence and rename it by
the provider block it sits under (the same old key maps to a different new key per provider, so this cannot be a
single blind replace):

```bash
grep -rn --include='*.yaml' 'CompletionURL' . 2>/dev/null
```

For each hit, rename the key per the table, leaving the value untouched:

- under `claude.llm.core:` -> `MessagesURL`
- under `gemini.llm.core:` -> `ModelsURL`
- under `chatgpt.llm.core:` or `lite.llm.core:` -> `ResponsesURL`

A `CompletionURL` under `all:` (applied to every service) must be split into the per-provider keys above, since no
single key covers all four providers anymore. If a hit is unrelated to an LLM provider, leave it.

#### Step 2: Replace Removed Model Constants (Mechanical)

The per-provider `Model*` constants are removed. Replace each reference with the literal model-ID string it held,
which preserves the exact model the code requested:

```bash
grep -rn --include='*.go' --exclude-dir=vendor -E '(claudellm|chatgptllm|geminillm)api\.Model' .
```

Apply the mapping (the value each removed const held):

```bash
for f in $(grep -rl --include='*.go' --exclude-dir=vendor -E '(claudellm|chatgptllm|geminillm)api\.Model' .); do
  perl -i -pe '
    s/\bclaudellmapi\.ModelOpus47\b/"claude-opus-4-7"/g;
    s/\bclaudellmapi\.ModelSonnet46\b/"claude-sonnet-4-6"/g;
    s/\bclaudellmapi\.ModelHaiku45\b/"claude-haiku-4-5"/g;
    s/\bchatgptllmapi\.ModelGPT4oMini\b/"gpt-4o-mini"/g;
    s/\bchatgptllmapi\.ModelGPT4o\b/"gpt-4o"/g;
    s/\bchatgptllmapi\.ModelGPT4Turbo\b/"gpt-4-turbo"/g;
    s/\bchatgptllmapi\.ModelGPT4\b/"gpt-4"/g;
    s/\bgeminillmapi\.ModelGemini20FlashExp\b/"gemini-2.0-flash-exp"/g;
    s/\bgeminillmapi\.ModelGemini20Flash\b/"gemini-2.0-flash"/g;
    s/\bgeminillmapi\.ModelGemini15Pro\b/"gemini-1.5-pro"/g;
    s/\bgeminillmapi\.ModelGemini15Flash\b/"gemini-1.5-flash"/g;
  ' "$f"
done
```

The `ModelGPT4o` line must run before `ModelGPT4` (and `ModelGemini20Flash` before its `Exp` sibling is handled by
ordering the longer name first), which the block above already does. After the replace, a provider `*api` package
that was imported *only* for a model const is now an unused import; the orchestrator's `go vet` flags it and it is
removed. As a follow-up (not required for the upgrade to compile), a pinned literal can be replaced with a portable
tier alias - `llmapi.ModelDefault`/`ModelFast`/`ModelSmart` - if the call does not need that exact model.

#### Step 3: Rename `Usage.ThinkingTokens` -> `ReasoningTokens` (Mechanical)

A field rename on `llmapi.Usage`:

```bash
grep -rl --include='*.go' --exclude-dir=vendor '\.ThinkingTokens\b' . \
  | while read -r f; do
      perl -i -pe 's/\.ThinkingTokens\b/.ReasoningTokens/g' "$f"
    done
```

Restrict the replace to the selector form `.ThinkingTokens` (as above) so it hits the `Usage` field access and not
an unrelated identifier.

#### Step 4: Migrate the `[]Message` Conversation to `[]Item` (Grep-Guided)

This has no safe mechanical rewrite: the correct new shape depends on how each call site built or consumed the
conversation. Find the affected sites and migrate each; when the right shape is unclear, ask the user rather than
guessing. The orchestrator's `go vet` also reports every one as a type error.

```bash
grep -rn --include='*.go' --exclude-dir=vendor -E 'llmapi\.(AppendItems|Message\b|NewMessage)|\]llmapi\.Message|\.Chat\(|\.Turn\(|\.ChatLoop\(' .
```

Common shapes and their new form:

- **Building a conversation.** `llmapi.AppendItems` is removed; append the item form:
  ```go
  // before
  items := llmapi.AppendItems(nil, llmapi.NewMessage("user", q))
  items = llmapi.AppendItems(items, toolResult)
  // after
  items := []llmapi.Item{llmapi.NewMessage("user", q).AsItem()}
  items = append(items, toolResult.AsItem())
  ```
  `NewMessage`, `ToolCall`, `ToolResult`, and reasoning values each expose `.AsItem()`. A `[]llmapi.Message{...}`
  literal becomes `[]llmapi.Item{ ...AsItem() }`.

- **`Chat` / `ChatLoop`.** Both now take `items []Item` and return `itemsOut []Item` (same positions the old
  `messages`/`messagesOut` occupied). Update the argument and the result variable's type; a caller that ranged over
  returned `messages` now ranges over `[]Item` and must branch on `item.Type()` (e.g. take the last
  `ItemMessage` with `Role == "assistant"` for the final answer, as the weather example's `finalAnswer` helper
  does).

- **`Turn`.** The return shape changed from `(content string, toolCalls []ToolCall, stopReason string, usage,
  err)` to `(itemsOut []Item, stopReason string, usage, err)`. A caller that used `content`/`toolCalls` directly
  now extracts them from `itemsOut` (iterate for the `ItemMessage` content and the `ItemToolCall` items). Most
  projects call `Chat`, not `Turn` directly; migrate `Turn` sites only if present.

- **Workflow state keys around `ChatLoop`.** If a task reads `ChatLoop`'s conversation out of flow state by key,
  the key is now `items` (the `messages`/`messagesOut` keys are gone). Update `flow.Get*("messages")` /
  `flow.Get*("messagesOut")` reads to `items`. A workflow that consumes `ChatLoop` only through the typed
  `NewSubgraph(flow).ChatLoop(...)` return needs no state-key change.

After Step 4 the project still does not compile (generated boilerplate is stale and the dependency has not been
re-resolved); that is expected. The orchestrator's final step regenerates each microservice's boilerplate, runs
`go mod tidy`, and verifies with `go vet ./...` and `go test ./...`.

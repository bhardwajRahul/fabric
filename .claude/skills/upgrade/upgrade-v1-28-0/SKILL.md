---
name: upgrade-v1-28-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project to v1.28.0. Renames each microservice's `AGENTS.md` to `CLAUDE.md`, replacing the old redirect-only `CLAUDE.md`.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.28.0:
- [ ] Step 1: Find all CLAUDE.md files to process
- [ ] Step 2: Merge AGENTS.md into CLAUDE.md
- [ ] Step 3: Strip boilerplate and seed empty files
- [ ] Step 4: Replace AGENTS.md references with CLAUDE.md
- [ ] Step 5: Update manifests
- [ ] Step 6: Migrate llm.core / provider Turn callers
- [ ] Step 7: Audit env.yaml / env.Push semantics flip
- [ ] Step 8: Validate subscription HTTP methods
- [ ] Step 9: Heads-up audits (time-budget header, PROD port blocking)
```

#### Step 1: Find All `CLAUDE.md` Files to Process

Find every `CLAUDE.md` file in the project. Exclude any files located under `.claude/skills/` — those are skill templates maintained separately.

For each `CLAUDE.md`, check whether an `AGENTS.md` exists in the same directory. Build two lists:

- **To merge**: directories that contain both `CLAUDE.md` and `AGENTS.md`
- **Skip**: directories that contain `CLAUDE.md` but no `AGENTS.md` — leave these untouched

#### Step 2: Merge `AGENTS.md` Into `CLAUDE.md`

For each directory in the **to merge** list:

1. Read the contents of `CLAUDE.md`. Strip any lines that consist solely of the redirect instruction (i.e. lines matching `**CRITICAL**: Read \`AGENTS.md\` immediately.`), as well as any leading or trailing blank lines left behind. Call what remains the **extra content** (may be empty).

2. If the extra content is non-empty, append it to `AGENTS.md`, separated from the existing content by a single blank line.

3. Delete `CLAUDE.md`.

4. Rename `AGENTS.md` to `CLAUDE.md`.

#### Step 3: Strip Boilerplate and Seed Empty Files

In every `CLAUDE.md` produced by Step 2, remove the following boilerplate lines if present, along with any blank lines they leave behind:

- `**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in \`.claude/rules/microbus.md\`.`
- `**CRITICAL**: The instructions and guidelines in this \`CLAUDE.md\` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.`

After stripping, if a `CLAUDE.md` is empty or contains only whitespace, seed it with the hostname as an H1 heading, read from the `manifest.yaml` in the same directory:

```md
# my.service.hostname
```

#### Step 4: Replace `AGENTS.md` References With `CLAUDE.md`

In every file produced or modified by Step 2 (the new `CLAUDE.md` files), replace all occurrences of the filename `AGENTS.md` with `CLAUDE.md`. This covers inline references such as links, instructions, or commentary that still point to the old filename.

Also apply the same replacement to any other files in the project that may reference `AGENTS.md` by name, excluding files under `.claude/skills/` (skill templates are maintained separately). Use a path exclusion that works regardless of whether the tool returns paths with or without a leading `./`, e.g. filter on `\.claude/skills/` (escaped dot, no leading slash), which matches `.claude/skills/` anywhere in the path.

#### Step 5: Update Manifests

Update the `frameworkVersion` in all `manifest.yaml` files in the project to `1.28.0`. Update each manifest's `modifiedAt` to the current UTC timestamp in RFC 3339 format.

#### Step 6: Migrate `llm.core` / Provider `Turn` Callers

v1.28.0 made breaking changes to the LLM service API. The `Chat` and `Turn` signatures both changed, and the `ProviderHostname` config (on `llm.core`) and `Model` config (on each provider microservice) were removed.

**a. Find all callers of `llmapi.Chat` and update them.**

Old signature:
```go
messagesOut, err := llmapi.NewClient(svc).Chat(ctx, messages, tools)
```

New signature:
```go
// TODO: pick a provider hostname (e.g. claudellmapi.Hostname) and a model identifier
// (e.g. claudellmapi.ModelHaiku45). The previous ProviderHostname / Model configs were
// removed in v1.28.0; provider and model are now per-call arguments.
messagesOut, usage, err := llmapi.NewClient(svc).Chat(ctx, "", "", messages, toolURLs, nil)
```

The `provider` and `model` arguments are required and the new `Chat` returns `400 Bad Request` if either is empty. **The skill cannot synthesize correct values for these from the project alone** — they used to come from operator config that has been removed. Pass empty strings `""` for both and add a `// TODO:` comment immediately above the call explaining what the developer must fill in. Empty strings (rather than placeholder identifiers like `"TODO_LLM_PROVIDER"`) are preferred because they fail loudly at runtime with the new validation, and they don't risk slipping past `go vet` into a deploy. After all edits, emit a migration warning to the user listing every call site touched.

If the call site already imports a specific provider's `*api` package, suggest the matching typed model constants:
- `claudellmapi.Hostname` + `claudellmapi.ModelHaiku45` / `ModelSonnet46` / `ModelOpus47`
- `chatgptllmapi.Hostname` + `chatgptllmapi.ModelGPT4o` / `ModelGPT4oMini` / `ModelGPT4`
- `geminillmapi.Hostname` + `geminillmapi.ModelGemini20Flash` / `ModelGemini15Pro`

The result is now a 3-tuple `(messagesOut, usage, err)` — callers that destructure should add a discard for `usage` if they don't care about it.

**b. Find all callers of `llmapi.Turn` (rare; usually only in test mocks).**

Old signature:
```go
completion, err := client.Turn(ctx, messages, tools)
// Use completion.Content, completion.ToolCalls
```

New signature:
```go
// TODO: pass the provider-specific model identifier (e.g. claudellmapi.ModelHaiku45).
// The previous Model config on each provider service was removed in v1.28.0.
content, toolCalls, usage, err := client.Turn(ctx, "", messages, tools, nil)
```

`provider` is NOT a `Turn` argument — `Turn` is invoked on a specific provider via `ForHost`. Pass `""` for the model with a `// TODO:` comment, same convention as Chat.

**c. Find all callers of `llmapi.Executor.ChatLoop` (rare; usually only in workflow integration tests).**

Old signature:
```go
messagesOut, status, err := exec.ChatLoop(ctx, messages, tools)
```

New signature:
```go
// TODO: pick a provider hostname and a model identifier for the workflow run.
messagesOut, usage, status, err := exec.ChatLoop(ctx, "", "", messages, tools, nil)
```

Same `""` + `// TODO:` convention as `Chat`. Note the return order changed — `usage` was inserted before `status`. Callers that destructure should update accordingly.

If the project triggers the `ChatLoop` workflow indirectly via `foremanapi.Run` or any other workflow runner (passing initial state as a `map[string]any`), the initial state must now include `"provider"` and `"model"` keys. The workflow's declared inputs were extended in v1.28.0 from `("messages", "tools")` to `("provider", "model", "messages", "tools", "options")`.

**d. `MockTurn` handler signatures.**

Update mock handler closures across all `service_test.go` files that mock claude/chatgpt/gemini providers:

Old:
```go
mock.MockTurn(func(ctx context.Context, messages []llmapi.Message, tools []llmapi.Tool) (completion *llmapi.TurnCompletion, err error) {
    return &llmapi.TurnCompletion{Content: "..."}, nil
})
```

New:
```go
mock.MockTurn(func(ctx context.Context, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.TurnOptions) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error) {
    return "...", nil, llmapi.Usage{Turns: 1, Model: model}, nil
})
```

**e. `MockChat` handler signatures (on the `llm.core` mock).**

Old:
```go
mock.MockChat(func(ctx context.Context, messages []llmapi.Message, tools []string) (messagesOut []llmapi.Message, err error) { ... })
```

New:
```go
mock.MockChat(func(ctx context.Context, provider string, model string, messages []llmapi.Message, toolURLs []string, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) { ... })
```

**f. `MockChatLoop` handler signatures (on the `llm.core` mock).**

Old:
```go
mock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, messages []llmapi.Message, tools []llmapi.Tool) (messagesOut []llmapi.Message, err error) { ... })
```

New:
```go
mock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, messages []llmapi.Message, tools []llmapi.Tool, options *llmapi.ChatOptions) (messagesOut []llmapi.Message, usage llmapi.Usage, err error) { ... })
```

**g. Drop config references.**

Search every `config.yaml` and `config.local.yaml` for these and remove them (commented or not):
- Under `llm.core:` → `ProviderHostname`
- Under `claude.llm.core:`, `chatgpt.llm.core:`, `gemini.llm.core:` (or any provider hostname) → `Model`

These configs no longer exist; leaving them produces a runtime warning about unknown config keys.

**h. Drop `SetProviderHostname` and `SetModel` calls from test setup.**

Search Go test files for `.SetProviderHostname(` or `<llm-service-var>.SetModel(` and delete those lines — the methods no longer exist. Provider/model are now passed to each `Chat` call directly.

**i. `TurnCompletion` type removal.**

The `llmapi.TurnCompletion` type was deleted. Any code that still references it (struct literals, type assertions, return types) must be rewritten to use the flat `(content, toolCalls, usage)` returns.

**j. Final verification.**

After all edits, suggest the user run:
```
go vet ./...
go test ./... -count=1
```
and grep for the `// TODO:` comments inserted at every migrated `Chat`, `Turn`, and `ChatLoop` call site to confirm provider/model values have been filled in. Any `Chat(ctx, "", "", ...)`, `Turn(ctx, "", ...)`, or `ChatLoop(ctx, "", "", ...)` left in the code will fail at runtime with a 400 Bad Request from the new validation.

#### Step 7: Audit `env.yaml` / `env.Push` Semantics Flip

v1.28.0 changed how the `env` package loads YAML files. Pre-1.28 loaded values into an in-memory shadow store that `env.Get` consulted ahead of `os.Getenv` — yaml effectively *overrode* the OS env, and only callers using `env.Get` saw yaml values. v1.28 writes yaml values through to the real OS environment at package init using dotenv conventions: **OS env now wins over yaml**, and yaml values are visible to every consumer of `os.Getenv` (third-party SDKs included).

Two specific things can silently break:

**a. Conflicting OS env vs. yaml entries.**

If a project's `env.yaml` (or `env.local.yaml`) contains keys that operators *also* set via shell, systemd, k8s, docker, or CI, the runtime value flips:
- Pre-1.28: yaml entry won (in-memory store consulted first by `env.Get`).
- v1.28+: OS entry wins (yaml is only applied when the key is absent from the OS env).

Read every `env.yaml` and `env.local.yaml` in the project. For each key, ask: *is this key also commonly set via the deployment environment?* The high-risk keys to call out explicitly to the user are `MICROBUS_NATS`, `MICROBUS_NATS_USER`, `MICROBUS_NATS_PASSWORD`, `MICROBUS_NATS_TOKEN`, `MICROBUS_DEPLOYMENT`, `MICROBUS_PLANE`, `MICROBUS_LOCALITY`, `MICROBUS_LOG_DEBUG`, anything starting with `OTEL_`, `AWS_`, `GOOGLE_`, or `GCP_`. If any of these appear in a yaml file, emit a warning listing the file, the key, and a one-line note that the deployed value will now win over the yaml value. Do not delete or edit the yaml entries — the developer must decide whether to keep them as fallbacks (the new behavior) or remove them.

**b. `env.Push` + `t.Parallel()`.**

Pre-1.28 `env.Push` mutated only the in-memory shadow store, so concurrent tests using `Push` did not race. v1.28 `env.Push` mutates the real OS env via `os.Setenv` (the whole point — third-party SDKs that read `os.Getenv` must see the test's overrides). The OS env is process-global, so any test that calls `env.Push` *must not* call `t.Parallel()`, and tests in the same package using `Push` must run serially.

Grep across `*_test.go` for files that contain both `env.Push(` and `t.Parallel()`. For each file, check whether any test that calls `env.Push` (directly or indirectly via a helper) is marked parallel. The fix is to remove `t.Parallel()` from those specific test functions. Emit a list of suspect file:line locations to the user; do not auto-edit because the relationship between Push call sites and parallel markers may be indirect (helpers, table-driven tests).

#### Step 8: Validate Subscription HTTP Methods

v1.28.0 validates HTTP method strings at subscription registration time. Only the standard verbs — `GET`, `HEAD`, `POST`, `PUT`, `DELETE`, `CONNECT`, `OPTIONS`, `TRACE`, `PATCH` — and the wildcard `ANY` are accepted. Matching is case-insensitive. Any other method string causes the microservice to fail at startup with a registration error.

Audit two surfaces:

**a. Manifest `method:` fields.**

Grep all `manifest.yaml` files for `method:` entries (under `webs:`, `functions:`, `outboundEvents:`). For each value, check that it is one of the accepted tokens (case-insensitive). Common offenders: typos like `Get` vs `GET` (now fine, case-insensitive), and non-standard verbs like `SEARCH`, `LINK`, `MKCOL`, `PROPFIND`, `LOCK`. The value `ANY` is accepted; `*` is *not* — a remnant `*` from a much older Microbus version would now be rejected (it was already deprecated in milestone 24 in favor of `ANY`).

**b. Hand-rolled `sub.At` / `sub.Method` calls in service code.**

Grep `*.go` files for `sub.At(` and `sub.Method(`. The first argument to each is the HTTP method. Validate the same way.

If anything fails the check, list the file, line, and offending value to the user. Do not auto-edit unless the offending value is `*` — that one has a clear mechanical replacement to `ANY`. Anything else (custom verbs) requires a design call about whether to use `ANY` and dispatch internally, or whether the endpoint should be removed.

#### Step 9: Heads-Up Audits

These two changes affect smaller audiences but are worth a quick mechanical check.

**a. `Microbus-Time-Budget` header format.**

The framework now serializes the `Microbus-Time-Budget` header as a Go duration string (`5ms`, `1h30m`) rather than a bare integer. The parser still accepts the legacy bare-integer format on read, so projects that *consume* the header through the framework are unaffected. The risk is in projects that *emit* or *parse* the header by hand (rare).

Grep all `*.go` files (excluding `vendor/` and the `fabric` module cache) for the literal string `"Microbus-Time-Budget"`. For each hit, inspect: if the code is reading the header, no change is needed (legacy format is still accepted). If the code is writing the header, switch the value to a Go duration string. Report any hits to the user without auto-editing.

**b. PROD ingress proxy port blocking.**

In `PROD` deployments, the HTTP ingress proxy now blocks inbound requests to ports `:1`–`:1023`, except `:80` and `:443`. Port `:888` was already blocked in all environments. The internal-only ports `:417`, `:428`, `:444`, and `:888` were never reachable through the proxy, so most projects are unaffected.

The risk is a project that uses `PortMappings` to map an external port to an internal port in the blocked range (other than `:80`/`:443`/`:888`). Read every `http.ingress.core` block in `config.yaml` and `config.local.yaml`, and look at the `PortMappings` value (if present). If any mapping rule's target internal port (`z` in `x:y->z`) falls in `:1`–`:1023` excluding `:80`/`:443`, flag it to the user. The mapping will silently produce 404s in PROD.

**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

### Provider as Microservice

The LLM service delegates to a provider microservice over the bus via `llmapi.NewClient(svc).ForHost(providerHostname).Turn(...)`. Provider selection is a config (`ProviderHostname`), not code - swapping providers requires no code changes. The three provider microservices (`claudellm`, `chatgptllm`, `geminillm`) each implement the same `Turn` endpoint using types from `llmapi`.

### Tool Resolution

The public `Chat` endpoint takes `[]string` of canonical Microbus URLs (e.g. `calculatorapi.Arithmetic.URL()`). At chat time `InitChat` fetches each host's `:888/openapi.json` in parallel (the connector's built-in handler, reached via `controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)`) and resolves the requested URL against the document's port-qualified path keys to build `[]llmapi.Tool` - capturing operation name, description, request-body JSON Schema, method, URL, and feature type. Authorization piggybacks on the OpenAPI fetch: the handler omits operations the caller's actor cannot satisfy, so unauthorized tools are simply absent from the resolved tool list. Operations whose feature type is not `FeatureFunction`/`FeatureWeb`/`FeatureWorkflow` never appear in the document and are therefore silently skipped.

Tool name de-duplication happens during resolution: when two endpoints share an operation name, the first one keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order. This lets callers concatenate URLs across multiple `*api` packages without collision.

The internal `ChatLoop` workflow and `InitChat`/`ExecuteTool` tasks carry already-resolved `[]llmapi.Tool` between steps via flow state - only the caller-facing `Chat` shape is `[]string`. `ExecuteTool` branches on `Tool.Type == FeatureWorkflow` to dispatch workflow tools as dynamic subgraphs; all other types go through a direct bus call.

### ChatLoop Workflow

The ChatLoop uses `AddTransitionForEach` on `pendingToolCalls` to fan out one `ExecuteTool` step per tool call. When the LLM returns no tool calls, `toolsRequested` is false and the conditional transition to END is taken.

`ExecuteTool` uses `toolExecuted`/`toolExecutedOut` (Out suffix pattern) for subgraph re-entry detection. On first run (`toolExecuted=false`), it executes the tool. On re-run after a dynamic subgraph completes (`toolExecuted=true`), it collects the child's result.

### messagesOut Naming

The output uses `messagesOut` (Out suffix) so the workflow's `messages` state key contains the full conversation after completion. This enables `foremanapi.Continue` to append new messages naturally via `ReducerAppend`.

### Stateful Config

Both `ProviderHostname` and `MaxToolRounds` are operator-configured settings, not per-request parameters. The `Chat` endpoint takes only `messages` and `tools` (URLs) - callers don't need to know about provider selection or round limits.

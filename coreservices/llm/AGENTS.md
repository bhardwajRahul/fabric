**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

### Provider as Microservice

The LLM service delegates to a provider microservice over the bus via `llmapi.NewClient(svc).ForHost(providerHostname).Turn(...)`. Provider selection is a config (`ProviderHostname`), not code - swapping providers requires no code changes. The three provider microservices (`claudellm`, `openaillm`, `geminillm`) each implement the same `Turn` endpoint using types from `llmapi`.

### Tool Resolution

Tool schemas are fetched from the target service's OpenAPI endpoint at call time (`tools.go`). The OpenAPI URL is derived by replacing the tool URL's path with `/openapi.json`. The `x-feature-type` extension field in the OpenAPI spec distinguishes functions from workflows.

### ChatLoop Workflow

The ChatLoop uses `AddTransitionForEach` on `pendingToolCalls` to fan out one `ExecuteTool` step per tool call. When the LLM returns no tool calls, `toolsRequested` is false and the conditional transition to END is taken.

`ExecuteTool` uses `toolExecuted`/`toolExecutedOut` (Out suffix pattern) for subgraph re-entry detection. On first run (`toolExecuted=false`), it executes the tool. On re-run after a dynamic subgraph completes (`toolExecuted=true`), it collects the child's result.

### messagesOut Naming

The output uses `messagesOut` (Out suffix) so the workflow's `messages` state key contains the full conversation after completion. This enables `foremanapi.Continue` to append new messages naturally via `ReducerAppend`.

### Stateful Config

Both `ProviderHostname` and `MaxToolRounds` are operator-configured settings, not per-request parameters. The `Chat` endpoint takes only `messages` and `tools` - callers don't need to know about provider selection or round limits.

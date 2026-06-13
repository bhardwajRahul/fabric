# llm.core

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Design Rationale

### Caller-Selected Provider and Model

`Chat` takes `provider` and `model` as required arguments. There is no `ProviderHostname` config and providers no longer carry a `Model` config - every call site picks both. This was a deliberate breaking change in v1.28.0:

- **Provider** is a *capability* choice (different schemas, tool-calling behavior, rate limits). Caller must know.
- **Model** is a *cost* choice (Opus is ~100x the cost of Haiku). Hiding it behind a config is dangerous; an operator config edit could silently change a service's spend by orders of magnitude. Forcing it into the signature makes cost visible at every call site.

Each provider's `*api` package exports typed model constants (e.g. `claudellmapi.ModelHaiku45`) so a typo is a compile error rather than a runtime failure.

### `Turn` on `llm.core` is a stub

The `Turn` endpoint is part of the contract that provider microservices implement. `llm.core` itself is not a provider - calling its `Turn` endpoint returns 501. Use `llmapi.NewClient(svc).ForHost(<providerHost>).Turn(...)` to invoke a specific provider directly, or use `Chat` for the conversation loop.

The endpoint stub is registered (rather than removed) because `llmapi.Turn.URL()` is referenced elsewhere as the canonical form of the contract.

### Tool Resolution

The public `Chat` endpoint takes `[]string` of canonical Microbus URLs (e.g. `calculatorapi.Arithmetic.URL()`). At chat time `InitChat` fetches each host's `:888/openapi.json` in parallel (the connector's built-in handler, reached via `controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)`) and resolves the requested URL against the document's port-qualified path keys to build `[]llmapi.Tool` - capturing operation name, description, request-body JSON Schema, method, URL, and feature type. Authorization piggybacks on the OpenAPI fetch: the handler omits operations the caller's actor cannot satisfy, so unauthorized tools are simply absent from the resolved tool list. Operations whose feature type is not `FeatureFunction`/`FeatureWeb`/`FeatureWorkflow` never appear in the document and are therefore silently skipped.

Tool name de-duplication happens during resolution: when two endpoints share an operation name, the first one keeps the bare name and subsequent ones get `_2`, `_3`, ... suffixes in argument order. This lets callers concatenate URLs across multiple `*api` packages without collision.

The internal `ChatLoop` workflow and `InitChat`/`ExecuteTool` tasks carry already-resolved `[]llmapi.Tool` between steps via flow state - only the caller-facing `Chat` shape is `[]string`. `ExecuteTool` branches on `Tool.Type == FeatureWorkflow` to dispatch workflow tools as dynamic subgraphs; all other types go through a direct bus call.

### Token Usage Tracking

Each provider's `Turn` populates `llmapi.Usage` with input/output/cache-read/cache-write tokens, the resolved model identifier, and `Turns: 1`. `Chat` aggregates per-turn usage via `Usage.Add` and returns the aggregate alongside the messages. The `LLMTokens` counter metric (Prometheus name `microbus_llm_tokens_total`, labeled by `provider`, `model`, `direction`) is emitted from `logCompletion` for each turn so cost-by-model dashboards work out of the box.

**Why not the OTel GenAI semantic convention?** The OTel GenAI spec defines a standard metric `gen_ai.client.token.usage` (histogram) with attributes `gen_ai.token.type`, `gen_ai.system`, `gen_ai.request.model`, etc., which off-the-shelf APM dashboards (Datadog, Grafana, Honeycomb) recognize. We deliberately did not adopt it for v1.28.0 because:

- It requires a histogram (higher cardinality and storage cost than a counter) for what is fundamentally a cumulative measurement.
- The `gen_ai.system` attribute requires a hostname-to-vendor mapping (`claude.llm.core` → `"anthropic"`, etc.) that doesn't exist natively in Microbus and would couple `llm.core` to the set of known providers.
- The spec doesn't yet standardize cache read/write tokens, which are first-class in `Usage`.

If/when external GenAI dashboard compatibility is needed, the OTel metric can be emitted in parallel as a second metric - both can coexist. The `Usage` struct already carries everything needed; only the attribute key mapping and a histogram emission would be added in `logCompletion`.

`ChatLoop` workflow accumulates usage in flow state via `ProcessResponse` (which `Add`s the per-turn `turnUsage` into the running `usage` key) and exposes `messages` and `usage` as declared workflow outputs.

### ChatLoop Workflow

The chat loop is `initChat → initLLM → processResponse → forEach pendingToolCalls → executeTool → loopLLM → processResponse`. Each round, processResponse decides:

- If no tool calls are pending (the prior turn's `stopReason` was a completion: `end_turn`,
  `stop_sequence`, or `refusal`) or the round limit is exhausted, it calls
  `flow.Goto(workflow.END)` to exit the loop. The forEach transition is skipped. A
  truncation / pause_turn / unknown `stopReason` never reaches `ProcessResponse` — `CallLLM`
  fails the step before that, so the workflow's `OnError` route (if any) handles it.
- Otherwise the forEach fans out one executeTool per pending tool call. All branches converge at `loopLLM` via `graph.SetFanIn("loopLLM")`. The fan-in merges per-tool messages into the `messages` field, which is wired with `graph.SetReducer("messages", workflow.ReducerAppend)` at graph-build time so the LLM sees the full conversation history on the next round.

`initLLM` and `loopLLM` are two graph positions sharing one task URL (`CallLLM`). `initLLM` is the initial sequential call after `initChat`; `loopLLM` is the fan-in nexus that closes each per-round tool cohort. The split is forced by the lineage validator: a fan-in target requires a stack frame to pop, so the initial entry (which has no frame) cannot also be the fan-in. Both nodes dispatch to the same task; the foreman runs `CallLLM` once at each visit. See `examples/creditflow` for the same pattern (the `reviewJoin` / `reviewCredit` split).

`ExecuteTool` dispatches a workflow tool via `flow.Subgraph(def.URL, input)`, which returns `(out, yield, err)`. On
the first call it yields (the foreman parks the step and runs the child); on re-entry it returns the child's
`final_state` as `out`, which `ExecuteTool` serializes into the tool-result message. The child's output is no longer
merged into the parent's state, so the old `preSubgraphKeys` snapshot-diff is gone. The `toolExecuted`/`toolExecutedOut`
argument is now vestigial - re-entry is detected by `flow.Subgraph`'s `yield`, not a state field - and is retained only
to avoid regenerating boilerplate.

**The live `Chat` entry point does not use this workflow.** `Chat` implements the loop entirely in Go for the synchronous request/response case. `ChatLoop` is exposed as a workflow so it can be invoked via `foremanapi.Run` (or composed as a subgraph) when the caller wants the foreman's persistence, fork/resume, and observability for an LLM conversation. The graph is part of the API contract whether or not it's exercised by every test.

### Options Layering

`ChatOptions` (caller-facing) and `TurnOptions` (provider-facing) are deliberately separate types so each layer controls what it exposes. `ChatOptions` adds `MaxToolRounds` (loop-level) and forwards `MaxTokens`/`Temperature` to a `TurnOptions` built per turn. The duplication is intentional: it lets future fields be added to one layer without auto-leaking to the other.

`MaxToolRounds` remains as a service config (operational guardrail), with `ChatOptions.MaxToolRounds` as an optional override.

### Stop-Reason Branching

`Turn` returns a normalized `stopReason` (constants in `llmapi/stopreason.go`: `StopReasonEndTurn`,
`StopReasonToolUse`, `StopReasonMaxTokens`, `StopReasonStopSequence`, `StopReasonRefusal`,
`StopReasonPauseTurn`, `StopReasonUnknown`). Each provider maps its native value (Anthropic's
`stop_reason`, OpenAI's `finish_reason`, Gemini's `finishReason`) into this set; anything that
doesn't map cleanly becomes `StopReasonUnknown`. The `Chat` loop and `CallLLM` workflow task
branch on it through `stopReasonError`:

- `tool_use` → continue the loop and execute the tool calls.
- `end_turn` / `stop_sequence` / `refusal` → emit the final assistant message and return.
- `max_tokens` → return `errors.New("LLM response truncated at max_tokens", 507)`. Truncation is
  treated as a budget violation: if the caller set `MaxTokens` (or accepted the provider default)
  and the response hit the cap, the orchestrator's job is to surface that, not to ship a partial
  response downstream. There is no `TruncationPolicy` knob; partial content is dropped because
  attaching it to the error would inflate every truncation failure with a potentially large
  payload, and the caller can re-architect (workflow loop with explicit continuation) if they
  need long-form generation.
- `pause_turn` → `502`. Anthropic's long-running-tools extension isn't wired through today; if it
  becomes load-bearing, replace the error with a `Sleep`+retry path.
- `""` / unknown → `502`, on the same principle as truncation: fail loud rather than silently
  treat an unrecognized state as completion.

The branching lives in `stopReasonError(stopReason, provider, model)` in `service.go` and is called
from both the live `Chat` loop and `CallLLM`. The post-loop "exhausted tool rounds, one final
call without tools" path also runs through the same gate.

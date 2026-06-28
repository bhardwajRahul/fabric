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

### Tool-Call Tracking

The `ToolCalls` counter (OTel name `microbus_llm_tool_calls`, queried in Prometheus as `microbus_llm_tool_calls_total`), labeled by `tool_url`, `tool_type`, and `outcome`, records one increment per resolved tool invocation. `tool_type` is the resolved feature type (`function`/`web`/`workflow`); `outcome` is `ok` or `error`.

It is emitted at the two places a tool actually resolves, so the live `Chat` Go loop and the `ChatLoop` workflow are both covered without double counting (a given tool call runs through exactly one path):

- **Direct bus tools** (`executeTool` in `tools.go`) - used by the live `Chat` loop for every tool and by the `ExecuteTool` task for non-workflow tools. `executeTool` folds transport errors and `>=400` responses into the tool-result JSON (returning a nil Go error so one bad tool doesn't fail the whole chat), so the outcome can only be read *inside* `executeTool`; a deferred increment flips to `error` on any of those failure branches. The `tool not found` early return is not counted (it has no URL to attribute).
- **Workflow tools** (the `ExecuteTool` task's subgraph branch) - counted on resolution, never on the park. `flow.Subgraph` yields (parks) on the first call and re-enters on the child's terminal state; the increment fires on re-entry (`ok`) or on a subgraph error (`error`), so a parked-but-not-yet-finished tool is not counted until it actually settles.

This is the LLM service's tool-use signal; it is distinct from the engine's `dwarf_task_concurrency_running{task_url}` (which sees workflow-tool subgraphs as ordinary tasks) and the connector's generic `microbus_client_*` downstream metrics.

### ChatLoop Workflow

The chat loop is `InitChat → FirstLLM → ProcessResponse → forEach pendingToolCalls → ExecuteTool → NextLLM → ProcessResponse`. Each round, ProcessResponse decides:

- If no tool calls are pending (the prior turn's `stopReason` was a completion: `end_turn`,
  `stop_sequence`, or `refusal`) or the round limit is exhausted, it calls
  `flow.Goto(workflow.END)` to exit the loop. The forEach transition is skipped. A
  truncation / pause_turn / unknown `stopReason` never reaches `ProcessResponse` — `CallLLM`
  fails the step before that, so the workflow's `OnError` route (if any) handles it.
- Otherwise the forEach fans out one ExecuteTool per pending tool call. All branches converge at `NextLLM` via `graph.SetFanIn("NextLLM")`. The fan-in merges per-tool messages into the `messages` field, which is wired with `graph.SetReducer("messages", workflow.ReducerAppend)` at graph-build time so the LLM sees the full conversation history on the next round.

`FirstLLM` and `NextLLM` are two graph positions sharing one task URL (`CallLLM`). `FirstLLM` is the initial sequential call after `InitChat`; `NextLLM` is the fan-in nexus that closes each per-round tool cohort. The split is forced by the lineage validator: a fan-in target requires a stack frame to pop, so the initial entry (which has no frame) cannot also be the fan-in. Both nodes dispatch to the same task; the foreman runs `CallLLM` once at each visit. See `examples/creditflow` for the same pattern (the `ReviewJoin` / `ReviewCredit` split).

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

### Rate-limit handling (the `retryAfter` contract)

A rate-limited provider error is identified by the **presence of a `retryAfter` attribute** on the
error (a duration string), never by the `429` status code. This is deliberate: a `429` can also report a
request the provider will never accept (e.g. one whose token count permanently exceeds the model's limit),
so each provider classifies its own error (it alone sees the status, body, headers, and token count) and
attaches `retryAfter` only on a genuine, transient throttle. Presence ⇒ retryable and the value is the
wait; absence ⇒ permanent. This fails closed - an error nobody could classify simply has no `retryAfter`
and is not retried.

**Short-term retry is the engine's, bounded by the task's time budget.** When `turn` returns a
`retryAfter` error, `CallLLM` arms `flow.Retry` with the wait carried in the initial delay (multiplier
1.0, no cap - a rate limit is a known-reset condition, not something to exponentially back off). The
horizon is `frame.Of(ctx).TimeBudget()` - the task's own time budget, read from the inbound frame, *not*
a config. So a rate-limited turn rides out only as long as the step is worth running; `flow.Retry`'s
next-delay give-up fails fast the moment a wait would overshoot the budget (e.g. a `retryAfter` longer
than the remaining budget never parks). There is **no** rate-limit retry config - the budget is the one
knob, and it is the same one that already says "how long is this task worth."

**Long-term retry is the caller's.** A patient, no-HITL workload (e.g. async document extraction) that
would rather finish late than fail wraps `Chat` in its own retry loop with whatever policy it likes. Two
affordances make that cheap and correct: `Chat` returns the **messages accumulated before the failure**
(so the caller resumes from them instead of restarting the conversation and re-paying for prior turns
and tool calls), and `llmapi.RetryAfter(err) (wait, retryable)` is the typed accessor for the
`retryAfter` signal (so the caller decides whether and how long to wait without spelunking the error's
properties or trusting the `429` status). The caller owns the attempt cap; the framework supplies the
facts.

**Provider-side preemption gate.** Each provider microservice also keeps an in-memory
`map[model]blockedUntil`: on a `retryAfter`-bearing throttle it records the block and, at the top of
`Turn`, preempts with a synthetic `429` (carrying the remaining wait as `retryAfter`) without dialing
the provider. It lives in the provider, not here, because the provider holds the API key (so it
unambiguously is one account) and the gate then covers every caller, not just `llm.core`-routed traffic.
Keyed by model, since rate limits are per-model.

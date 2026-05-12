## Design Rationale

### State is JSON, not Go values

Every entry in `Flow.state` and `Flow.changes` is a `json.RawMessage`, not a native Go value. `Set`, `SetString`, `SetInt`, etc. all marshal to JSON before storing. There are two reasons:

- The foreman persists state to SQL and ships it across replicas at every step. Native Go values would force the foreman to know every task's types.
- `SetChanges` compares against a snapshot by JSON byte-equality. Storing native values would make the diff implementation-defined (e.g., struct field ordering, unexported fields).

This is also why reducers (`Append`, `Add`, `Union`, `Merge`) marshal/unmarshal at every step rather than working in Go: the input may already be `json.RawMessage`, and `union` dedupes by raw JSON string - so it is sensitive to whitespace and number formatting. Values produced through Go's `encoding/json` are deterministic and dedupe consistently; hand-built `json.RawMessage` is the danger zone.

### Reducer selection is name-driven; SetReducer is the escape hatch

The reducer for a fan-in field is normally inferred from the field name's prefix:

- `sum*` → `Add` (numeric)
- `list*` → `Append` (array, duplicates kept)
- `set*` → polymorphic: `Union` for arrays (dedupe), `Merge` for objects (new key wins)
- anything else → `Replace` (last write wins)

The character right after the prefix must be uppercase. `summary`, `listening`, `setup` do not match; `sumScore`, `listMessages`, `setUsers` do. This is the same boundary rule used elsewhere in Microbus naming conventions.

`graph.SetReducer(field, reducer)` overrides the inferred reducer. Reach for it only when the field name is dictated by an external schema or a pre-existing API surface that can't be renamed (e.g. `messages` mandated by an LLM provider's request format). For new graphs and new fields, the prefix convention is preferred.

The polymorphism on `set*` only applies to inferred reducers. Explicit `SetReducer(name, ReducerUnion)` rejects object values; explicit `SetReducer(name, ReducerMerge)` rejects array values.

### `state` and `changes` are tracked separately on purpose

A `Flow` carries two maps: `state` (everything the task can read) and `changes` (only what *this* step wrote). The foreman persists `changes` per step as the audit trail and merges them via reducers at fan-in points. Without the split, fan-in could not distinguish "this branch wrote X" from "this branch read X."

`SetChanges(source, snapshot)` only records fields whose JSON differs from the snapshot, so a parse-modify-write-back loop (`ParseState` → mutate struct → `SetChanges`) does not pollute the change set with untouched fields.

`SetState` and `SetRawState` write to `state` *without* recording changes - these are orchestrator entry points (the foreman uses them when materializing a flow from SQL), not for task code.

### Generic `Set` is the contract; typed setters are convenience

`Set(key, value)` returns an error and is the full API. The typed variants (`SetString`, `SetInt`, ..., `SetDuration`) panic on marshal failure rather than return an error. This is deliberate - primitive types never fail to marshal, so propagating an error at every call site would be noise. If a typed setter ever did panic, the connector's request handler wraps task invocations in `errors.CatchPanic` (see `connector/subscribe.go`), so the panic surfaces as a normal error response to the foreman rather than crashing the microservice.

### Control signals are deferred requests, not actions

`Goto`, `Retry` / `RetryNow`, `Sleep`, `Interrupt`, `Subgraph` do *not* alter execution mid-task. They set a field on `Flow` and the task continues. The task should `return nil` immediately after - the control signal *is* the result. The foreman reads the signal after the task returns and acts on it.

This is why a task can call `flow.Interrupt(payload); return nil` and have the flow pause: the return is what hands control back to the foreman.

### `Retry` returns a bool the caller must branch on

`Retry(maxAttempts, ...)` returns `true` when a retry will be scheduled, `false` when attempts are exhausted. The expected idiom is:

```go
if err != nil {
    if flow.Retry(5, time.Second, 2.0, 30*time.Second) {
        return nil      // retry scheduled, suppress the error
    }
    return err           // exhausted, surface the error
}
```

The bool exists so the foreman can tell the task when retries have been spent. On `false`, the task surfaces the error and the foreman takes whatever path it would normally take for a failed task (`OnError` transition or flow failure). The attempt counter is carried in `Flow.attempt` and incremented by the foreman before each redispatch.

### `Subgraph` and `Interrupt` share a re-entry pattern

`flow.Subgraph(workflowURL, input); return nil` parks the step. The foreman runs the child workflow and re-dispatches the *same* task once the child completes. On re-entry the task sees the child's outputs in state and should return normally without calling `Subgraph` again.

`Interrupt` works the same way against an external `Resume` rather than a child flow. There is no first-class "I'm being re-entered" flag - tasks must encode the distinction themselves in state. The universal idiom is the **`*Out` suffix**: a task input arg `foo` and output arg `fooOut` represent the same state field, with the `Out` suffix only because Go forbids duplicate parameter names in input and output positions. The framework writes the output back without the suffix. First run: `foo` is absent (zero value), task does its side trip, returns. Second run: `foo` carries the result, task returns normally. This convention is the same one Microbus task generation uses everywhere.

### Subgraph state-passing semantics

The wire-up is done by the foreman, not by the workflow package itself, but the rules are worth knowing because they shape what a task can rely on:

- **Into the child:** Build parent's full `state` + accumulated `changes` of the surgraph step. For dynamic subgraphs (`flow.Subgraph(url, input)`), merge the explicit `input` map on top using the *child* graph's reducers. Filter the result through the child's `DeclareInputs`. That becomes the child flow's initial state.
- **Back to the parent:** The child's `final_state` is filtered through the child's `DeclareOutputs` and merged into the surgraph step's `changes` using the *parent* graph's reducers.

Both `DeclareInputs` and `DeclareOutputs` are enforced - they are the subgraph's contract surface. The child graph's reducers govern the input merge only; the parent graph's reducers govern the output merge only.

### Transitions are rules evaluated per fan-out, not graph edges

Multiple transitions can match from the same task. *All* matching ones fire - that is what makes fan-out implicit:

- Unconditional + unconditional = parallel branches.
- Two `When` transitions with non-mutually-exclusive predicates = parallel branches (the framework does not enforce exclusivity).
- `ForEach` spawns one branch per array element.
- `WithGoto` transitions are excluded from automatic evaluation; only taken when the task calls `flow.Goto(target)`.
- `OnError` transitions are taken when the source task returned an error; the error is serialized as a `TracedError` into the target task's state under the key `onErr`. The error-handler task otherwise sees the same state as any other task in the workflow and can also pull values via typed input args. Without an `OnError` transition, an error fails the flow.

`OnError` cannot combine with `When`, `ForEach`, or `WithGoto` - the validator rejects it.

`OnError` transitions carry an optional `StatusCode` discriminator (zero = "match any error"). `AddTransitionOnTimeout` is the only public constructor that sets a non-zero status code today; it sets `StatusCode: http.StatusRequestTimeout` so it fires only when the task's error has status 408. Both foreman-side `pub.Timeout` expiry and subscriber-side handler timeouts surface as 408, so a single transition covers either side timing out first - there's no asymmetry the foreman has to encode separately. When both an `OnTimeout` and a catch-all `OnError` are wired from the same task, the status-coded transition wins on a match and the catch-all wins otherwise (most-specific-wins). The `StatusCode` field is the extension point for future status-keyed transitions (e.g. an `OnConflict` for 409s) without redesigning the `Transition` struct - the validator rejects `StatusCode != 0` paired with `OnError: false`.

### Error transitions route, they don't retry

`OnError` and `OnTimeout` are for routing to a *different* handler (a fallback model, a degraded path, an error-logging task). The validator rejects `from == to` on either kind, because a self-loop bypasses every safety property of `flow.Retry`: it creates a new step row per cycle (instead of reusing one), it has no attempt counter, no backoff, and no bound. For "keep retrying this task" semantics, use `flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay)` in the task body - or `flow.RetryOnTimeout(err, ...)` for the common case of "retry only when the error is a 408." The two primitives compose: the task retries internally until exhausted, then the surviving error propagates and the graph's `OnTimeout`/`OnError` transition (to a *different* target) takes over.

### Fan-in is explicit via SetFanIn

`Graph.SetFanIn(name)` marks a node as a fan-in nexus. `Graph.Validate` enforces a single invariant: every transition stays in its current lineage scope, or is an edge into a `SetFanIn` node that pops one frame. The validator computes a lineage stack per node by BFS from the entry point — pushing a frame at fan-out sources (any node with 2+ non-goto/non-error outgoing edges, or any forEach edge) and popping at `SetFanIn` nodes. It rejects:

- A `SetFanIn` reached with empty stack (fan-in outside any fan-out scope).
- END reached with a non-empty stack (a branch that doesn't pop back through every fan-in on its way to END).
- The same node reached via paths with different stacks. Aliasing is the escape hatch — register the same task URL under two different names so each lives in its own scope.
- A `Goto`, `OnError`, or `OnTimeout` whose target stack would differ from the source's. These edges don't push or pop, so source and target must share a scope. There is no "bypass" for a fan-in via goto.

A consequence of the END rule: the graph is forced to have a single, single-scope termination point. Branches can do whatever they want internally, but every cohort must converge through its `SetFanIn`. Graphs with no fan-out (and therefore no `SetFanIn`) are trivially accepted — the validator just walks the structure and confirms END is reachable with empty stack.

**Side effect: `fanOutToFanIn` map.** As validation runs, it records, per fan-out source, the `SetFanIn` node that pops its frame. The map is unambiguous because the validator forbids two paths reaching the same node with different stacks. The foreman uses this map to handle the empty-cohort case: when a forEach yields zero elements (no branches spawn), the foreman creates the corresponding fan-in step directly with empty state and the parent scope's lineage so the flow continues past the join.

**Direct fan-out → fan-in edge is allowed.** If a fan-out source has one sibling that goes straight to the `SetFanIn` and another that goes through intermediate work, both arrive in the same scope: the push and pop on the direct edge cancel.

### Lineage runtime accounting

A cohort spawn step records `cohort_size = N` (the number of branches actually spawned at this fan-out evaluation, which can differ between runs for `forEach` and `When`). Each branch step inherits `lineage_id = spawn.step_id` for normal transitions, or `lineage_id = parent.lineage_id` for non-fan-out transitions (Goto and OnError edges do not push, regardless of whether the source is statically a fan-out).

When a step transitions to a `SetFanIn` target the foreman atomically increments `cohort_arrivals` on the spawn row inside the same transaction that completes the step. The last incrementer (the one whose post-update `cohort_arrivals == cohort_size`) creates the fan-in step in the same transaction with state merged from all cohort members (steps with that `lineage_id`, ordered by `updated_at, step_id`, reduced via the graph's reducers).

**Cancelled branches count as arrivals.** When an `OnError` route fires, the foreman cancels other branches in the same cohort and increments `cohort_arrivals` by the number of cancellations. Without this the fan-in would block waiting for branches that never arrive.

**Empty cohort fires the fan-in directly.** A fan-out source that produces zero branches (e.g. `forEach` over an empty array, or a static fan-out where all `When` predicates evaluated false) looks up the destination via `graph.FanInFor(sourceTaskName)` and creates that step immediately with empty state and the parent scope's lineage. This is why the validator records `fanOutToFanIn` as a side effect.

**Aliases for re-entry.** A node marked `SetFanIn` cannot be re-entered via Goto from outside its cohort scope (the goto target stack would differ from the source's, which the validator rejects). The pattern for "fan in, then loop with goto" is to introduce two graph positions sharing one task URL — for example `reviewJoin` (the fan-in nexus) and `reviewCredit` (sequential after the join, hosts the goto loop). Both `AddTask("name", url)` registrations point at the same dispatch URL; the graph treats them as distinct positions and the foreman runs the task once per visit. See `examples/creditflow` for a worked example.

**Loop-back via Goto-to-END.** A node downstream of a fan-in can use `flow.Goto(workflow.END)` to terminate the flow even when other transitions out of that node are normal edges. This is how `coreservices/llm`'s `ProcessResponse` exits the chat loop when no more tool calls are pending — the alternative `forEach pendingToolCalls` transition is skipped (Goto wins) and the flow completes cleanly.

### `END` is a pseudo-node

`END` is the constant `"END"`. It is never registered in `nodes`. Transitions targeting `END` mark a terminal path; `Validate` requires at least one. `AddTask(END)` is a no-op so transitions can be built up without special-casing the sink.

### Auto-registration on `AddTransition`

`AddTransition`, `AddTransitionWhen`, `AddTransitionGoto`, `AddTransitionForEach`, `AddTransitionOnError`, `AddTransitionOnTimeout` all auto-register both endpoints as tasks. The first node added - by any path - becomes the default `entryPoint` unless `SetEntryPoint` overrides. So a graph can be built from transitions alone, without explicit `AddTask` calls.

### `DeclareInputs` / `DeclareOutputs` use a three-state encoding

The same convention is used in both directions and in `FilterState`:

- `nil` or empty slice = pass nothing.
- `["*"]` = pass everything.
- Named fields = pass exactly those fields.

`Outputs` filtering applies whether the graph runs as a subgraph or as a root flow. This is what lets a workflow declare "my contract is just these output fields" and have the foreman strip the rest before returning to the caller.

### `RawFlow` is the orchestrator's API; tasks see only `Flow`

`Flow` exposes the read/write/control surface a task author needs. `RawFlow` embeds `Flow` and adds bulk-state mutation (`SetRawState`), control-signal clearing (`ClearControl`, `ClearChanges`), and attempt setting (`SetAttempt`). Only the foreman calls these - they're how a flow gets reconstituted from SQL between steps and how control signals are reset before the next dispatch.

The split is enforced by Go's package-private fields, not by an interface, because `RawFlow` needs to mutate `Flow`'s unexported state directly. This is why both types live in the same package and there is no `Flow` interface.

### `MarshalJSON` exists because of unexported fields

`Flow.MarshalJSON` and `UnmarshalJSON` round-trip through a private `flowJSON` struct because the wire format is the SQL-persisted representation, and the standard encoder cannot see `state`, `changes`, `gotoNext`, etc. Don't add a new field to `Flow` without updating `flowJSON` - silent drop on persistence is the failure mode.

### Empty `forEach` means "no successors via this transition"

`AddTransitionForEach` over an empty state field spawns zero tasks.

Under the legacy validator, if it is the *only* outgoing non-`OnError` transition from the source, the flow completes at the source. This is the correct behavior for "fan out over results, none found" cases, but it is also a foot-gun if you forgot a sibling unconditional transition to a default target.

Under the lineage validator (with `SetFanIn`), an empty forEach is not a footgun: the foreman looks up the source's fan-in via `g.FanInFor(source)` and creates that fan-in step directly with empty state. Execution continues past the join naturally.

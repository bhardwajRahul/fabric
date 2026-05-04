## Design Rationale

### State is JSON, not Go values

Every entry in `Flow.state` and `Flow.changes` is a `json.RawMessage`, not a native Go value. `Set`, `SetString`, `SetInt`, etc. all marshal to JSON before storing. There are two reasons:

- The foreman persists state to SQL and ships it across replicas at every step. Native Go values would force the foreman to know every task's types.
- `SetChanges` compares against a snapshot by JSON byte-equality. Storing native values would make the diff implementation-defined (e.g., struct field ordering, unexported fields).

This is also why reducers (`Append`, `Add`, `Union`) marshal/unmarshal at every step rather than working in Go: the input may already be `json.RawMessage`, and `union` dedupes by raw JSON string — so for objects it is sensitive to key order. Don't union maps with non-canonical key ordering.

### `state` and `changes` are tracked separately on purpose

A `Flow` carries two maps: `state` (everything the task can read) and `changes` (only what *this* step wrote). The foreman persists `changes` per step as the audit trail and merges them via reducers at fan-in points. Without the split, fan-in could not distinguish "this branch wrote X" from "this branch read X."

`SetChanges(source, snapshot)` only records fields whose JSON differs from the snapshot, so a parse-modify-write-back loop (`ParseState` → mutate struct → `SetChanges`) does not pollute the change set with untouched fields.

`SetState` and `SetRawState` write to `state` *without* recording changes — these are orchestrator entry points (the foreman uses them when materializing a flow from SQL), not for task code.

### Generic `Set` is the contract; typed setters are convenience

`Set(key, value)` returns an error and is the full API. The typed variants (`SetString`, `SetInt`, ..., `SetDuration`) panic on marshal failure rather than return an error. This is deliberate — primitive types never fail to marshal, so propagating an error at every call site would be noise. If a typed setter ever did panic, the connector's request handler wraps task invocations in `errors.CatchPanic` (see `connector/subscribe.go`), so the panic surfaces as a normal error response to the foreman rather than crashing the microservice.

### Control signals are deferred requests, not actions

`Goto`, `Retry` / `RetryNow`, `Sleep`, `Interrupt`, `Subgraph` do *not* alter execution mid-task. They set a field on `Flow` and the task continues. The task should `return nil` immediately after — the control signal *is* the result. The foreman reads the signal after the task returns and acts on it.

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

`Interrupt` works the same way against an external `Resume` rather than a child flow. There is no first-class "I'm being re-entered" flag — tasks must encode the distinction themselves in state. The universal idiom is the **`*Out` suffix**: a task input arg `foo` and output arg `fooOut` represent the same state field, with the `Out` suffix only because Go forbids duplicate parameter names in input and output positions. The framework writes the output back without the suffix. First run: `foo` is absent (zero value), task does its side trip, returns. Second run: `foo` carries the result, task returns normally. This convention is the same one Microbus task generation uses everywhere.

### Subgraph state-passing semantics

The wire-up is done by the foreman, not by the workflow package itself, but the rules are worth knowing because they shape what a task can rely on:

- **Into the child:** Build parent's full `state` + accumulated `changes` of the surgraph step. For dynamic subgraphs (`flow.Subgraph(url, input)`), merge the explicit `input` map on top using the *child* graph's reducers. Filter the result through the child's `DeclareInputs`. That becomes the child flow's initial state.
- **Back to the parent:** The child's `final_state` is filtered through the child's `DeclareOutputs` and merged into the surgraph step's `changes` using the *parent* graph's reducers.

Both `DeclareInputs` and `DeclareOutputs` are enforced — they are the subgraph's contract surface. The child graph's reducers govern the input merge only; the parent graph's reducers govern the output merge only.

### Transitions are rules evaluated per fan-out, not graph edges

Multiple transitions can match from the same task. *All* matching ones fire — that is what makes fan-out implicit:

- Unconditional + unconditional = parallel branches.
- Two `When` transitions with non-mutually-exclusive predicates = parallel branches (the framework does not enforce exclusivity).
- `ForEach` spawns one branch per array element.
- `WithGoto` transitions are excluded from automatic evaluation; only taken when the task calls `flow.Goto(target)`.
- `OnError` transitions are taken when the source task returned an error; the error is serialized as a `TracedError` into the target task's state under the key `onErr`. The error-handler task otherwise sees the same state as any other task in the workflow and can also pull values via typed input args. Without an `OnError` transition, an error fails the flow.

`OnError` cannot combine with `When`, `ForEach`, or `WithGoto` — the validator rejects it.

### Fan-out siblings must converge to the same successors

`Graph.Validate` rejects graphs where two non-goto, non-error siblings of a fan-out have different outgoing transition targets. The reason is in the foreman's evaluator: when siblings execute at the same `step_num`, the workflow only continues to the next step when the last sibling finishes, and outgoing transitions are evaluated only from that last sibling. If siblings disagreed about where to go next, the chosen successor would depend on a race. Forcing a common successor set makes fan-in deterministic.

This is the most surprising graph constraint. If you find yourself wanting different successors per sibling, you usually want a conditional transition out of a *single* convergence node instead.

### `END` is a pseudo-node

`END` is the constant `"END"`. It is never registered in `nodes`. Transitions targeting `END` mark a terminal path; `Validate` requires at least one. `AddTask(END)` is a no-op so transitions can be built up without special-casing the sink.

### Auto-registration on `AddTransition`

`AddTransition`, `AddTransitionWhen`, `AddTransitionGoto`, `AddTransitionForEach`, `AddErrorTransition` all auto-register both endpoints as tasks. The first node added — by any path — becomes the default `entryPoint` unless `SetEntryPoint` overrides. So a graph can be built from transitions alone, without explicit `AddTask` calls.

### `DeclareInputs` / `DeclareOutputs` use a three-state encoding

The same convention is used in both directions and in `FilterState`:

- `nil` or empty slice = pass nothing.
- `["*"]` = pass everything.
- Named fields = pass exactly those fields.

`Outputs` filtering applies whether the graph runs as a subgraph or as a root flow. This is what lets a workflow declare "my contract is just these output fields" and have the foreman strip the rest before returning to the caller.

### `RawFlow` is the orchestrator's API; tasks see only `Flow`

`Flow` exposes the read/write/control surface a task author needs. `RawFlow` embeds `Flow` and adds bulk-state mutation (`SetRawState`), control-signal clearing (`ClearControl`, `ClearChanges`), and attempt setting (`SetAttempt`). Only the foreman calls these — they're how a flow gets reconstituted from SQL between steps and how control signals are reset before the next dispatch.

The split is enforced by Go's package-private fields, not by an interface, because `RawFlow` needs to mutate `Flow`'s unexported state directly. This is why both types live in the same package and there is no `Flow` interface.

### `MarshalJSON` exists because of unexported fields

`Flow.MarshalJSON` and `UnmarshalJSON` round-trip through a private `flowJSON` struct because the wire format is the SQL-persisted representation, and the standard encoder cannot see `state`, `changes`, `gotoNext`, etc. Don't add a new field to `Flow` without updating `flowJSON` — silent drop on persistence is the failure mode.

### Empty `forEach` means "no successors via this transition"

`AddTransitionForEach` over an empty state field spawns zero tasks. If it is the *only* outgoing non-`OnError` transition from the source, the flow completes at the source. This is the correct behavior for "fan out over results, none found" cases, but it is also a foot-gun if you forgot a sibling unconditional transition to a default target.

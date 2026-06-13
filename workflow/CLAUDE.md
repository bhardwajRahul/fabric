## Design Rationale

### State is JSON, not Go values

Every entry in `Flow.state` and `Flow.changes` is a `json.RawMessage`, not a native Go value. `Set`, `SetString`, `SetInt`, etc. all marshal to JSON before storing. There are two reasons:

- The foreman persists state to SQL and ships it across replicas at every step. Native Go values would force the foreman to know every task's types.
- `SetChanges` compares against a snapshot by JSON byte-equality. Storing native values would make the diff implementation-defined (e.g., struct field ordering, unexported fields).

This is also why reducers (`Append`, `Add`, `Union`, `Merge`) marshal/unmarshal at every step rather than working in Go: the input may already be `json.RawMessage`, and `union` dedupes by raw JSON string - so it is sensitive to whitespace and number formatting. Values produced through Go's `encoding/json` are deterministic and dedupe consistently; hand-built `json.RawMessage` is the danger zone.

### Reducer selection is explicit; defaults are Replace

Every fan-in state field that needs anything other than last-write-wins must be wired with `graph.SetReducer(field, reducer)` at graph-build time. Fields without a registered reducer use `Replace` (last write wins). There is no name-driven inference — a field named `messages` and a field named `total` are treated identically until `SetReducer` says otherwise.

This is a deliberate departure from an earlier prefix-based convention (`sum*`, `list*`, `set*`, etc.). The inference saved one line of wiring but at the cost of (a) state field names carrying hidden execution semantics, (b) those names then leaking into task argument lists, struct types, JSON tags, OpenAPI surfaces, and LLM tool definitions, and (c) a long list of "reserved prefix" gotchas an author had to know to avoid accidentally turning a harmless `listFiles` field into a silent fan-in accumulator. Making the reducer explicit costs one `SetReducer` call per non-default field and removes the entire foot-gun class.

```go
graph.SetReducer("messages", workflow.ReducerAppend)  // accumulate per-branch deltas
graph.SetReducer("total",    workflow.ReducerAdd)     // sum numeric contributions
graph.SetReducer("lowScore", workflow.ReducerMin)     // smallest contribution wins
graph.SetReducer("topScore", workflow.ReducerMax)     // largest contribution wins
graph.SetReducer("seen",     workflow.ReducerUnion)   // dedupe across branches
graph.SetReducer("approved", workflow.ReducerAnd)     // all branches must approve
graph.SetReducer("flagged",  workflow.ReducerOr)      // any branch flags
graph.SetReducer("notes",    workflow.ReducerConcat)  // join string deltas
graph.SetReducer("attrs",    workflow.ReducerMerge)   // merge per-branch objects
```

**Empty-cohort identity.** A `forEach` that spawns zero branches still fires the fan-in. The field takes the reducer's identity — the Go zero value for the underlying type:

| Reducer | Identity | Type |
|---|---|---|
| `Add` | `0` | number |
| `Append` | `[]` | array |
| `Union` | `[]` | array |
| `Merge` | `{}` | object |
| `And` | `false` | bool |
| `Or` | `false` | bool |
| `Concat` | `""` | string |

Both And and Or return `false` on an empty cohort. There is no "vacuously true" mode — if you need an empty-cohort to be treated as approval, branch on the cohort size downstream rather than relying on the reducer.

`Min` and `Max` have no algebraic identity (0 is a legitimate value, not a neutral one), so they fold cleared slots through asymmetrically: a cleared side defers to the other side rather than collapsing to 0. An all-cleared empty cohort therefore leaves the field absent on the fan-in step (no folded contributions). A workflow that needs a defined value when the cohort is empty must seed the field upstream of the fan-out.

**Order sensitivity.** `Append` (arrays) and `Concat` (strings) preserve the cohort merge order, which is `updated_at, step_id` — not creation order under shard scatter. If you need a specific ordering, sort downstream rather than relying on fan-in order.

**Type strictness.** Reducers are strict on their input type. `ReducerUnion` rejects object values; `ReducerMerge` rejects array values; `ReducerAdd`/`ReducerMin`/`ReducerMax` reject strings; etc. A cleared slot (Go nil or JSON null) short-circuits to the reducer's identity rather than failing the type check, so `flow.Clear` contributions are ignored at fan-in.

### `state` and `changes` are tracked separately on purpose

A `Flow` carries two maps: `state` (everything the task can read) and `changes` (only what *this* step wrote). The foreman persists `changes` per step as the audit trail and merges them via reducers at fan-in points. Without the split, fan-in could not distinguish "this branch wrote X" from "this branch read X."

`SetChanges(source, snapshot)` only records fields whose JSON differs from the snapshot, so a parse-modify-write-back loop (`ParseState` → mutate struct → `SetChanges`) does not pollute the change set with untouched fields.

`SetState` and `SetRawState` write to `state` *without* recording changes - these are orchestrator entry points (the foreman uses them when materializing a flow from SQL), not for task code.

### Generic `Set` is the contract; typed setters are convenience

`Set(key, value)` returns an error and is the full API. The typed variants (`SetString`, `SetInt`, ..., `SetDuration`) panic on marshal failure rather than return an error. This is deliberate - primitive types never fail to marshal, so propagating an error at every call site would be noise. If a typed setter ever did panic, the connector's request handler wraps task invocations in `errors.CatchPanic` (see `connector/subscribe.go`), so the panic surfaces as a normal error response to the foreman rather than crashing the microservice.

### Control signals are deferred requests, not actions

`Goto`, `Retry`, `Sleep`, `Interrupt`, `Subgraph` do *not* alter execution mid-task. They set a field on `Flow` and the task continues. The task should `return nil` immediately after - the control signal *is* the result. The foreman reads the signal after the task returns and acts on it.

This is why a task can call `flow.Interrupt(payload)` and, when it yields, `return nil` to pause the flow: the
return is what hands control back to the foreman.

### `Retry` returns a bool the caller must branch on

`Retry(maxAttempts, ...)` returns `true` while attempts remain, `false` once `maxAttempts` is reached. Call it inside an error branch:

```go
result, err := callExternalAPI(ctx)
if err != nil {
    if flow.Retry(5, time.Second, 2.0, 30*time.Second) {
        return result, nil   // retry scheduled, suppress the error
    }
    return result, err       // exhausted, surface the error
}
```

`Retry` is the **single** retry primitive - there are no `RetryOnError`/`RetryOnTimeout`/`RetryNow` variants. It carries no condition of its own, so the task writes the retryable condition explicitly in the surrounding `if`. This is deliberate: a condition baked into a method name (especially "retry on any error") makes it too easy to retry faults that should never be retried (validation, 4xx, business rejections); keeping it explicit forces the author to name *which* errors are transient. To retry only on a timeout, gate on the 408 status - both a foreman-side `pub.Timeout` expiry and a subscriber-side handler timeout surface as `http.StatusRequestTimeout`:

```go
result, err := callExternalAPI(ctx)
if err != nil {
    if errors.StatusCode(err) == http.StatusRequestTimeout && flow.Retry(5, time.Second, 2.0, 30*time.Second) {
        return result, nil
    }
    return result, err
}
```

Immediate retry is zero delays; unlimited is `math.MaxInt32` as `maxAttempts` - there is deliberately no unlimited-retry verb, matching the foreman's "flow lifetime is the author's responsibility" stance (prefer a bounded cap plus an `OnError`/`OnTimeout` give-up transition). A retry also clears the step's park slot, so retrying after a resolved `Subgraph`/`Interrupt` re-runs the side-trip.

The bool exists so the foreman can tell the task when retries have been spent. On `false`, the task surfaces the error and the foreman takes whatever path it would normally take for a failed task (`OnError` transition or flow failure). The attempt counter is carried in `Flow.attempt` and incremented by the foreman before each redispatch.

### `Interrupt` parks and returns the resume data on re-entry

`flow.Interrupt(payload)` returns `(resumeData, yield, err)`. On the first call it arms the interrupt park and
returns `yield=true`; the task must `return nil` immediately. The foreman parks the flow (`interrupted`) and, when
`Resume(flowKey, data)` arrives, records `data` on the step's `resume_data` column and sets `interrupt_done=1`. On
re-dispatch the same `flow.Interrupt(payload)` call sees `interrupt_done=1`, returns `(data, false, nil)`, and does
*not* re-arm, so the task proceeds. The idiom:

```go
resumeData, yield, err := flow.Interrupt(map[string]any{"request": "userInput"})
if yield {
    return nil // parked, awaiting Resume
}
// proceed with resumeData
```

The re-entry signal is the framework-managed `interrupt_done` flag, not a value the task encodes in state - resume
data is delivered as the return value, not merged into state by field name. `Subgraph` follows the same shape:
`flow.Subgraph(url, input)` returns `(out, yield, err)`, parks on the first call, and returns the child's
`final_state` as `out` on re-entry (re-entry detected by the framework's `subgraph_done` flag).

### A step parks at most once - guarded in the Flow, enforced by the foreman

A step may arm **one** park, interrupt XOR subgraph, never both and never the other kind on re-entry. Sequential
side-trips are sequential graph tasks by design (this keeps the only re-run-on-redispatch surface small and avoids
Temporal-style whole-task replay determinism). The guard lives in two places, deliberately:

- **In the `Flow`, at the call site.** Each parker rejects a conflicting second park synchronously, returning a
  clear error in its `err` return: `Interrupt` errors if a subgraph is already armed this dispatch
  (`subgraphWorkflow` set) or already resolved on this step (`subgraphDone`); `Subgraph` errors symmetrically on
  `interrupt`/`interruptDone`. A same-kind re-arm cannot reach the guard - a resolved parker short-circuits to its
  cached value first - so the guard only fires on the cross-kind case. This is the friendly, immediate signal a task
  author sees: `if err != nil { return err }` surfaces it at once.
- **In the foreman, after the task returns.** The foreman re-checks the returned flow's signals against the step
  row's `_done` flags (see `coreservices/foreman/CLAUDE.md`). The returned `Flow` is untrusted JSON: a task that
  bypasses the parker methods (hand-built response body, raw setters) could present both signals set, and the
  foreman must not act on a self-inconsistent flow. The in-`Flow` guard is good ergonomics; the foreman guard is the
  trust boundary.

### Subgraph state-passing semantics

Subgraphs are invoked one way: a regular task calls `flow.Subgraph(url, input)`. There is no static subgraph node
type - `graph.AddSubgraph` was retired in v1.37.0, so the graph definition never models a child workflow directly; a
subgraph call is invisible to the parent graph (it is just a task that happens to delegate). The wire-up is done by
the foreman, not by the workflow package itself, but the rules are worth knowing because they shape what a task can
rely on:

- **Into the child:** The child's initial state is exactly the explicit `input` map passed to `flow.Subgraph` -
  nothing more. The parent's state and accumulated changes are NOT inherited. A `nil` input means "no arguments"
  (empty state). To pass the parent's full state, the caller passes `flow.Snapshot()` (or a derived map) as input.
  Subgraph is a function call, and `input` is the argument list.
- **Back to the parent:** The child's `final_state` is *returned to the calling task* as `out`, not merged into the
  parent's state. The task adopts the fields it wants (e.g. `flow.Set(...)`, or returns them as its own typed
  outputs). This is the "scope the state after the subgraph" contract - no parent-reducer merge, no downstream
  adapter needed for the common case.

State hygiene at subgraph boundaries is the workflow author's responsibility, but the function-call shape means
hygiene is now a matter of choosing what to put in `input` and what to read from `out`, not of building adapter
tasks to scrub the parent's state. The `flow.Snapshot`/`Transform`/`Delete`/`Clear` primitives help when the
caller wants to pass a *derived* view of parent state (snapshot, then mutate the local copy). The typed
`MyWorkflowIn`/`MyWorkflowOut` structs declared via `sub.Workflow` are documentation and OpenAPI surface, not
runtime contracts.

### Switch transitions are first-match-wins routing, not fan-out

`AddTransitionSwitch(from, to, when)` is the routing primitive: the foreman walks a source's `Switch` transitions in registration order and follows the **first** whose `when` matches. The rest are skipped on that step. The last entry is typically `when="true"` for a default branch; if no `Switch` matches, no successor is created and the flow ends at that node via the normal "no next steps" path.

The design decisions that make `Switch` worth having as a separate construct from `When`:

- **First-match wins is a different semantic from `When`.** `AddTransitionWhen` is a fan-out primitive in disguise: every matching predicate fires, in parallel, and the validator demands a `SetFanIn` downstream to merge the branches. That's the right shape when you genuinely want multiple branches to run, the wrong shape when you want exactly one. Two `When` predicates intended to be mutually exclusive (`x>0` and `x<=0`) compile fine and quietly become a 2-branch fan-out at runtime if both ever match - a footgun the author has to defend against by hand. `Switch` collapses that into one declarative ladder where the validator does the defending.
- **No `SetFanIn` needed.** Because at most one Switch branch ever runs per step, a Switch node is not a fan-out source in `IsFanOutSource`'s sense. The lineage validator skips it (no frame pushed; no pop expected at a downstream `SetFanIn`). A graph that uses Switch for every branching point can be written without ever calling `SetFanIn`.
- **Validator enforces all-or-nothing on the source.** A node that uses Switch must use Switch for every success-path outgoing transition. Mixing Switch with `When`/plain/`ForEach`/`Goto` from the same source is rejected because the resulting semantics would be ill-defined (does the plain transition fire alongside the chosen Switch? Before? After?). `OnError`/`OnTimeout` are orthogonal (error axis) and stay allowed.
- **`When` is required.** A Switch transition with an empty `when` is rejected by the validator. The default branch uses `when="true"`, written explicitly, to keep the diagram self-documenting (an unlabeled fallthrough arm reads ambiguously in Mermaid).

The Mermaid renderer labels Switch edges as `switch <expr>` so the predicate appears on each arm. The `Switch` field on the `Transition` struct rides through JSON round-trips unchanged; no other field combinations are valid alongside it.

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

`OnError` and `OnTimeout` are for routing to a *different* handler (a fallback model, a degraded path, an error-logging task). The validator rejects `from == to` on either kind, because a self-loop bypasses every safety property of `flow.Retry`: it creates a new step row per cycle (instead of reusing one), it has no attempt counter, no backoff, and no bound. For "keep retrying this task" semantics, use `flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay)` in the task body, gating it on whatever error condition the task deems retryable (e.g. `errors.StatusCode(err) == http.StatusRequestTimeout` for timeouts only). The two primitives compose: the task retries internally until exhausted, then the surviving error propagates and the graph's `OnTimeout`/`OnError` transition (to a *different* target) takes over.

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

### `RawFlow` is the orchestrator's API; tasks see only `Flow`

`Flow` exposes the read/write/control surface a task author needs. `RawFlow` embeds `Flow` and adds bulk-state mutation (`SetRawState`), control-signal clearing (`ClearControl`, `ClearChanges`), and attempt setting (`SetAttempt`). Only the foreman calls these - they're how a flow gets reconstituted from SQL between steps and how control signals are reset before the next dispatch.

The split is enforced by Go's package-private fields, not by an interface, because `RawFlow` needs to mutate `Flow`'s unexported state directly. This is why both types live in the same package and there is no `Flow` interface.

### `MarshalJSON` exists because of unexported fields

`Flow.MarshalJSON` and `UnmarshalJSON` round-trip through a private `flowJSON` struct because the wire format is the SQL-persisted representation, and the standard encoder cannot see `state`, `changes`, `gotoNext`, etc. Don't add a new field to `Flow` without updating `flowJSON` - silent drop on persistence is the failure mode.

### Empty `forEach` means "no successors via this transition"

`AddTransitionForEach` over an empty state field spawns zero tasks.

Under the legacy validator, if it is the *only* outgoing non-`OnError` transition from the source, the flow completes at the source. This is the correct behavior for "fan out over results, none found" cases, but it is also a foot-gun if you forgot a sibling unconditional transition to a default target.

Under the lineage validator (with `SetFanIn`), an empty forEach is not a footgun: the foreman looks up the source's fan-in via `g.FanInFor(source)` and creates that fan-in step directly with empty state. Execution continues past the join naturally.

### `FlowOptions` lives here, not in `foremanapi`

`FlowOptions` (priority, fairness key, fairness weight) is a flow-level scheduling concept - priority is a
property of the execution, not of any task or graph. It lives in the `workflow` package rather than
`foremanapi` for a concrete reason: it is referenced by `foremanapi` (`CreateIn`/`RunIn`), by the foreman
itself, by the generated `WorkflowRunner` interface, and by every workflow-bearing `*api/client.go`'s
Executor. `foremanapi` already imports `workflow`, and every workflow `*api` package already imports
`workflow` for `workflow.Flow`, so placing it here reaches every consumer with zero new dependencies and
without making 40-plus api packages depend on the foreman. The struct is pure data with JSON tags (it rides
the wire inside `CreateIn`/`RunIn`); it has no methods and the `workflow` package never interprets it - the
foreman owns all defaulting and resolution (see `coreservices/foreman/CLAUDE.md`). A `nil *FlowOptions` or
any zero field means "use the foreman default", which is why callers that don't care pass `nil`.

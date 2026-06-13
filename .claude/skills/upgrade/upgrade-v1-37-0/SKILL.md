---
name: upgrade-v1-37-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.36.x to v1.37.0. Four groups of breaking changes land together. (1) Workflow park/control signals: graph.AddSubgraph (the static subgraph node) is removed, so every static subgraph becomes a task that calls flow.Subgraph dynamically; flow.Interrupt and flow.Subgraph now return (data, yield, err) and no longer merge results into state by field name (silent break); flow.Subgraph's input is now a function-call argument list and the parent's state no longer auto-crosses into the child (silent break - pass flow.Snapshot() for back-compat); flow.RetryNow and flow.RetryNowOnTimeout are removed. (2) Reducer inference: the sum*/list*/set* fan-in prefixes no longer pick a reducer, so each prefix-named fan-in field needs an explicit graph.SetReducer. (3) Foreman public API: foremanapi.Fork is removed; foremanapi.Retry is replaced by foremanapi.Restart (whole-flow) and foremanapi.RestartFrom (subtree from a step), both accepting state overrides. (4) Other framework changes: the TESTING deployment no longer reads config.yaml / config.local.yaml at startup, and the llmapi.Turn output gained a StopReason field that downstream LLM providers must return.
---

## What changed

v1.37.0 bundles four groups of breaking changes. Migrate the **loud** compile errors first (the project will
not build until they are fixed), then the **silent** behavior changes, then the reducer wiring.

### Group A: workflow park / control signals

- **`graph.AddSubgraph` is removed (loud).** The static subgraph node type is gone. A child workflow is no
  longer a declared node in the parent graph; it is invoked at runtime by a regular task calling
  `flow.Subgraph(url, input)`. `graph.IsSubgraph` and the `Node.Subgraph` field are also gone.
- **`flow.Interrupt` and `flow.Subgraph` now return `(data, yield, err)` (silent).** A bare call statement
  still compiles, so the compiler will not catch this. Both parkers used to mutate parent state behind the
  task's back: `Resume` merged the resume payload into the leaf step's `state`, and a completed subgraph
  merged its `final_state` into the parent step's `changes`. **That auto-merge is gone.** Resume data is
  now delivered only through `flow.Interrupt`'s return value; the child's output is delivered only through
  `flow.Subgraph`'s return value. A task that called either bare and then read the result back from state
  by field name will silently see stale or zero values.
- **`flow.Subgraph` is now a function call: only the explicit `input` map crosses into the child (silent).**
  In v1.36.x the child's initial state was `parent_state + parent_changes + input` — the parent's full
  state inherited automatically and the input map overlaid on top. In v1.37.0 the child's initial state is
  *exactly* `input` — nothing else crosses. A `nil` input means "no arguments" (the child starts with empty
  state). Silent break: a child that read a parent field by name and wasn't passed it explicitly will
  silently see the zero value. The mechanical back-compat rewrite is to pass `flow.Snapshot()` as input
  (the snapshot is the parent's current state, so the child sees what it used to see); the preferred
  long-term form is a named-fields `map[string]any{...}` listing exactly what the child reads.
- **`flow.RetryNow`, `flow.RetryNowOnTimeout`, and `flow.RetryOnTimeout` are removed (loud).** `flow.Retry` is
  now the single retry primitive - it carries no condition, so the retryable condition is written explicitly in
  the surrounding `if`. The bare `flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay)` signature is
  unchanged (no break); only the conditional/immediate variants are gone.
- **A retry now clears the step's park slot.** Calling `flow.Retry` after a resolved `flow.Subgraph` re-runs
  a fresh child; after a resolved `flow.Interrupt` it re-arms the interrupt. This is new capability, not a
  break, but it is why the `if yield { return }` guard belongs *before* any retry logic.

### Group B: reducer inference removed

The workflow framework no longer infers the fan-in reducer from a state field's name prefix. The `sum*`,
`list*`, `set*` convention (with the uppercase-boundary rule that made `sumScore`, `listMessages`,
`setUsers` load-bearing) is gone. The polymorphic `set*` dispatch to `ReducerUnion`/`ReducerMerge` based on
value kind is also gone. Fields without a registered reducer use `ReducerReplace` (last write wins), which
is silently wrong for previously-inferred fields.

`workflow.ReducerForFieldName` has been removed from the public API. Three reducers were also introduced
(`ReducerAnd`, `ReducerOr`, `ReducerConcat`); they only apply when wired via `graph.SetReducer`. The
empty-cohort identity for `ReducerAnd` is `false` (Go zero value), not `true`.

The framework's own `coreservices/llm.ChatLoop` workflow renamed its conversation-history state field from
`listMessages` to `messages` (with explicit `graph.SetReducer("messages", workflow.ReducerAppend)`).
Anything that composes `ChatLoop` as a subgraph and was bridging through `listMessages` will silently
desync.

### Group C: foreman public API

- **`foremanapi.Fork` is removed (loud).** The endpoint, its `ForkIn`/`ForkOut` types, and the `Fork`
  method on `foremanapi.Client` are gone. Fork created a new flow from an existing flow's step
  checkpoint as an independent investigation thread. The replacement for the "rewind to a step and
  re-run with overrides" use case is `foremanapi.RestartFrom`, which mutates the existing flow
  in place rather than creating a sibling. See Step 8.
- **`foremanapi.Retry` is replaced by `foremanapi.Restart` and `foremanapi.RestartFrom` (loud).** The
  old `Retry(flowKey)` endpoint - which re-executed the last failed step - is gone. Two new endpoints
  take its place:
  - `Restart(flowKey, stateOverrides)` wipes everything past the flow's entry step and re-runs from the
    entry with the overrides merged in.
  - `RestartFrom(stepKey, stateOverrides)` sweeps the DAG subtree below a chosen step and resets that
    step with overrides, re-running from there.
  Both accept a `stateOverrides any` argument (pass `nil` for no overrides) - they are operator/debug
  tools, not the in-task retry primitive. The in-task `flow.Retry(...)` from Group A is unchanged.

### Group D: other framework changes

- **The `TESTING` deployment no longer reads `config.yaml` / `config.local.yaml` (silent).** In
  v1.36.x the framework loaded the project's config files even in `TESTING` mode before falling back
  to defaults. In v1.37.0 the file is skipped entirely in `TESTING`; tests see only the declared
  defaults plus anything explicitly set via `Set` / `SetConfig` on the connector or via the test
  harness. A test that depended on a value living in `config.yaml` will silently see the default.
  Move those values into the test code that sets them, or call `svc.SetConfig(name, value)` before
  `app.RunInTest(t)`.
- **`llmapi.TurnOut` gained a `StopReason` field (loud for LLM providers).** Microservices that
  implement an LLM provider (a `Turn` endpoint matching the `llmapi.Turn` signature) must add
  `stopReason string` to the return list before `usage`. Pure consumers of LLMs (anything calling
  `llmapi.NewClient(svc).Chat(...)` or composing `ChatLoop`) need no change; the framework reads
  `StopReason` itself and surfaces it on `TurnOut`. Return one of the
  `llmapi.StopReason*` constants (`StopReasonEndTurn`, `StopReasonToolUse`, `StopReasonMaxTokens`,
  `StopReasonStopSequence`, `StopReasonRefusal`, `StopReasonPauseTurn`) or `""` when the provider
  cannot infer a reason.

## Workflow

```
Upgrade a Microbus project to v1.37.0:
- [ ] Step 1: Fix the loud RetryNow / RetryNowOnTimeout removals
- [ ] Step 2: Convert AddSubgraph nodes to flow.Subgraph caller tasks
- [ ] Step 3: Rewrite flow.Interrupt call sites to capture (resumeData, yield, err)
- [ ] Step 4: Rewrite flow.Subgraph call sites to capture (out, yield, err)
- [ ] Step 5: Add a graph.SetReducer call per prefix-named fan-in field
- [ ] Step 6: Fix ChatLoop subgraph callers (listMessages -> messages)
- [ ] Step 7: Remove ReducerForFieldName references in any custom code
- [ ] Step 8: Migrate foremanapi.Retry callers to Restart / RestartFrom; replace Fork callers with RestartFrom or remove
- [ ] Step 9: Add stopReason to any LLM provider's Turn signature
- [ ] Step 10: Move TESTING-mode config values out of config.yaml into the test setup
- [ ] Step 11: Regenerate mocks + manifests, then go vet ./... && go test ./...
```

#### Step 1: Fix the Loud Retry-Variant Removals

`RetryNow`, `RetryNowOnTimeout`, and `RetryOnTimeout` are gone; only the bare `flow.Retry` remains. These are
compile errors, so the build finds them, but grep first to size the work:

```bash
grep -rn '\.RetryNow(\|\.RetryNowOnTimeout(\|\.RetryOnTimeout(' --include='*.go' .
```

Rewrite each. The timeout variants gain an explicit 408 check (`errors.StatusCode(err) == http.StatusRequestTimeout`,
which needs the `net/http` import); the immediate ones gain an unlimited `math.MaxInt32` cap:

| Old call                                       | Replacement                                                                          |
|------------------------------------------------|--------------------------------------------------------------------------------------|
| `flow.RetryNow()`                              | `flow.Retry(math.MaxInt32, 0, 0, 0)`                                                 |
| `flow.RetryOnTimeout(err, n, d, m, x)`         | `errors.StatusCode(err) == http.StatusRequestTimeout && flow.Retry(n, d, m, x)`      |
| `flow.RetryNowOnTimeout(err)`                  | `errors.StatusCode(err) == http.StatusRequestTimeout && flow.Retry(math.MaxInt32, 0, 0, 0)` |

Fold the rewritten expression into the enclosing `if`, e.g.:

```go
result, err := svc.doWork(ctx)
if err != nil {
    if errors.StatusCode(err) == http.StatusRequestTimeout && flow.Retry(5, 2*time.Second, 2.0, time.Minute) {
        return result, nil // transient timeout: retry scheduled
    }
    return result, err // non-timeout, or attempts exhausted
}
```

`math.MaxInt32` is the explicit "effectively unlimited" cap; there is deliberately no unlimited-retry verb.
If the call site has its own stop condition (most do), a bounded cap is preferable to `math.MaxInt32` -
prefer a finite number plus an `OnError`/`OnTimeout` give-up transition over an unbounded loop. Do not
touch bare `flow.Retry(...)` calls; that signature is unchanged.

#### Step 2: Convert `AddSubgraph` Nodes to `flow.Subgraph` Caller Tasks

`graph.AddSubgraph` is a compile error. Find every static subgraph node:

```bash
grep -rn '\.AddSubgraph(\|\.IsSubgraph(' --include='*.go' .
```

For each `graph.AddSubgraph("nodeName", childWorkflowURL)`, replace the node with a regular task that calls
the child workflow dynamically. The migration is mechanical:

1. **Scaffold a caller task** with the `add-task` skill. Name it after the node it replaces (e.g. a node
   `identityVerification` becomes a task `RunIdentityVerification`). Its inputs are the parent state
   fields the child reads (so the caller can pass them explicitly to `flow.Subgraph`), and its outputs
   are exactly the child fields the parent needs downstream. For a mechanical AddSubgraph rewrite where
   you don't yet want to enumerate the child's dependencies, leave the inputs empty and pass
   `flow.Snapshot()` as the subgraph input — see boundary notes below.
2. **Write the caller body** using the park/re-entry idiom. Mechanical back-compat form (preserves the
   v1.36 "child inherits parent state" behavior via `flow.Snapshot()`):

   ```go
   func (svc *Service) RunChild(ctx context.Context, flow *workflow.Flow) (childResult string, err error) { // MARKER: RunChild
       out, yield, err := flow.Subgraph(childapi.Child.URL(), flow.Snapshot())
       if err != nil {
           return "", errors.Trace(err)
       }
       if yield {
           return "", nil // first pass: parked, child workflow running
       }
       // resolved: adopt exactly the child outputs the parent wants
       if v, ok := out["childResult"].(string); ok {
           return v, nil
       }
       return "", nil
   }
   ```

   Preferred form once you know what the child reads — pass a named-fields map as the input contract,
   and take the same fields as typed arguments on the caller so they auto-bind from parent state:

   ```go
   func (svc *Service) RunChild(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string) (childResult string, err error) { // MARKER: RunChild
       out, yield, err := flow.Subgraph(childapi.Child.URL(), map[string]any{
           "applicantName": applicantName,
           "ssn":           ssn,
       })
       // ... same yield / err / adopt-out shape as above
   }
   ```

3. **Rewire the graph**: replace the node registration and keep the node name so the transitions are
   unchanged.

   ```go
   // before
   graph.AddSubgraph("child", childapi.Child.URL())
   // after
   graph.AddTask("child", childapi.RunChild.URL())
   ```

State boundary notes that make the migration correct:

- **Into the child:** `flow.Subgraph` is a function call. The child's initial state is *exactly* the
  `input` map you pass; the parent's state does NOT cross automatically. Pass `flow.Snapshot()` for the
  mechanical "inherit everything" rewrite from v1.36, or a named-fields `map[string]any{...}` for the
  explicit form (preferred once you know the contract).
- **Back to the parent:** the child's `final_state` is returned as `out` and is **not** merged into
  parent state. The caller adopts what it wants - either returning fields as its own typed outputs (as
  above) or calling `flow.Set(...)`. This is the boundary payoff: the parent scopes exactly what crosses
  back, just as `input` scoped exactly what crossed in.
- **JSON types:** `out` values are JSON-decoded, so numbers are `float64`, not `int`. Use
  `out["n"].(float64)` then convert.

For a **parallel subgraph fan-out** (a fan-out where several siblings were subgraph nodes), give each its
own caller task. If the fan-out iterated a runtime array, use a `forEach` transition into a single caller
task that calls `flow.Subgraph` once per element; if it was a fixed set of distinct siblings, keep the
static fan-out and replace each subgraph node with its own caller task.

#### Step 3: Rewrite `flow.Interrupt` Call Sites to Capture `(resumeData, yield, err)`

This is a **silent** break - bare `flow.Interrupt(payload)` still compiles. Find every call:

```bash
grep -rn '\.Interrupt(' --include='*.go' .
```

The old idiom called `flow.Interrupt(payload)`, returned `nil`, and the *resumed* task re-read the resume
data from state by field name (because `Resume` merged it in). The new idiom captures the return value and
reads the resume data from it. Resume data is no longer in state.

```go
// before (v1.36.x): payload sent, resume data later read from state by field name
flow.Interrupt(map[string]any{"request": "userInput"})
return nil
// ... and elsewhere the resumed task did: userInput := flow.GetString("userInput")

// after (v1.37.0)
resumeData, yield, err := flow.Interrupt(map[string]any{"request": "userInput"})
if err != nil {
    return errors.Trace(err)
}
if yield {
    return nil // parked, awaiting Resume
}
userInput, _ := resumeData["userInput"].(string)
// proceed with userInput
```

The single task now both arms the interrupt and consumes the resume data on re-entry; the
`if yield { return }` guard separates the two passes. Any exactly-once side effect (firing an event,
charging a card) must go *after* the guard, because the before-guard region runs on every dispatch.

#### Step 4: Rewrite `flow.Subgraph` Call Sites to Capture `(out, yield, err)`

Also a **silent** break. This covers both the nodes you just created in Step 2 (already correct) and any
*pre-existing* dynamic `flow.Subgraph` calls the project already had. Find them:

```bash
grep -rn '\.Subgraph(' --include='*.go' .
```

The old idiom relied on the child's `final_state` being auto-merged into the parent and re-read by field
name (often with an `*Out` re-entry sentinel). The new idiom captures `out` and adopts explicitly:

```go
// before (v1.36.x): child output auto-merged into parent state, re-entry detected by a child-set field
if !innerDone {
    flow.Subgraph(childapi.Child.URL(), map[string]any{"value": value})
    return "", nil
}
return fmt.Sprintf("parent:%d", innerResult), nil  // innerResult read from merged state

// after (v1.37.0)
out, yield, err := flow.Subgraph(childapi.Child.URL(), map[string]any{"value": value})
if err != nil {
    return "", errors.Trace(err)
}
if yield {
    return "", nil
}
result := 0
if v, ok := out["innerResult"].(float64); ok {
    result = int(v)
}
return fmt.Sprintf("parent:%d", result), nil
```

The framework no longer tracks re-entry via a child-set state field; the `subgraph_done` flag does it, and
`yield` reports it. Delete any `*Out` re-entry sentinel the old code carried for this purpose.

**Subgraph failure now surfaces as `err`, not a cascaded flow failure.** A child that fails delivers its
error to the calling task as the `err` return. Returning that error reproduces the old behavior (the flow
fails or routes via `OnError`); the new capability is that the task may instead `flow.Retry` (which re-runs
the child) or route. A common shape:

```go
out, yield, err := flow.Subgraph(childapi.Child.URL(), flow.Snapshot()) // or an explicit map
if yield {
    return nil
}
if err != nil {
    if flow.Retry(5, time.Second, 2.0, 30*time.Second) {
        return nil // retry re-runs the child
    }
    return errors.Trace(err) // exhausted: fail / OnError
}
// adopt out
```

**Subgraph input is now a function-call argument list, not an overlay on inherited parent state.** In
v1.36.x the child's initial state was `parent_state + parent_changes + input` (full state inheritance). In
v1.37.0 the child's initial state is exactly `input` - nothing else crosses. A `nil` input means "no
arguments" (empty state). This is a **silent** break: a child task that read a parent field by name (and
wasn't passed it explicitly) silently sees the zero value.

The safe back-compat rewrite is to pass `flow.Snapshot()` as input - the snapshot is the parent's current
state, so the child sees what it used to see. Find every `flow.Subgraph(url, nil)` call site:

```bash
grep -rn '\.Subgraph(.*,\s*nil)' --include='*.go' .
```

Rewrite to either preserve the old inheriting behavior:

```go
// safe back-compat: pass everything the parent has
out, yield, err := flow.Subgraph(childapi.Child.URL(), flow.Snapshot())
```

Or, preferred when you know what the child actually reads, switch to an explicit argument list:

```go
// preferred: explicit arguments, like any function call
out, yield, err := flow.Subgraph(childapi.Child.URL(), map[string]any{
    "applicantName": applicantName,
    "ssn":           ssn,
})
```

The explicit form is preferred; the `flow.Snapshot()` form exists as the mechanical rewrite for cases where
you can't easily enumerate the dependencies.

Calls that already pass an explicit input map (`flow.Subgraph(url, map[string]any{...})`) need no change for
this rewrite - the explicit map is now the *complete* input, where before it was an overlay on top of
inherited state. If the child also depended on a field the parent had but the input map didn't list, add it
to the map.

#### Step 5: Add a `graph.SetReducer` Call Per Prefix-Named Fan-In Field

The manifest is the documented map of what each microservice exposes. Scan every `manifest.yaml` for task
signatures whose argument or return names start with `sum`, `list`, or `set` followed by an uppercase
letter (the original boundary rule). This catches every fan-in field that previously relied on inference.

```bash
grep -rEn '\b(sum|list|set)[A-Z][a-zA-Z0-9_]*' --include='manifest.yaml' .
```

Each hit identifies a microservice and one or more tasks. For each, open the microservice's `service.go`
and find the corresponding `*workflow.Graph`-returning function (the one wired by `MARKER: <WorkflowName>`)
that includes the task in its `AddTask(...)` block. For each affected workflow's graph builder, add one
`graph.SetReducer(name, reducer)` line after the `graph.SetFanIn(...)` calls and before the transition
wiring. **Keep the field name unchanged** - only the wiring is new.

| Old prefix | Old inferred reducer        | Replacement                                          |
|------------|-----------------------------|------------------------------------------------------|
| `sum*`     | `ReducerAdd`                | `graph.SetReducer("sumX", workflow.ReducerAdd)`      |
| `list*`    | `ReducerAppend`             | `graph.SetReducer("listX", workflow.ReducerAppend)`  |
| `set*`     | `ReducerUnion` (for arrays) | `graph.SetReducer("setX", workflow.ReducerUnion)`    |
| `set*`     | `ReducerMerge` (for objects)| `graph.SetReducer("setX", workflow.ReducerMerge)`    |

The `set*` prefix was polymorphic at runtime - it dispatched to Union for JSON arrays and to Merge for JSON
objects. To pick the right replacement, look at the Go type of the field where it is declared in the api
package's In/Out struct:

- Array / slice (`[]T`) -> `ReducerUnion`
- Object (`map[string]T`, a custom struct type, or `any` holding a JSON object) -> `ReducerMerge`
- Anything else -> stop and ask the workflow author; the field may not have been a fan-in accumulator at
  all, in which case no `SetReducer` is needed

Example (a graph with `sumTotal`, `listTags`, `setSeen` arriving at fan-in node `taskE`):

```go
graph.SetFanIn("taskE")
graph.SetReducer("sumTotal", workflow.ReducerAdd)
graph.SetReducer("listTags", workflow.ReducerAppend)
graph.SetReducer("setSeen",  workflow.ReducerUnion)
graph.AddTransition("taskA", "taskB")
// ...
```

Only fields that are actually written by parallel branches converging at a `SetFanIn` node need the
reducer. A prefix-named field that is only written by a single task on a sequential path needs no
`SetReducer` - on that path the behavior is identical to last-write-wins regardless of registered reducer,
so leaving it unregistered is correct. If unsure, add the `SetReducer` anyway; it is harmless for purely
sequential writes and correct for parallel ones.

Matches inside `mock.go` and `mock_test.go` are regenerated artifacts; do not edit them. Since this skill
deliberately *keeps* the existing field names, the `*api/` package and `intermediate.go` stay untouched.

#### Step 6: Fix ChatLoop Subgraph Callers (`listMessages` -> `messages`)

This step only applies if the project composes `coreservices/llm.ChatLoop` as a subgraph AND your parent
workflow's state carries a `listMessages` field that bridged through the boundary. Find these with:

```bash
grep -rln 'llmapi\.ChatLoop\.URL()' --include='*.go' .
```

The framework renamed its wire field from `listMessages` to `messages`. The child reads from `messages` and
writes back to `messages` - it no longer matches `listMessages` on either side. Two acceptable fixes; pick
one per call site.

**Option A: rename your parent's field to `messages`.** Mechanical across the parent service and its api
package, then regenerate, then add `graph.SetReducer("messages", workflow.ReducerAppend)` in the parent's
graph builder. Recommended if `listMessages` only lives inside this project's workflows.

```bash
perl -pi -e 's/listMessages\b/messages/g; s/ListMessages\b/Messages/g' \
    parent/service.go parent/intermediate.go parent/parentapi/*.go
```

**Option B: fold the field-name translation into the `flow.Subgraph` caller task** the parent already
gained in Step 2, keeping the parent's `listMessages` vocabulary. The caller reads `listMessages` out of
parent state, passes it across the boundary as `messages`, and writes the child's `messages` reply back
into `listMessages`. (Do not introduce dedicated `Before<NodeName>`/`After<NodeName>` adapter tasks - that
convention was retired in v1.37 along with the term "adapter task"; if your project has leftover ones from
a v1.36-era migration, collapse their work into ordinary upstream/downstream tasks the same way.)

```go
func (svc *Service) RunChatLoop(ctx context.Context, flow *workflow.Flow) (err error) {
    var msgs []llmapi.Message
    flow.Get("listMessages", &msgs)
    out, yield, err := flow.Subgraph(llmapi.ChatLoop.URL(), map[string]any{"messages": msgs})
    if err != nil {
        return errors.Trace(err)
    }
    if yield {
        return nil
    }
    var reply []llmapi.Message
    if raw, ok := out["messages"]; ok {
        b, _ := json.Marshal(raw)
        json.Unmarshal(b, &reply)
    }
    return flow.Set("listMessages", reply)
}
```

#### Step 7: Remove `ReducerForFieldName` References in Any Custom Code

`workflow.ReducerForFieldName` and `workflow.MergeState`'s prefix-inference branch are removed. Find and
replace each:

```bash
grep -rn 'workflow\.ReducerForFieldName\b' --include='*.go' .
```

If the call populated a reducer map at runtime, register the reducers directly via `graph.SetReducer` at
graph-build time instead.

#### Step 8: Migrate `foremanapi.Retry` / `foremanapi.Fork` Callers

`foremanapi.Retry`, `foremanapi.Fork`, the `RetryIn`/`RetryOut` and `ForkIn`/`ForkOut` types, and the
matching methods on `foremanapi.Client` are removed - so the build finds every caller. Grep first:

```bash
grep -rn 'foremanapi\.Retry\b\|foremanapi\.Fork\b\|\.Retry(\s*ctx\b\|\.Fork(\s*ctx\b' --include='*.go' .
```

Rewrite per call site:

| Old call                                           | Replacement                                                                                |
|----------------------------------------------------|--------------------------------------------------------------------------------------------|
| `foreman.Retry(ctx, flowKey)`                      | `foreman.Restart(ctx, flowKey, nil)` (no overrides) or `RestartFrom(ctx, failedStepKey, nil)` |
| `foreman.Fork(ctx, stepKey, overrides, opts)`      | `foreman.RestartFrom(ctx, stepKey, overrides)` — mutates the existing flow in place        |

A few rewrite notes:

- **`Retry` -> `Restart` vs `RestartFrom`.** The old `Retry` re-executed only the *last failed step*.
  `Restart(flowKey, nil)` re-runs the flow from its entry step (broader); `RestartFrom(stepKey, nil)`
  re-runs from a specific step (narrower, closer to the old semantic). When the goal is "retry the
  failure that just happened," fetch the failed step via `foreman.History` or `foreman.List` (status
  `failed`) and pass its step key to `RestartFrom`. When the goal is "run this flow again with new
  inputs," prefer `Restart` and pass the overrides map.
- **`Fork` -> `RestartFrom` is a semantic change, not a like-for-like rename.** `Fork` created an
  independent sibling flow; `RestartFrom` mutates the original. If the call site relied on the
  original flow staying intact (a debug fork that should not disturb the production run), there is
  no in-framework replacement - drop the call, or persist the snapshot externally and use it as
  `Restart` input on a new flow.
- **`FlowOptions` on Fork.** The `opts` argument to `Fork` is gone; scheduling priority/fairness on
  `RestartFrom` are inherited from the existing flow row.

Matches inside `mock.go` / `mock_test.go` are regenerated; do not hand-edit.

#### Step 9: Add `stopReason` to LLM Provider `Turn` Signatures

This step only applies if the project implements an LLM provider — a microservice whose `Turn`
endpoint signature matches `llmapi.Turn`. Pure consumers (anything calling `llmapi.NewClient(svc).Chat`
or composing `ChatLoop` as a subgraph) need no change.

Find candidate microservices:

```bash
grep -rln 'MARKER: Turn' --include='*.go' . \
    | xargs grep -l 'func .* Turn(ctx context.Context' 2>/dev/null
```

For each, the `Turn` function and its `TurnOut` need a `stopReason string` return added immediately
before `usage`, and every `return` statement updated to pass one of the `llmapi.StopReason*`
constants. The matching `<svc>api/endpoints.go` `TurnOut` struct gains a `StopReason` field of the
same JSON shape as `llmapi.TurnOut.StopReason`:

```go
// Before
func (svc *Service) Turn(ctx context.Context, ...) (content string, toolCalls []llmapi.ToolCall, usage llmapi.Usage, err error)

// After
func (svc *Service) Turn(ctx context.Context, ...) (content string, toolCalls []llmapi.ToolCall, stopReason string, usage llmapi.Usage, err error)
```

```go
// In <svc>api/endpoints.go
type TurnOut struct { // MARKER: Turn
    Content    string            `json:"content,omitzero"`
    ToolCalls  []llmapi.ToolCall `json:"toolCalls,omitzero"`
    StopReason string            `json:"stopReason,omitzero" jsonschema:"description=StopReason is the normalized reason the turn ended"`
    Usage      llmapi.Usage      `json:"usage,omitzero"`
}
```

Pick the constant by mapping the provider's native stop reason: `llmapi.StopReasonToolUse` when
returning non-empty `toolCalls`, `llmapi.StopReasonEndTurn` for an ordinary completion,
`StopReasonMaxTokens` / `StopReasonStopSequence` / `StopReasonRefusal` / `StopReasonPauseTurn` for
the corresponding provider events, and `""` (or omit) when the provider does not report a reason.
Regenerate mocks/manifests in Step 11. See `coreservices/claudellm/service.go` and
`coreservices/chatgptllm/service.go` for canonical examples.

#### Step 10: Move `TESTING`-mode Config Values Out of `config.yaml`

The framework no longer reads `config.yaml` / `config.local.yaml` when a connector is run in
`TESTING` deployment. A test that previously relied on a config-file value will silently see the
declared default; the failure mode is "the test reads a default it didn't expect" or "the test now
errors with a validation message that was previously masked by the file value."

Find candidate values:

```bash
grep -rn 'TESTING\b' --include='*_test.go' . | head -20
test -f config.local.yaml && cat config.local.yaml
test -f config.yaml && cat config.yaml
```

For each config property a test depends on, set it directly in the test setup instead of leaving it
in a file. Use whichever shape the connector exposes, typically `svc.SetConfig(name, value)` before
`app.RunInTest(t)`, or the harness's per-microservice config override hook:

```go
svc := myservice.NewService()
svc.SetConfig("MyKnob", "test-value")
app := application.New()
app.Add(svc)
app.RunInTest(t)
```

Production deployments (`PROD`, `LAB`, `LOCAL`) still read the files; only `TESTING` changed.
`config.yaml` entries that exist purely to back production behavior need no migration.

#### Step 11: Regenerate Mocks and Manifests, Then Verify

Caller tasks added in Step 2 and any renames in Step 6 change the `ToDo` interface and endpoint set, so
regenerate per microservice that you touched:

```bash
for d in $(find . -name "mock.go" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genmock --path "$d"
done
for d in $(find . -name "manifest.yaml" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genmanifest --path "$d"
done
```

`genmanifest` bumps `frameworkVersion` to `1.37.0` in each manifest it regenerates. Then from the project
root:

```bash
go vet ./...
go test ./...
```

The load-bearing assertions are the silent ones: a workflow test that previously saw a subgraph's output in
parent state, a resume that fed data back to an interrupted task, or a fan-in field summed across branches.
If a test now sees a zero value, an empty `out`, or a last-write-wins value, the corresponding rewrite from
Step 2-5 is missing at that call site.

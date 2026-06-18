---
name: upgrade-v1-40-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.39.x to v1.40.0. Breaking workflow API cleanup in the dwarf module ("names over URLs"), plus a new subflow client. (1) workflow.NewGraph dropped its url argument - NewGraph("Name", url) becomes NewGraph("Name") - the graph no longer carries a resolve URL; the resolve key is the separate URL still passed to the foreman's Run/Create. (2) graph.AddTask was renamed to graph.SetEndpoint with identical (name, url) arguments and create-or-update semantics. (3) FlowOutcome.FlowKey was removed - the flow key is identity, not outcome - so any code reading outcome.FlowKey must get the key elsewhere. (4) The foreman OnFlowStopped event/hook callback gained a leading flowKey argument: func(ctx, outcome) becomes func(ctx, flowKey string, outcome). (5) The foreman CreateTask endpoint/client gained a required leading name argument - CreateTask(taskURL, ...) becomes CreateTask(name, taskURL, ...) - a loud break for callers. (6) Additive: each microservice's generated client now exposes a NewSubflow(flow) typed client (a Subflow type + NewSubflow + marshalSubflow boilerplate, plus a per-endpoint method for every task/workflow) for invoking another service's task or workflow as an isolated child flow from inside a task body, via flow.Subtask/flow.Subgraph; the boilerplate is added to every *api/client.go regardless of whether it currently has tasks/workflows, so it is already present when one is added later. The foreman's Run/Await/Snapshot endpoints are unchanged (still return the outcome), so only outcome.FlowKey readers among those callers are affected. All breaks are loud compile errors except graph node semantics, which are unchanged.
---

## What changed

v1.40.0 is a single group of breaking changes to the workflow API in the [dwarf](https://github.com/microbus-io/dwarf)
module ("dwarf operates on node names; URLs are opaque identifiers the host resolves"). It must be applied to
any project that defines tasks or workflow graphs, or that subscribes to the foreman's `OnFlowStopped` event. A
project with no workflows needs none of it. Every break below is a **loud compile error** - there are no silent
behavioral changes this round.

- **`workflow.NewGraph` dropped its url argument (loud).** It went from `NewGraph(name, url string)` to
  `NewGraph(name string)`. The graph no longer stores its own resolve URL; the resolve key is the separate URL
  you already pass to the foreman (`Run`/`Create`/`LoadGraph`). The display name argument is unchanged. A call
  left at two arguments is a compile error.
- **`graph.AddTask` renamed to `graph.SetEndpoint` (loud).** Same `(name, url string)` arguments and same job -
  bind a graph node (by name) to its dispatch URL - now with create-or-update semantics (re-binding a name
  updates its URL instead of being ignored). Pure rename; the method `AddTask` no longer exists, so every call
  is a compile error.
- **`workflow.FlowOutcome.FlowKey` removed (loud).** The flow key is identity, not outcome, so it is no longer a
  field on `FlowOutcome`. Code that read `outcome.FlowKey` (from a `Run`/`Await`/`Snapshot` result, or from an
  `OnFlowStopped` payload) must get the key elsewhere: `Create` returns it, and the `OnFlowStopped` callback now
  receives it as an argument (below). `FlowSummary.FlowKey` (from `List`/`History`) is unchanged.
- **The foreman `OnFlowStopped` event gained a leading `flowKey` (loud).** The hook callback went from
  `func(ctx context.Context, outcome *workflow.FlowOutcome) error` to
  `func(ctx context.Context, flowKey string, outcome *workflow.FlowOutcome) error`. `flowKey` identifies which
  flow stopped (it used to be read off `outcome.FlowKey`). The arity change is a compile error.
- **The foreman `CreateTask` gained a required leading `name` (loud).** It went from
  `CreateTask(ctx, taskURL, initialState, opts)` to `CreateTask(ctx, name, taskURL, initialState, opts)`,
  matching `NewGraph`/`SetEndpoint`/`flow.Subtask`. `name` is the single-task flow's display name (shown in
  diagrams/history; required, non-empty); `taskURL` is the dispatch target. Every `foremanClient.CreateTask(...)`
  call is a compile error until the name is supplied.
- **New `NewSubflow(flow)` typed client (additive).** Each microservice's generated `*api` package now exposes a
  `Subflow` client for invoking that service's tasks and workflows as **isolated child flows from inside a task
  body** - `otherapi.NewSubflow(flow).SomeTask(ctx, args...)` (a task → `flow.Subtask`) or `.SomeWorkflow(ctx,
  args...)` (a workflow → `flow.Subgraph`). It is the blessed replacement for calling `foremanapi...Run` or the
  test-only `Executor` from a task body. Nothing breaks if you ignore it, but the migration adds the boilerplate
  to **every** `*api/client.go` (so it is present before a task/workflow is ever added) and a per-endpoint method
  for each existing task/workflow.

The foreman's `Run`, `Await`, and `Snapshot` endpoints are **unchanged** - they still return
`(outcome *workflow.FlowOutcome, err error)`. Only callers that read `outcome.FlowKey` from those results are
affected (see the `FlowKey` removal above); a caller that needs the key uses `Create` + `Start` + `Await`.

Graph node names and transition wiring are unchanged - this is an API-shape cleanup, not a topology change.

## Workflow

```
Upgrade a Microbus project to v1.40.0:
- [ ] Step 1: Detect workflow usage (Steps 2-6); Step 7 runs on every microservice regardless
- [ ] Step 2: Drop the url argument from every workflow.NewGraph call
- [ ] Step 3: Rename graph.AddTask(...) to graph.SetEndpoint(...)
- [ ] Step 4: Add the leading flowKey argument to every OnFlowStopped hook callback
- [ ] Step 5: Fix readers of the removed FlowOutcome.FlowKey field
- [ ] Step 6: Add the name argument to every foreman CreateTask caller
- [ ] Step 7: Add the NewSubflow client to every microservice's *api/client.go
- [ ] Step 8: Regenerate mocks + manifests, then go mod tidy && go vet ./... && go test ./...
```

#### Step 1: Detect Workflow Usage

If the project defines no tasks or workflow graphs, does not subscribe to `OnFlowStopped`, and never calls the
foreman's `CreateTask`, it does not touch the breaking API (Steps 2-6); skip to Step 7. Detect:

```bash
grep -rlE 'workflow\.NewGraph\(|\.AddTask\(|\.OnFlowStopped\(|\.FlowKey\b|\.CreateTask\(' --include='*.go' .
```

No matches means nothing to migrate in Steps 2-6. **Step 7 (the additive `NewSubflow` client) still runs on
every microservice**, even one with no workflows today, so the boilerplate is already present when a task or
graph is added later.

#### Step 2: Drop the url Argument From `workflow.NewGraph`

`NewGraph(name, url)` is now `NewGraph(name)` - a compile error until fixed. Find the calls:

```bash
grep -rn 'workflow\.NewGraph(' --include='*.go' .
```

The common form passes a string-literal name and the workflow's own `Def.URL()` as the second argument; the
transform drops that second argument. `NewGraph("CreditApproval", fooapi.CreditApproval.URL())` becomes
`NewGraph("CreditApproval")`:

```bash
grep -rl 'workflow\.NewGraph(' --include='*.go' . \
    | xargs perl -pi -e 's/NewGraph\(\s*("[^"]*")\s*,\s*[\w.]*\.URL\(\)\s*\)/NewGraph($1)/g'
```

The regex only matches the `("Name", pkg.X.URL())` form, so it is idempotent (a one-argument call is left
alone). Verify any call whose second argument was not a `.URL()` literal (a variable, or a computed URL) by
hand - drop the second argument and confirm the resolve URL is still passed wherever the flow is created
(`Run`/`Create`). The matching `NewGraph` in generated `mock.go` is regenerated in Step 6.

#### Step 3: Rename `graph.AddTask` to `graph.SetEndpoint`

Pure rename, same arguments. The method `AddTask` no longer exists, so every call is a compile error. Find and
rewrite project-wide (hand-written and generated alike - the generated `mock.go` is also regenerated in Step 6,
but rewriting it now keeps the build green in between):

```bash
find . -path ./vendor -prune -o -name '*.go' -exec \
    sed -i.bak 's/\.AddTask(/.SetEndpoint(/g' {} +
find . -name '*.go.bak' -delete
```

`SetEndpoint` is create-or-update, so a graph that bound the same node name twice (previously a no-op on the
second call) now takes the second URL - harmless when both bind the same URL, which is the only sensible prior
usage.

#### Step 4: Add the `flowKey` Argument to `OnFlowStopped` Callbacks

The `OnFlowStopped` hook handler gained a leading `flowKey string`. Find every subscription:

```bash
grep -rn '\.OnFlowStopped(' --include='*.go' .
```

For each, add `flowKey string` as the first callback parameter (after `ctx`). If the body used the flow key, it
almost certainly read it from `outcome.FlowKey` (now removed - Step 5); use the new `flowKey` argument instead:

```go
// before
.OnFlowStopped(func(ctx context.Context, outcome *workflow.FlowOutcome) error {
    log(outcome.FlowKey, outcome.Status) // FlowKey field is gone
    return nil
})
// after - flowKey is a parameter
.OnFlowStopped(func(ctx context.Context, flowKey string, outcome *workflow.FlowOutcome) error {
    log(flowKey, outcome.Status)
    return nil
})
```

#### Step 5: Fix Readers of the Removed `FlowOutcome.FlowKey`

`outcome.FlowKey` no longer compiles. Find the readers (excluding `FlowSummary.FlowKey`, which is unchanged):

```bash
grep -rn '\.FlowKey\b' --include='*.go' . | grep -iv 'summary\|flowsummary'
```

For each, source the key from where it is now available:

- In an `OnFlowStopped` callback: use the new `flowKey` parameter (Step 4).
- After launching a flow and needing its key (e.g. to `Resume`/`Cancel`/`History` an interrupted flow): use
  `Create` (which returns the key) + `Start` + `Await` instead of `Run` - `Run` awaits the flow but does not
  return the key.

```go
// before - Run, then use outcome.FlowKey to resume
out, err := client.Run(ctx, url, nil, nil)
... client.Resume(ctx, out.FlowKey, data)
// after - Create+Start+Await so we hold the key
flowKey, err := client.Create(ctx, url, nil, nil)
... client.Start(ctx, flowKey)
out, err := client.Await(ctx, flowKey)
... client.Resume(ctx, flowKey, data)
```

A `.FlowKey` access on a `List`/`History` result (`FlowSummary`) is unaffected - leave it.

#### Step 6: Add the `name` Argument to `CreateTask` Callers

The foreman's `CreateTask` gained a required leading `name`. Find every caller:

```bash
grep -rn '\.CreateTask(' --include='*.go' . | grep -v 'func '
```

For each, insert a `name` (the single-task flow's display name, PascalCase by convention) before the `taskURL`:

```go
// before
flowKey, err := foremanapi.NewClient(svc).CreateTask(ctx, myserviceapi.DoThing.URL(), state, nil)
// after - name first (matches NewGraph / SetEndpoint / flow.Subtask)
flowKey, err := foremanapi.NewClient(svc).CreateTask(ctx, "DoThing", myserviceapi.DoThing.URL(), state, nil)
```

The arity change is a compile error, so `go vet` flags every missed call. The foreman's own `CreateTask`
endpoint signature is part of the framework and is already updated by the version bump.

#### Step 7: Add the `NewSubflow` Client to Every `*api/client.go`

This step is **additive** and runs on **every** microservice in the project - even those with no tasks or
workflows yet - so the typed subflow client is already present when one is added later. It edits the
hand-maintained `*api/client.go`; it does **not** touch `manifest.yaml` or `mock.go`, so no `genmanifest`/
`genmock` run is needed for it (Step 8 only re-runs them for the breaking changes above).

**(a) Add the boilerplate to every `client.go`.** Immediately after the `marshalWorkflow` function in each
`*api/client.go`, add the `Subflow` type, `NewSubflow`, and `marshalSubflow` helper (the imports it needs -
`workflow` and `errors` - are already present wherever `marshalWorkflow` is):

```go
// Subflow runs this microservice's tasks and workflows as isolated child flows from INSIDE a task body.
// Unlike Executor (which carries a service.Publisher and is for tests), Subflow carries the calling
// task's *workflow.Flow: each method parks the calling step and re-enters it when the child terminates,
// returning (..., yield bool, err error). Only the explicit inputs cross into the child and only the
// explicit outputs cross back - the caller's flow state is NOT shared. This is the blessed way for one
// task to invoke another unit of work with state isolation; do not call Executor or foremanapi from a
// task body.
type Subflow struct {
	flow *workflow.Flow
}

// NewSubflow creates a subflow client bound to the calling task's flow carrier.
func NewSubflow(flow *workflow.Flow) Subflow {
	return Subflow{flow: flow}
}

// marshalSubflow runs a child flow via the flow carrier and returns the parker's yield. A non-empty
// taskName selects flow.Subtask (the engine synthesizes a single-task graph named taskName around url);
// an empty taskName selects flow.Subgraph (the host loads the graph by url) - mirroring the engine's
// taskName-presence discriminator. in is marshaled to the child's input; the child's final_state is
// unmarshaled into out.
func marshalSubflow(flow *workflow.Flow, taskName, url string, in any, out any) (yield bool, err error) {
	if flow == nil {
		return false, errors.New("Subflow requires a flow carrier (call from a task body)")
	}
	if taskName != "" {
		return flow.Subtask(taskName, url, in, out)
	}
	return flow.Subgraph(url, in, out)
}
```

(In a service with no tasks/workflows, `marshalSubflow` is an unused package function - legal in Go, and it
becomes used the moment the first per-endpoint method below is added.)

**(b) Add one `Subflow` method per task/workflow endpoint.** For each existing `Executor` endpoint method,
append a parallel `Subflow` method. **Mirror the `Executor` method's signature exactly**, inserting `yield bool`
before the trailing `err` - the `Executor` is the source of truth for the output names (it already resolves
read-modify-write `Out`-suffix names like `reviewAttemptsOut`, which the raw JSON tag would collide with the
input). A **task** endpoint maps to `flow.Subtask` with the PascalCase task name; a **workflow** endpoint maps
to `flow.Subgraph` with an empty name and **drops** the Executor's `status` return:

```go
// task endpoint  ->  Subtask (PascalCase name), mirrors VerifyCredit(ctx, creditScore) (creditVerified, err)
func (_sf Subflow) VerifyCredit(ctx context.Context, creditScore int) (creditVerified bool, yield bool, err error) { // MARKER: VerifyCredit
	var out VerifyCreditOut
	yield, err = marshalSubflow(_sf.flow, "VerifyCredit", VerifyCredit.URL(), VerifyCreditIn{
		CreditScore: creditScore,
	}, &out)
	if yield || err != nil {
		return creditVerified, yield, err
	}
	return out.CreditVerified, false, nil
}

// workflow endpoint  ->  Subgraph (empty name), drops the Executor's `status` return
func (_sf Subflow) CreditApproval(ctx context.Context, applicant Applicant) (approved bool, creditVerified bool, employmentFailures int, identityVerified bool, yield bool, err error) { // MARKER: CreditApproval
	var out CreditApprovalOut
	yield, err = marshalSubflow(_sf.flow, "", CreditApproval.URL(), CreditApprovalIn{
		Applicant: applicant,
	}, &out)
	if yield || err != nil {
		return approved, creditVerified, employmentFailures, identityVerified, yield, err
	}
	return out.Approved, out.CreditVerified, out.EmploymentFailures, out.IdentityVerified, false, nil
}
```

A task that returns no outputs passes `nil` as the last `marshalSubflow` argument (no `var out`) and returns
just `(yield bool, err error)`. If an `Executor` method was hand-written to *discard* a task's declared outputs
(it passes `&SomethingOut{}` and returns only `err`), fix that `Executor` method first to surface the outputs,
then mirror it - both the `Executor` and the `Subflow` should expose what the task produces.

#### Step 8: Regenerate, Tidy, and Verify

The `NewGraph` and `AddTask` changes touch generated artifacts (`mock.go`, `manifest.yaml`), so regenerate them
from the now-fixed source for every microservice you touched:

```bash
for d in $(find . -name "mock.go" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genmock --path "$d"
done
for d in $(find . -name "manifest.yaml" -exec dirname {} \; | sort -u); do
    go run github.com/microbus-io/fabric/cmd/genmanifest --path "$d"
done
```

`genmock` re-emits `mock.go` with the one-argument `NewGraph`, and `genmanifest` bumps each manifest's
`frameworkVersion` to `1.40.0`. Then resolve dependencies and verify:

```bash
go mod tidy
go vet ./...
go test ./...
```

`go mod tidy` pulls the dwarf version the upgraded fabric requires. All four breaks are compile errors, so a
clean `go vet ./...` is strong evidence the migration is complete; the workflow tests confirm runtime behavior.

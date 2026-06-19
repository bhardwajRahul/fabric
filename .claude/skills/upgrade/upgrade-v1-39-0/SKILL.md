---
name: upgrade-v1-39-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.38.x to v1.39.0. Two groups of changes. (A) Non-breaking sequel telemetry - the sequel library (v1.10.2) now emits OpenTelemetry spans, sequel_* metrics, and slog migration logs once the connector's TracerProvider/MeterProvider/Logger are attached to the *sequel.DB, so every SQL CRUD microservice's openDatabase gains three setter calls. (B) Breaking workflow changes - the workflow package moved out of the fabric module into the dwarf module (github.com/microbus-io/fabric/workflow becomes github.com/microbus-io/dwarf/workflow, adding a dwarf dependency), workflow.NewGraph gained a leading name argument (NewGraph(url) becomes NewGraph("Name", url)), flow.Goto now takes a graph node name instead of an endpoint URL (a silent break, since both are strings), and flow.Subgraph/flow.Interrupt dropped their returned result map in favor of an out-pointer argument (Subgraph(url, in) (out, yield, err) becomes Subgraph(url, in, &out) (yield, err); Interrupt(payload) (resume, yield, err) becomes Interrupt(payload, &resume) (yield, err)) - a loud compile break. The foreman's StartNotify endpoint was removed in favor of FlowOptions.NotifyOnStop set at Create (a loud break - the endpoint no longer exists). Existing graphs keep their node names; PascalCase naming is only the convention for newly scaffolded graphs.
---

## What changed

v1.39.0 bundles two unrelated groups. Group A is additive and safe to skip; Group B is breaking and must be
applied to any project with workflows (tasks or workflow graphs). A project may need one group, both, or
neither - the steps are individually guarded.

### Group A: sequel telemetry (non-breaking)

sequel v1.10.2 added an opt-in observability layer. A `*sequel.DB` emits OpenTelemetry client spans (per
query, `Transact`, and `Migrate`), `sequel_*` metrics (query/transaction duration histograms, lock
contention and migration counters, connection-pool gauges), and `slog` migration logs - but **only after the
caller attaches the providers** with `SetTracerProvider` / `SetMeterProvider` / `SetLogger`. Without them
sequel falls back to the process-wide OTEL no-op providers and a discard logger, so nothing reaches the
connector's telemetry pipeline. The project compiles and behaves identically without the wiring; the only
consequence of skipping it is that sequel's signals stay dark.

### Group B: workflow package relocated to the dwarf module (breaking)

The workflow engine was extracted into its own module, [dwarf](https://github.com/microbus-io/dwarf). The
`workflow` package that defined `Graph`, `Flow`, `FlowOptions`, `FlowOutcome`, the reducers, and `END` moved
with it. Three things change for any project that defines tasks or workflow graphs:

- **Import path (loud).** `github.com/microbus-io/fabric/workflow` becomes
  `github.com/microbus-io/dwarf/workflow`. Every file that imports it - hand-written `service.go`, the
  generated `intermediate.go` / `mock.go` / `mock_test.go`, and the `*api/client.go` proxy - is affected, and
  the project gains a direct dependency on the dwarf module. The package's exported identifiers
  (`workflow.Flow`, `workflow.FlowOptions`, `workflow.END`, `workflow.ReducerAppend`, ...) are otherwise
  unchanged - only the import path moves.
- **`workflow.NewGraph` gained a leading name argument (loud).** It went from `NewGraph(url string)` to
  `NewGraph(name, url string)`; the new first argument is the graph's name (its PascalCase feature name, the
  same `Def` whose `URL()` is the second argument). A call left at one argument is a compile error.
- **`flow.Goto` now takes a graph node name, not an endpoint URL (silent).** The signature is unchanged
  (`Goto(string)`), so it still compiles, but the value's meaning changed: pass the **node name** the task was
  registered under in the graph's `AddTask`, not `someapi.Task.URL()`. A call left as `flow.Goto(api.X.URL())`
  compiles and then fails to route at runtime.
- **`flow.Subgraph` and `flow.Interrupt` dropped the returned result map for an out-pointer (loud).** Both used
  to return the child/resume result as a leading `map[string]any`; they now take a trailing `out any` pointer
  the result is unmarshaled into and return only `(yield, err)`. `Subgraph(url, in) (out, yield, err)` becomes
  `Subgraph(url, in, &out) (yield, err)`; `Interrupt(payload) (resume, yield, err)` becomes
  `Interrupt(payload, &resume) (yield, err)`. The input/payload argument was already `any`, so a struct or a
  `map[string]any` works on the way in; the out pointer may be a `*struct` (read fields with type safety) or a
  `*map[string]any` (preserve the old map access), or `nil` to ignore the result. The arity change is a compile
  error, so every call site is caught by the build.
- **`foremanapi.StartNotify` removed for `FlowOptions.NotifyOnStop` (loud).** The `StartNotify(flowKey, host)`
  endpoint no longer exists. To receive the `OnFlowStopped` event when a flow terminates, set
  `NotifyOnStop: true` in the `*workflow.FlowOptions` passed to `Create` (or `Run`), then `Start` the flow
  normally. The foreman records the caller's host from the request frame at `Create`, so no hostname argument
  is passed and no separate call is made.

The framework also adopted **PascalCase for graph and task (node) names** (`AddTask("VerifySSN", ...)` rather
than `"verifySSN"`) going forward, and the refreshed agent rules scaffold new graphs that way. Node names are
arbitrary strings that only need to be internally consistent, so **this convention does not require touching
existing graphs** - leave their node names exactly as they are. The `flow.Goto` migration below therefore
uses each graph's *existing* registered names, whatever their case.

## Workflow

```
Upgrade a Microbus project to v1.39.0:
- [ ] Step 1: Bump the sequel dependency to v1.10.2 (if present)
- [ ] Step 2: Wire sequel telemetry into each SQL CRUD microservice's openDatabase
- [ ] Step 3: Relocate the workflow import project-wide (fabric/workflow -> dwarf/workflow)
- [ ] Step 4: Add the leading name argument to every workflow.NewGraph call
- [ ] Step 5: Change flow.Goto arguments from endpoint URLs to node names
- [ ] Step 6: Convert flow.Subgraph / flow.Interrupt call sites to the out-pointer signature
- [ ] Step 7: Replace foremanapi.StartNotify with FlowOptions.NotifyOnStop
- [ ] Step 8: Copy updated Grafana dashboards (regeneration + verification deferred to the orchestrator)
```

#### Step 1: Bump the sequel Dependency

If the project imports sequel directly, pin it to v1.10.2:

```bash
grep -q 'github.com/microbus-io/sequel' go.mod && go get github.com/microbus-io/sequel@v1.10.2
```

A project with no `github.com/microbus-io/sequel` line in `go.mod` has no SQL CRUD microservices; skip
Steps 1-2. (`go mod tidy` is deferred to Step 8.)

#### Step 2: Wire sequel Telemetry Into Each `openDatabase`

SQL CRUD microservices open their database in an `openDatabase` method that calls `sequel.OpenSingleton`
(or `sequel.Open`). Find every call site:

```bash
grep -rln 'sequel\.OpenSingleton\|sequel\.Open(' --include='*.go' .
```

Each hit is a microservice to migrate; a file that already contains `svc.db.SetMeterProvider(` is done -
skip it. (`cmd/`, `main/`, and packages that merely import sequel for DSN parsing do not call
`Open`/`OpenSingleton` and will not match.) In each matched file, immediately after the opened-DB error check
and **before** the `Migrate` call (so migrations are instrumented too), insert the three setters:

```go
svc.db, err = sequel.OpenSingleton(driverName, dataSourceName)
if err != nil {
    return errors.Trace(err)
}
// Route sequel's spans, sequel_* metrics, and migration logs through the connector's telemetry pipeline.
svc.db.SetTracerProvider(svc.TracerProvider())
svc.db.SetMeterProvider(svc.MeterProvider())
svc.db.SetLogger(svc.Logger())
dirFS, err := fs.Sub(svc.ResFS(), "sql")
// ... existing Migrate call unchanged
```

The accessors are inherited from the embedded connector (no new import) and return no-op providers / the
connector logger when a signal is disabled, so the block is safe in every deployment including `TESTING`. Do
not add `SetVerbose(true)` - per-query Debug logs are off by design; an operator enables them with
`MICROBUS_LOG_DEBUG=1`.

#### Step 3: Relocate the workflow Import Project-Wide

If the project defines no tasks or workflows it never imports the workflow package; skip Steps 3-7. Detect:

```bash
grep -rl 'github.com/microbus-io/fabric/workflow' --include='*.go' .
```

If there are no matches, skip to Step 8. Otherwise rewrite the import path across **every** `.go` file in one
pass - hand-written and generated alike. A single project-wide `sed` does it (the `-i.bak` form is portable
across GNU and BSD/macOS sed; the second command removes the backups):

```bash
find . -path ./vendor -prune -o -name '*.go' -exec \
    sed -i.bak 's#github.com/microbus-io/fabric/workflow#github.com/microbus-io/dwarf/workflow#g' {} +
find . -name '*.go.bak' -delete
```

This is a pure path move - identifiers like `workflow.Flow`, `workflow.FlowOptions`, `workflow.FlowOutcome`,
`workflow.END`, and the `workflow.Reducer*` values are unchanged. `go mod tidy` (Step 8) adds the
`github.com/microbus-io/dwarf` dependency at the version the upgraded fabric requires.

#### Step 4: Add the Name Argument to `workflow.NewGraph`

`NewGraph(url)` is now `NewGraph(name, url)` - a compile error until fixed. Find the calls:

```bash
grep -rn 'workflow\.NewGraph(' --include='*.go' .
```

Each one passes the workflow's own `Def.URL()`; the new name is that `Def`'s identifier (the graph name is a
fresh argument, not a node rename, so using the PascalCase `Def` identifier is correct regardless of how the
graph's nodes are named). The transform is mechanical -
`NewGraph(fooapi.CreditApproval.URL())` becomes `NewGraph("CreditApproval", fooapi.CreditApproval.URL())`:

```bash
grep -rl 'workflow\.NewGraph(' --include='*.go' . \
    | xargs perl -pi -e 's/NewGraph\(\s*([\w.]*\b(\w+)\.URL\(\))\s*\)/NewGraph("$2", $1)/g'
```

The regex only matches the single-URL-argument form, so it is idempotent (a call already carrying a string
name is left alone). It rewrites the hand-written graph builder in `service.go`; the matching `NewGraph` in
the generated `mock.go` is regenerated in Step 8.

#### Step 5: Change `flow.Goto` Arguments From URLs to Node Names

This is a **silent** break - `flow.Goto(api.X.URL())` still compiles. `Goto` now expects the **node name** the
target task was registered under in the graph's `AddTask("<nodeName>", api.X.URL())`, not the URL. Find every
call:

```bash
grep -rn '\.Goto(' --include='*.go' .
```

For each, open the graph builder, find the `AddTask` whose URL matches, and pass **that exact registered node
name** as a string - case included. Existing graphs are not renamed (Step ordering keeps their node names as
they are), so a v1.38 graph that registered `AddTask("requestMoreInfo", creditflowapi.RequestMoreInfo.URL())`
must use the camelCase name it already has:

```go
// before
flow.Goto(creditflowapi.RequestMoreInfo.URL())
// after - the node name from AddTask, not the Def identifier
flow.Goto("requestMoreInfo")
```

Do not derive the node name from the `Def` identifier or auto-transform these blindly: the registered name
may differ in case (or entirely) from the `Def`, and a mismatch between `Goto` and `AddTask` is caught only by
`graph.Validate()` at startup, not by the compiler. Verify each one against the graph builder.

#### Step 6: Convert `flow.Subgraph` / `flow.Interrupt` Call Sites

`flow.Subgraph` and `flow.Interrupt` no longer return the result as a leading `map[string]any`; the result is
unmarshaled into a trailing `out`/`resume` pointer argument and the calls return only `(yield, err)`. This is a
loud arity break - every call site is a compile error until converted. Find them:

```bash
grep -rn '\.Subgraph(\|\.Interrupt(' --include='*.go' .
```

(`SubgraphRequested`/`InterruptRequested` and the unrelated `application.Interrupt()` test-signal calls do not
match this two-element form - ignore them.) For each call, move the leading result variable to a `var`
declaration above the call and pass its address as the new trailing argument:

```go
// before
out, yield, err := flow.Subgraph(childURL, inputMap)
...
v, _ := out["field"].(float64)

// after - declare the target, pass &out, drop the leading return
var out map[string]any
yield, err := flow.Subgraph(childURL, inputMap, &out)
...
v, _ := out["field"].(float64)
```

```go
// before
resume, yield, err := flow.Interrupt(payload)
// after
var resume map[string]any
yield, err := flow.Interrupt(payload, &resume)
```

A call that ignored the result (`_, yield, err := flow.Subgraph(url, in)`) becomes
`yield, err := flow.Subgraph(url, in, nil)` - pass `nil` for the out pointer rather than declaring an unused
variable. Keeping `out`/`resume` typed as `map[string]any` is the smallest diff and preserves existing
`out["field"]` access. Where the child workflow has generated `…In`/`…Out` arg structs (any `sub.Workflow`
endpoint), prefer the typed form for input and output - `var out fooapi.ChildOut; flow.Subgraph(fooapi.Child.URL(),
fooapi.ChildIn{Field: v}, &out)` then read `out.Field` - which is the new ergonomic the change exists to enable.
The generic dynamic-tool dispatcher that calls `flow.Subgraph(def.URL, …)` over an arbitrary tool URL has no
single arg type and stays on `map[string]any`.

#### Step 7: Replace `StartNotify` with `FlowOptions.NotifyOnStop`

The foreman's `StartNotify` endpoint is removed. A flow now opts into the `OnFlowStopped` event by setting
`NotifyOnStop` at `Create`. Find the calls:

```bash
grep -rn 'StartNotify' --include='*.go' .
```

For each, set `NotifyOnStop: true` in the `*workflow.FlowOptions` passed to `Create` (or `Run`) and delete the
separate `StartNotify` call; the foreman records the caller's host from the request frame, so the explicit
hostname argument goes away:

```go
// before
flowID, err := client.Create(ctx, url, initialState, nil)
...
err = client.StartNotify(ctx, flowID, svc.Hostname())

// after - opt in at Create, then Start normally
flowID, err := client.Create(ctx, url, initialState, &workflow.FlowOptions{NotifyOnStop: true})
...
err = client.Start(ctx, flowID)
```

The inbound `OnFlowStopped` event sink is unchanged. A project that never called `StartNotify` needs no change.

#### Step 8: Copy Updated Grafana Dashboards

The import move and `NewGraph` change touch generated artifacts (`mock.go`, `mock_test.go`, `manifest.yaml`), but
do not regenerate them here - the `upgrade-microbus` orchestrator regenerates every microservice's boilerplate from
source and runs `go mod tidy && go vet ./... && go test ./...` once, after every numbered skill has run. That
`go mod tidy` adds `github.com/microbus-io/dwarf` (at fabric's required version) and the sequel bump. The
load-bearing check there is the silent break from Step 5: a workflow test that drives a `Goto` transition will fail
to route if a call still passes a `.URL()` instead of the node name.

The sequel signals wired in Step 2 are charted by the framework's `SQL Overview` Grafana dashboard
(`setup/grafana/dashboards/sequel-overview.json`) and a SQL row on `microservice-focus`; the workflow engine's
own database pressure appears on `workflow-overview`. If the project keeps its own Grafana provisioning, copy
those dashboards from the framework's `setup/grafana/dashboards/`; the `upgrade-microbus` refresh updates
`.claude` only, not `setup/`.

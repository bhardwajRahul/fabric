---
name: upgrade-v1-35-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.34.x to v1.35.0. Removes the Microbus clock-shift mechanism (svc.Now, frame.ClockShift family, Microbus-Clock-Shift header), introduces the workflow.FlowOutcome return shape for Snapshot/Await/Run and the OnFlowStopped event payload, moves the flow status constants from foremanapi to the workflow package (foremanapi.StatusCompleted -> workflow.StatusCompleted, etc.), adds a reason argument to foremanapi.Cancel, adds a trailing *workflow.FlowOptions argument to foremanapi.Fork and foremanapi.Continue, captures the new nextCursor return from foremanapi.List, renames the foreman's Prometheus metrics under microbus_flows_* / microbus_steps_* / microbus_task_* prefixes with shorter attribute labels, and migrates every sequel microservice from the old sequel.Open/OpenTesting (now renamed OpenSingleton/OpenSingletonTesting) to the explicit two-step pattern: CreateTestingDatabase resolves the per-test DSN in TESTING, then OpenSingleton opens the pool.
---

## Background

- **Clock-shift removed.** `svc.Now(ctx)` is gone. The clock-shift frame header
  (`Microbus-Clock-Shift`) and its accessors (`frame.SetClockShift`, `frame.IncrementClockShift`,
  `frame.ClockShift`, `frame.HeaderClockShift`) are gone. The mechanism only worked at the
  per-request handler granularity, did not virtualize sleeps/tickers/database `NOW()`/background
  goroutines, and the framework no longer pretends to offer time mocking. Code that read
  `svc.Now(ctx)` should call `time.Now()` directly. Tests that depended on `SetClockShift` should
  move to a real clock-mocking tool: Go 1.25 ships `testing/synctest` in the standard library, and
  `github.com/benbjohnson/clock` is a popular external option. `svc.Sleep(ctx, dur)` is unchanged
  and stays - it integrates with the microservice's lifetime context (early-return on shutdown),
  which is unrelated to clock mocking.

- **Foreman `Snapshot`/`Await`/`Run` return `*workflow.FlowOutcome`.** The previous
  `(status string, state map[string]any, err error)` tuple is replaced with a single struct that
  also carries `Error` (when Status="failed"), `InterruptPayload` (when Status="interrupted"),
  `CancelReason` (when Status="cancelled"), and `FlowKey`. The Go-level `error` return is now
  reserved for transport/foreman/timeout failures; a workflow failure surfaces as
  `outcome.Status == "failed"` with `outcome.Error` populated, so callers should branch on the
  outcome's Status field rather than treating a non-nil Go error as workflow failure. The
  `AwaitAndParse`/`RunAndParse`/`SnapshotAndParse` helpers gained the same return shape:
  `(*FlowOutcome, error)`.

- **`Snapshot` of an interrupted flow no longer merges the interrupt payload into `State`.**
  `State` returns the flow's accumulated state *at the time of the interrupt*; `InterruptPayload`
  returns the raw `flow.Interrupt(payload)` argument as a distinct field. Callers that relied on
  the pre-merged view should `workflow.MergeState(out.State, out.InterruptPayload, graph.Reducers())`
  themselves.

- **`OnFlowStopped` event payload is `*FlowOutcome`.** Subscriber handler signature changes from
  `func(ctx, flowKey, status, snapshot) error` to `func(ctx, *workflow.FlowOutcome) error`. The
  handler accesses `outcome.FlowKey`, `outcome.Status`, `outcome.State`, and the side-channel
  fields directly.

- **`foremanapi.Cancel` takes a `reason string` argument.** `Client.Cancel(ctx, flowKey)` becomes
  `Client.Cancel(ctx, flowKey, reason)`. Pass `""` if no reason. The reason is stored on every flow
  in the cancellation chain (root + descendants) as `cancel_reason`, surfaced through
  `outcome.CancelReason`.

- **`FlowStep.InterruptPayload` is exposed via `History`.** Per-step `interrupt_payload` was always
  in SQL but not surfaced on the API type; the field is additive and callers needn't migrate.

- **Flow status constants moved from `foremanapi` to `workflow`.** `foremanapi.StatusCompleted`,
  `StatusFailed`, `StatusCancelled`, `StatusInterrupted`, `StatusRunning`, `StatusPending`,
  `StatusCreated`, `StatusRetried` are now `workflow.StatusCompleted`, etc. The string values are
  unchanged. This is symmetric with the `FlowOutcome` move - they're workflow-domain identifiers,
  not foreman-implementation details, and downstream code that compares `outcome.Status` against a
  constant should reach into `workflow/`, not `foremanapi/`.

- **`FlowSummary` (returned by `List`) gained `Error string` and `CancelReason string`.** No `State`
  or `InterruptPayload` to keep the listing row light; callers needing those follow up with
  `Snapshot` for the specific flow.

- **Schema migration `9.sql`.** Adds `error TEXT`, `cancel_reason TEXT`, and `tenant_id INT` columns
  on `microbus_flows`, applied automatically at foreman startup via the sequel migration runner.
  No manual schema change required for downstream projects.

- **Foreman `Fork` and `Continue` take a trailing `opts *workflow.FlowOptions`.** Same shape as the
  v1.32.0 `Create`/`Run` change. `nil` means defaults. Both `Client` and `MulticastClient` methods,
  and their `Response.Get()` shapes via the underlying `*In` structs. `Executor` methods are
  unchanged.

- **Foreman `List` returns a pagination cursor.** `Client.List(ctx, query)` now returns
  `(flows []FlowSummary, nextCursor string, err error)`; `ListResponse.Get()` gained the same
  trailing `nextCursor string`. Pass it back as `Query.Cursor` to fetch the next page; ignore with
  `_` to retain old behavior. Pagination is per-shard newest-first - see the foreman's docs.

- **Foreman metrics regrouped under `microbus_(flows|steps|task)_*` and attribute labels
  shortened.** Wire-name and label changes only; the values and intent are unchanged. If your
  project copies the foreman's Grafana dashboards or has its own alerts referencing these metric
  names, update them. Replace via this table:

  | Before (otelName / label) | After |
  |---|---|
  | `microbus_queue_depth` | `microbus_steps_queue_depth` |
  | `microbus_pending_steps_by_priority` | `microbus_steps_pending` |
  | `microbus_oldest_pending_age_seconds` | `microbus_steps_oldest_pending_age_seconds` |
  | `microbus_distinct_fairness_keys` | `microbus_steps_fairness_keys` |
  | `microbus_completion_contention_total` | **removed** |
  | `microbus_claim_wait_seconds` | **removed** |
  | `microbus_backpressure_backoffs_total` | `microbus_task_rate_cuts_total` |
  | label `workflow_name=` | `workflow=` |
  | label `task_name=` | `task=` |

  `microbus_completion_contention_total` was a diagnostic that only fired meaningfully on SQLite;
  the rewind-on-lock-contention recovery path still runs, just unmetered. `microbus_claim_wait_seconds`
  conflated "no work to do" (idle workers, large samples) with "refiller starved while backlog
  exists" (real problem, same large samples) and was structurally unable to distinguish them; use
  the `microbus_steps_pending` vs `microbus_steps_queue_depth` divergence instead.

- **Sequel `Open`/`OpenTesting` were renamed; new constructors with different semantics took the old
  names.** The previous `sequel.Open(driver, dsn)` and `sequel.OpenTesting(driver, dsn, testID)`
  coalesced openers per DSN and auto-managed the connection pool via a sqrt-of-refcount formula -
  designed for the bundled-microservices case where many idle services share a database. They are now
  `sequel.OpenSingleton` and `sequel.OpenSingletonTesting`, and the unsuffixed names refer to new
  constructors that return a fresh `*sql.DB` per call with standard `database/sql` pool defaults. In
  TESTING, the per-test database creation is split off into `sequel.CreateTestingDatabase(driver,
  baseDSN, testID) (dsn string, err)`, which the caller pairs with either Open or OpenSingleton
  depending on whether they want a shared or dedicated pool. Most existing SQL CRUD microservices
  want the singleton behavior (shared pool across the test app's microservices), so they migrate to
  `CreateTestingDatabase` + `OpenSingleton`. Heavy single-consumer services (orchestrators with
  worker pools, etc.) want a dedicated pool and migrate to `CreateTestingDatabase` + `Open`. The
  string-formatted DSN, migration runner, macro expansion, busy-timeout injection, and lock-
  contention detection are unchanged.

- **Non-breaking additions worth knowing about**, not migrated by this skill:
  - `flow.CreatedAt()` and `flow.UpdatedAt()` on `*workflow.Flow` expose the flow's lifecycle
    timestamps. Use these for elapsed-time guards inside tasks.
  - `flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay) bool` and the shorthand
    `flow.RetryNow() bool` request a foreman-driven re-execution of the current step. Distinct
    from the API-level `Retry` which creates a new step.
  - `foremanapi.ShardInfo` is a read-only endpoint returning per-shard health summaries.
  - `foremanapi.Delete(flowKey)` removes one flow and its steps. Refuses to act on a running flow.
  - `foremanapi.Purge(query)` bulk-deletes flows matching the query, except running flows. Same
    `Query` shape as `List`; capped at 10000 flows per call. Returns the deleted count.
  - `Query` gained `TaskName` (join filter for "flows blocked on task X"),
    `TenantID` (multi-tenant filter; populated from `frame.Tenant()` at Create time and
    inherited by subgraph/Fork; `0` in the query means "no filter" rather than "tenant=0"),
    `OlderThan time.Duration` (database-anchored age filter), and `NewerThan time.Duration`
    (composable inverse of OlderThan).
  - `FlowSummary` gained `Error` and `CancelReason` so listings carry the side-channel info
    without a follow-up `Snapshot`.

## Workflow

```
Upgrade a Microbus project to v1.35.0:
- [ ] Step 1: Replace svc.Now(ctx) with time.Now()
- [ ] Step 2: Remove frame clock-shift calls
- [ ] Step 3: Update every microservice's *api/client.go to the new WorkflowRunner interface and marshalWorkflow
- [ ] Step 4: Move flow status constants from foremanapi to workflow
- [ ] Step 5: Migrate Snapshot/Await/Run/AwaitAndParse/RunAndParse/SnapshotAndParse to *FlowOutcome
- [ ] Step 6: Migrate OnFlowStopped hook handler to the FlowOutcome signature
- [ ] Step 7: Append a reason argument to Cancel callers
- [ ] Step 8: Insert a nil argument into foreman Fork / Continue callers
- [ ] Step 9: Capture the new nextCursor return from foreman List
- [ ] Step 10: Rename foreman metric references in Grafana / alert configs
- [ ] Step 11: Migrate sequel microservices to CreateTestingDatabase + OpenSingleton
- [ ] Step 12: (manifest + mock regeneration deferred to the orchestrator)
```

#### Step 1: Replace `svc.Now(ctx)` With `time.Now()`

```bash
grep -rn "svc\.Now(\|connector\.Now(" --include="*.go" .
```

For each call, substitute. The previous return type was `time.Time` in UTC, so preserve UTC if any
downstream comparison or formatting depended on it:

- `svc.Now(ctx)` -> `time.Now().UTC()` (safe default - matches the old behavior byte-for-byte)
- If the call site immediately does arithmetic like `svc.Now(ctx).Sub(t)`, prefer
  `time.Since(t)` which is more idiomatic.
- If the call site stamps a wall-clock for storage and the storage is timezone-aware, plain
  `time.Now()` is fine; the old `.UTC()` was decorative.

Drop the `ctx` argument from any helper that only existed to thread it to `svc.Now`. Run
`goimports -w` to clean up any newly unused `context` imports.

#### Step 2: Remove Frame Clock-Shift Calls

```bash
grep -rn "ClockShift\|HeaderClockShift" --include="*.go" .
```

All occurrences are removable. Concrete forms:

- `frame.Of(ctx).SetClockShift(d)` - delete the line; whatever test was setting the shift needs
  a real clock-mocking tool (see Background).
- `frame.Of(ctx).IncrementClockShift(d)` - same.
- `frame.Of(ctx).ClockShift()` - returned `time.Duration`; the value was only meaningful for
  feeding back into `svc.Now`, which is also gone. Delete the read and any code branch keyed on it.
- `frame.HeaderClockShift` - delete any header-allowlist entry or test assertion referencing it.

If the removal leaves the `frame` import unused, drop it.

#### Step 3: Update Every Microservice's `*api/client.go`

**Every** `*api/client.go` file in the project carries a copy of the `WorkflowRunner` interface and the `marshalWorkflow` helper. These are hand-edited templates, not generated, so the upgrade has to touch each one. The interface return type and the helper's binding both changed.

```bash
grep -rln "WorkflowRunner interface" --include="*.go" .
```

For each file in the output:

**3a. Update the `WorkflowRunner` interface declaration.**

Find:

```go
type WorkflowRunner interface {
    Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (status string, state map[string]any, err error)
}
```

Replace with:

```go
type WorkflowRunner interface {
    Run(ctx context.Context, workflowName string, initialState any, opts *workflow.FlowOptions) (outcome *workflow.FlowOutcome, err error)
}
```

Note the type is `*workflow.FlowOutcome` (in the `workflow` package, alongside `FlowOptions`), **not** `*foremanapi.FlowOutcome`. The struct was moved to `workflow/` precisely so downstream `*api/client.go` files do not have to pull in `coreservices/foreman/foremanapi/` for the runner interface.

**3b. Update the `marshalWorkflow` function body.**

Find:

```go
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, flowOptions *workflow.FlowOptions, workflowURL string, in any, out any) (status string, err error) {
    status, state, err := runner.Run(ctx, workflowURL, in, flowOptions)
    if err != nil {
        return status, err // No trace
    }
    if out != nil && state != nil {
        data, err := json.Marshal(state)
        if err != nil {
            return status, errors.Trace(err)
        }
        err = json.Unmarshal(data, out)
        if err != nil {
            return status, errors.Trace(err)
        }
    }
    return status, nil
}
```

Replace with:

```go
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, flowOptions *workflow.FlowOptions, workflowURL string, in any, out any) (status string, err error) {
    outcome, err := runner.Run(ctx, workflowURL, in, flowOptions)
    if err != nil {
        if outcome != nil {
            return outcome.Status, err // No trace
        }
        return "", err // No trace
    }
    if outcome == nil {
        return "", nil
    }
    status = outcome.Status
    if out != nil && outcome.State != nil {
        data, err := json.Marshal(outcome.State)
        if err != nil {
            return status, errors.Trace(err)
        }
        err = json.Unmarshal(data, out)
        if err != nil {
            return status, errors.Trace(err)
        }
    }
    return status, nil
}
```

The `marshalWorkflow` external signature is **unchanged** (`(status string, err error)`), so the per-workflow `Executor.MyWorkflow(...)` methods in `*api/client.go` keep their tuple return shape. Tests written against the Executor pattern do not need to migrate. Only `Client`-side direct calls to `Run`/`Snapshot`/`Await` need migration (Step 4).

**3c. Ensure the `workflow` import is present, drop the `foremanapi` import if it became unused.**

Every `*api/client.go` already imports `workflow` for `FlowOptions`, so the `*workflow.FlowOutcome` reference doesn't add a new dependency. If your file previously imported `"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"` only for the runner interface (the common case in non-foreman microservices), drop the import. A `goimports -w` pass after the edit catches this.

**3d. Sweep heuristic.**

If your project has many microservices, the substitution is mechanical and `gofmt -r` style. A quick Python script that finds every `client.go` with `WorkflowRunner interface` and applies the two text replacements is faster than per-file hand-editing; the framework's own upgrade for v1.35.0 used exactly this approach across 50+ files.

#### Step 4: Move Flow Status Constants From `foremanapi` to `workflow`

```bash
grep -rn "foremanapi\.Status" --include="*.go" .
```

Every reference to `foremanapi.StatusCompleted`, `foremanapi.StatusFailed`, `foremanapi.StatusCancelled`, `foremanapi.StatusInterrupted`, `foremanapi.StatusRunning`, `foremanapi.StatusPending`, `foremanapi.StatusCreated`, or `foremanapi.StatusRetried` becomes `workflow.StatusCompleted`, etc. The string values are unchanged, so no wire-format or stored-data migration is needed.

A mechanical substitution covers it:

```bash
gofmt -r 'foremanapi.StatusCompleted -> workflow.StatusCompleted' -w <file>
gofmt -r 'foremanapi.StatusFailed -> workflow.StatusFailed' -w <file>
# ... and so on for the other six status constants
```

Or via a Python regex sweep:

```python
re.sub(r'\bforemanapi\.Status(\w+)', r'workflow.Status\1', text)
```

In files that previously imported `foremanapi` only for the status constants (now without other foremanapi references), drop the import; ensure `"github.com/microbus-io/fabric/workflow"` is imported instead. Workflow-microservice tests typically already import both packages (foremanapi for `NewClient`, workflow for `FlowOptions`); the change usually only affects the import that becomes unused.

#### Step 5: Migrate `Snapshot`/`Await`/`Run`/`*AndParse` to `*FlowOutcome`

```bash
grep -rn -E "\.(Snapshot|Await|Run|AwaitAndParse|RunAndParse|SnapshotAndParse)\(" --include="*.go" . | grep -iE "(foreman|workflow)"
```

These methods now return `(*workflow.FlowOutcome, error)` instead of `(status string, state map[string]any, error)`. The tuple-binding callers are the bulk of the work.

Mechanical migration patterns:

- `status, state, err := c.Await(ctx, k)` -> `outcome, err := c.Await(ctx, k); status, state := outcome.Status, outcome.State` (defensively, guard `outcome != nil` before dereferencing if the call may return both nil outcome and nil err, e.g. on transport errors that haven't been traced).
- `status, _, err := c.Await(ctx, k)` -> `outcome, err := c.Await(ctx, k); status := outcome.Status`.
- `status, err := c.RunAndParse(ctx, url, in, opts, &out)` -> `outcome, err := c.RunAndParse(ctx, url, in, opts, &out); status := outcome.Status`.
- Same shape for `Run`, `Snapshot`, `AwaitAndParse`, `SnapshotAndParse`.

For test files with many call sites, define a small helper near the top of the file once and reuse it:

```go
func outcomeStatusState(o *workflow.FlowOutcome) (string, map[string]any) {
    if o == nil {
        return "", nil
    }
    return o.Status, o.State
}
```

then `status, state := outcomeStatusState(outcome)` per call.

**Behavior change to watch for:** `Snapshot` of an `interrupted` flow no longer pre-merges the interrupt payload into `State`. If your code relied on reading the merged view, use `workflow.MergeState(outcome.State, outcome.InterruptPayload, graph.Reducers())` explicitly. Most callers should switch to reading `outcome.InterruptPayload` directly as the resume-request payload.

**Workflow failure detection:** `Run` no longer returns a non-nil Go error for workflow failures. Code like `if err != nil { ... workflow failed ... }` should become `if err != nil { ... transport/foreman error ... } else if outcome.Status == "failed" { ... outcome.Error ... }`.

#### Step 6: Migrate `OnFlowStopped` Hook Handler Signature

```bash
grep -rn "OnFlowStopped(" --include="*.go" . | grep -v "_test.go" | grep -v "/foremanapi/"
```

The hook handler signature changes from `func(ctx context.Context, flowKey string, status string, snapshot map[string]any) (err error)` to `func(ctx context.Context, outcome *workflow.FlowOutcome) (err error)`. The handler now receives a single `*FlowOutcome` with `FlowKey`, `Status`, `State`, plus the populated side-channel (`Error`, `InterruptPayload`, or `CancelReason` depending on Status).

Before:

```go
foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(
    func(ctx context.Context, flowKey string, status string, snapshot map[string]any) error {
        log.Printf("flow %s stopped with status %s", flowKey, status)
        return nil
    },
)
```

After:

```go
foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(
    func(ctx context.Context, outcome *workflow.FlowOutcome) error {
        if outcome == nil {
            return nil
        }
        log.Printf("flow %s stopped with status %s", outcome.FlowKey, outcome.Status)
        if outcome.Status == "failed" {
            log.Printf("error: %s", outcome.Error)
        }
        return nil
    },
)
```

Subscribers that recorded errors or cancel reasons via a follow-up `History` query can now read them directly off the outcome.

#### Step 7: Append a Reason Argument to `Cancel` Callers

```bash
grep -rn "\.Cancel(" --include="*.go" . | grep -iE "(foreman|workflow)"
```

`foremanapi.Cancel(ctx, flowKey)` becomes `foremanapi.Cancel(ctx, flowKey, reason)`. Pass `""` for callers that don't have a reason:

```bash
gofmt -r 'c.Cancel(a, b) -> c.Cancel(a, b, "")' -w <file>
```

The reason string is stored on every flow in the cancellation chain as `cancel_reason` and surfaces as `outcome.CancelReason` for `Snapshot`/`Await`/`Run`/`OnFlowStopped`. Pass a meaningful string when the cancellation is for a known operational reason ("user requested", "tenant suspended", etc.); the empty string is the documented "no reason" sentinel.

#### Step 8: Insert a `nil` Argument Into Foreman `Fork` / `Continue` Callers

```bash
grep -rn "\.Fork(\|\.Continue(" --include="*.go" . | grep -iE "(foreman|workflow)"
```

Only direct calls to `foremanapi.Client` / `foremanapi.MulticastClient` / their mocks are
affected. Append a final `nil` argument (the `*workflow.FlowOptions`):

- `foremanClient.Fork(ctx, stepKey, overrides)` -> `foremanClient.Fork(ctx, stepKey, overrides, nil)`
- `foremanClient.Continue(ctx, threadKey, additional)` -> `foremanClient.Continue(ctx, threadKey, additional, nil)`

An AST-safe `gofmt -r` per file avoids touching multiline arguments by hand, e.g.
`gofmt -r 'c.Fork(a, b, d) -> c.Fork(a, b, d, nil)' -w <file>` (and the same for `Continue`).
Scope it to files that call the foreman client so an unrelated 3-argument `.Fork` elsewhere is not
rewritten.

To actually pass scheduling options through, build a `&workflow.FlowOptions{Priority: ...,
FairnessKey: ..., FairnessWeight: ...}` literal and pass it in place of `nil`.

#### Step 9: Capture the New `nextCursor` Return From Foreman `List`

```bash
grep -rn "\.List(" --include="*.go" . | grep -iE "(foreman|workflowapi)"
```

`Client.List(ctx, query)` now returns `(flows []FlowSummary, nextCursor string, err error)`.
Capture the new value or discard it explicitly:

- `flows, err := foremanClient.List(ctx, query)` -> `flows, _, err := foremanClient.List(ctx, query)`
- For paginated callers, capture the cursor and feed it back: set `query.Cursor = nextCursor` on
  the next call until `nextCursor == ""`.

The `MulticastClient.List` iterator's element type (`*ListResponse`) is unchanged, but
`resp.Get()` itself gained the trailing `nextCursor string`:

- `flows, err := resp.Get()` -> `flows, _, err := resp.Get()`

#### Step 10: Rename Foreman Metric References in Grafana / Alert Configs

```bash
grep -rn -E "(microbus_queue_depth|microbus_pending_steps_by_priority|microbus_oldest_pending_age_seconds|microbus_distinct_fairness_keys|microbus_completion_contention_total|microbus_claim_wait_seconds|microbus_backpressure_backoffs_total|workflow_name|task_name)" --include="*.json" --include="*.yaml" --include="*.yml" .
```

Apply the rename table from "Background" above. The `workflow_name` and `task_name` label
renames also affect PromQL aggregations: `sum by (workflow_name) (...)` becomes
`sum by (workflow) (...)`.

Drop any panel/alert built on `microbus_completion_contention_total` or
`microbus_claim_wait_seconds` - both are gone. The "refiller throughput" question those panels
tried to answer is better expressed as `microbus_steps_pending` vs `microbus_steps_queue_depth`
on one panel (orange/blue overlay); divergence means the refiller is the bottleneck.

#### Step 11: Migrate Sequel Microservices to `CreateTestingDatabase` + `OpenSingleton`

```bash
grep -rln "github.com/microbus-io/sequel" --include="*.go" . | xargs grep -ln "sequel\.\(Open\|OpenTesting\)\b"
```

Every file in the output is a sequel microservice that needs its `openDatabase` function migrated.
The old `sequel.Open`/`sequel.OpenTesting` constructors were renamed to `OpenSingleton`/
`OpenSingletonTesting` (same semantics: per-DSN coalescing, sqrt-managed pool); a new
`sequel.Open` opens a fresh dedicated pool. The per-test database creation is now a separate call,
`sequel.CreateTestingDatabase`, that returns the resolved DSN.

For ordinary SQL CRUD microservices (the bundled-services case - shared pool with other openers
on the same DSN is correct), migrate to `CreateTestingDatabase` + `OpenSingleton`:

```go
// Before
if svc.Deployment() == connector.TESTING {
    svc.db, err = sequel.OpenTesting(driverName, dataSourceName, svc.Plane())
} else {
    svc.db, err = sequel.Open(driverName, dataSourceName)
}
if err != nil {
    return errors.Trace(err)
}

// After
if svc.Deployment() == connector.TESTING {
    dataSourceName, err = sequel.CreateTestingDatabase(driverName, dataSourceName, svc.Plane())
    if err != nil {
        return errors.Trace(err)
    }
}
svc.db, err = sequel.OpenSingleton(driverName, dataSourceName)
if err != nil {
    return errors.Trace(err)
}
```

For services that hold a dedicated pool (foreman-style orchestrators where the pool is sized to a
worker count and must not be silently coalesced with other openers), use `Open` instead of
`OpenSingleton`. Recognize these by either an existing `db.SetMaxOpenConns(...)` / `SetMaxIdleConns
(...)` call after Open, or by an explicit `*ConnectionPool`-style config the service reads to size
the pool. Most SQL CRUD microservices are *not* in this category.

The DSN passed to `CreateTestingDatabase` is the same DSN the production path uses (it parses the
DSN to pick out the base host/credentials and substitutes a per-test database name). Don't
pre-resolve or strip parts of the DSN before passing it.

If your `openDatabase` had an `if dataSourceName == "" && svc.Deployment() == connector.LOCAL`
preamble setting a SQLite fallback path, keep it - it runs before the `TESTING` branch and is
unchanged by this migration.

After editing, build the project and run the microservice's tests to confirm the migration works
end-to-end - the per-test database is materialized lazily on first call, so a compile-clean
migration is not sufficient to prove correctness.

#### Step 12: Defer Manifest and Mock Regeneration

The `Fork`/`Continue`/`List` signature changes and the other source edits above require regenerating each
microservice's `manifest.yaml`, `mock.go`, and `mock_test.go`. Do **not** run a generator here - the
`upgrade-microbus` orchestrator regenerates every microservice's boilerplate from source and verifies the whole
project once, after every numbered skill has run.

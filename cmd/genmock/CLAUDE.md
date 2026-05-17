# cmd/genmock

Generates `mock.go` and `mock_test.go` from the `ToDo` interface in a microservice's `intermediate.go`. Companion to `cmd/genmanifest`: all three artifacts are derived from `intermediate.go` so that the per-feature add-skills don't have to keep multiple parallel surfaces in sync.

## What `mock_test.go` contains

A single `Test<Pascal(pkg)>_Mock(t *testing.T)` function that wires up a `NewMock()`, sets the deployment to `TESTING`, and then runs:

- `on_startup` - smoke-tests `mock.OnStartup(ctx)`.
- `on_shutdown` - smoke-tests `mock.OnShutdown(ctx)`.
- One subtest per `ToDo` method - installs a no-op `MockX(handler)` and calls `mock.X(...)` with zero-valued inputs and a single `assert.NoError(err)` check.

The generated test deliberately does not assert anything about return values. It is a structural smoke test, not a behavioral one - the real per-feature integration tests live in the hand-written `service_test.go`. Subtest bodies branch on the method shape:

- Web handlers (`(w http.ResponseWriter, r *http.Request) (err error)`) use `httpx.NewResponseRecorder` + `httpx.MustNewRequest` and call the mock directly.
- Workflow-graph endpoints install a typed handler reconstructed from the api package's `<Name>In` / `<Name>Out` structs, call `mock.X(ctx)` to fetch the synthetic graph, and assert it is non-nil. Same In/Out reconstruction as the workflow-graph mock variant in `mock.go`.
- Everything else declares `var <name> <type>` locals for each input (skipping `ctx` and `*workflow.Flow`, both of which are passed inline), then calls the mock and asserts no error.

The test function's name uses `Test<Pascal(pkg)>_Mock` where `Pascal(pkg)` is the package directory with its first letter uppercased. This is deterministic; vanity-cased identifiers like `ChatGPTLLM` come out as `Chatgptllm`. The package qualification is implicit (test functions live in the microservice's package), so the prefix is purely a one-line `go test -run` filter.

## `Mock.OnStartup` is a thin deployment guard

The generated `Mock.OnStartup` only enforces that the mock is being run under `LOCAL` or `TESTING` (mocking in `PROD` / `LAB` is rejected). It does not iterate subscriptions or activate manual ones - that responsibility moved into the connector itself: in `TESTING` deployment, `activateSubs` ignores the `sub.Manual` flag and brings every registered subscription on-bus. That keeps `RunInTest` ergonomic for mocks (Python-style services with manual subs are reachable without per-test setup) while leaving real `LOCAL`/`PROD` behavior unchanged.

## Why the `ToDo` interface is the single source

Every mockable method already lives on `ToDo` - that's how `Intermediate` delegates into `Service`, and the framework's plumbing depends on each `ToDo` method existing on both `Service` and `Mock`. The shape of `mock.go` is therefore fully determined by:

1. The method names and Go signatures on `ToDo` (skipping `OnStartup` / `OnShutdown`, which the mock implements directly).
2. The `// MARKER: <Name>` trailing comment on each method, which is preserved so `modify-feature` / `remove-feature` can still find the regenerated entries.
3. The In/Out structs in `*api/endpoints.go`, but only for workflow graphs (see below).

No other inputs are required. Custom types referenced by `ToDo` signatures (e.g. `bearertokenapi.JWK`, `time.Time`) are resolved against `intermediate.go`'s own import block - if `ToDo` references it, the file already imports the package.

## The var-guard block is gone

Old hand-written `mock.go` files had a `var ( _ http.Request; _ json.Encoder; ... )` block immediately after the imports. It existed because the templates imported a fixed superset of packages and used the blank-identifier vars to silence "imported and not used" errors. Generated `mock.go` does not need this: the generator emits *only* the imports it actually uses, so every import is referenced by real code. Removing the guard block keeps the file shorter and removes a maintenance hazard (forgetting to add a new guard line when adding a new keeper-import).

## Unmocked methods return zero values

Calling a method on a `Mock` whose handler was never set returns the zero value for every named result (including `nil` for `error`). The generated body is:

```go
if svc.mockX != nil {
    out1, out2, err = svc.mockX(ctx, ...)
}
return out1, out2, errors.Trace(err)
```

For methods that return only `(err error)`, the same shape collapses to:

```go
if svc.mockX != nil {
    err = svc.mockX(ctx, ...)
}
return errors.Trace(err)
```

This is a deliberate trade-off. Earlier versions returned `errors.New("mock not implemented", http.StatusNotImplemented)` to catch "I forgot to mock this" early and loudly. In practice the cost was paid on every test (every downstream mock had to enumerate every endpoint, even ones the test didn't care about), and a test that quietly hits an unmocked endpoint usually fails anyway when its real assertion doesn't match. The new default trades that rarely-useful guard for less boilerplate across every mock in the project.

Methods that declare a bare `error` return (e.g. `Hello(w, r) error` rather than `Hello(w, r) (err error)`) are normalized: the result is renamed to `err` internally so the same emission template applies.

The workflow-graph mock variant follows the same convention: an unmocked graph endpoint returns `nil graph, nil err`. The foreman will surface a clear failure downstream if a workflow is invoked without a mocked graph; the mock doesn't need to pre-empt it with a 501.

## Workflow graph mock variant

A `ToDo` method of the form `Method(ctx context.Context) (graph *workflow.Graph, err error)` is a workflow-graph endpoint, not a regular function. Its mock cannot simply dispatch to the user's handler the way other mocks do, because callers don't invoke the graph directly - they execute its tasks via the Foreman runtime, which posts state to subscribed task URLs.

`MockMyWorkflow(handler)` therefore:

1. Subscribes a synthetic task endpoint on `:428/mock-<kebab>-<rand>` whose body decodes the incoming `workflow.Flow`, parses the workflow's `In` struct out of state, calls the user's typed handler, and writes the `Out` struct back via `f.SetChanges`.
2. Replaces the workflow's graph with a single-transition graph that targets the synthetic task and ends. `DeclareInputs("*")` / `DeclareOutputs("*")` are emitted so the subgraph passes all state through to the mock task and back; without them the framework's `FilterState` returns an empty map and the handler sees zero-valued inputs.
3. Stores an `unsubMock<X>` callback so subsequent `MockX(...)` calls (and `MockX(nil)` to clear) can tear down the previous subscription cleanly.

The handler signature is reconstructed from the api package's `<Name>In` and `<Name>Out` structs. Bare type identifiers in those struct fields are qualified with the api package alias when rendered into the parent package (`Applicant` -> `creditflowapi.Applicant`), since the structs themselves live in the api package but the mock does not.

When the handler is called, body lines read `in.<RawGoFieldName>` and the output struct literal uses `<RawGoFieldName>: <argName>`, where `argName` is the Go field name lowercased on its first letter. This preserves the `Out` suffix used by read-modify-write workflow fields and keeps the rendered signature valid Go even when an input and output share a JSON tag.

## Method ordering

Hand-written `mock.go` files put regular endpoints first and `OnChangedXxx` config callbacks at the end, regardless of where they appear in the `ToDo` interface. The generator follows that convention: methods whose names start with `OnChanged` are emitted after the rest. Within each group, the order of `ToDo` is preserved.

This is purely cosmetic but matches longstanding convention, so the regeneration diff against an existing hand-written `mock.go` is smaller and reviewable line by line.

## What about descriptions, queue options, required claims?

Those properties live on the `svc.Subscribe(...)` call in `intermediate.go`, not on the `ToDo` method. The mock doesn't need them - it stubs the *method* and lets the rest of the framework (or the real intermediate) carry the subscription metadata. So genmock does not parse anything beyond the `ToDo` interface, intermediate.go's import block, and (for workflow graphs) the api package's In/Out structs.

## Round-trip and idempotency

Running genmock once is enough; running it a second time over its own output is a no-op. `TestRoundtrip_RealServices` enforces this for every committed service. The first invocation produces canonical output; the second must read its own output and produce the same bytes.

This matters because:

- Skills invoke genmock right after they touch `intermediate.go`, then again at housekeeping time as a safety net. A non-idempotent generator would produce phantom diffs on the second invocation.
- The `--check` flag is the audit guard: CI invokes `genmock --check --path <dir>` and any divergence between committed `mock.go` and what the generator would emit is a hard failure.

## What about manifest reuse

We deliberately do not consult `manifest.yaml`. The manifest is itself a derived artifact (produced by genmanifest from the same `intermediate.go`); routing the mock through the manifest would couple two generators and introduce ordering hazards. Both tools read `intermediate.go` directly.

## Limits

- Workflow-graph mocks require the api package's `<Name>In` and `<Name>Out` structs to exist. A workflow declared on `ToDo` without those structs falls back to the standard mock pattern (which still compiles but loses the typed handler convenience).
- Type expressions are rendered with the same support set as `cmd/genmanifest`'s `exprString`: identifiers, selectors, pointers, slices, maps, arrays, `any`, and func types. Generic type parameters and channel types aren't supported because the framework's surface doesn't use them.
- The leading comment block (license header, banner, anything before the `package` clause) is reused verbatim from the existing `mock.go` if present. The tool does not synthesize a license; operators who want one add it once and the tool preserves it on every subsequent run.
- Every generated `mock.go` and `mock_test.go` always carries the line `// Code generated by cmd/genmock. DO NOT EDIT.` immediately before the `package` clause (after the reused license header, if any). This is the Go-standard machine-generated marker (`^// Code generated .* DO NOT EDIT\.$`), recognized by the toolchain. `reuseHeader` strips any prior marker out of the captured header so it is re-emitted exactly once - the generator stays idempotent (`TestRoundtrip_RealServices` / `--check`). These files must never be hand-edited; change `intermediate.go` and regenerate.

## What's intentionally NOT in mock.go

- `_ pkg.Symbol` keeper vars - the generator only emits used imports.
- A `package <name>api` import when no method signature references the api package - new microservices with no `ToDo` entries have a minimal mock with just `context`, `connector`, and `errors`.
- Per-method godoc beyond the one-line summary - the real godoc is on `ToDo` and on `Service`; the mock just dispatches.

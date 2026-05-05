# cmd/genmanifest

Generates `manifest.yaml` from a microservice's source code. Inverse of the `regenerate-boilerplate` skill (which goes manifest → code).

## What this tool does NOT do

genmanifest produces an interface-and-dependency snapshot - what this service *exposes* (configs, metrics, endpoints, events) and what other services it *depends on* (downstream hostnames + packages). It deliberately does NOT record per-call route detail (which routes/methods this service invokes on each downstream). That data is derived from source by `cmd/gencreds` at deploy time, where it can be resolved against the live `*api/endpoints.go` of each dep without going stale when callees rename Defs.

This separation means a route rename in service A does not require regenerating service B's manifest - B's manifest only references A's hostname + package, both of which are stable across callee Def renames. The cascade-regen story that earlier versions of the framework needed is gone.

## Why intermediate.go is the primary source

Endpoint declarations are canonical in `intermediate.go`: `Subscribe(...)` blocks carry `sub.At(api.X.Method, api.X.Route)`, `sub.Description(...)`, `sub.Function`/`Web`/`Task`, `sub.NoQueue()`, and `sub.RequiredClaims(...)`. Same for `DefineConfig`, `DescribeCounter`/`Gauge`/`Histogram`, and `StartTicker`. The `ToDo` interface lists method signatures. Recorder methods (`IncrementX`/`RecordX`) provide metric labels.

## Workflow signatures use Go field names, not JSON tags

Function and outbound-event signatures come from the `ToDo` interface or method godocs - their field names are taken directly from the Go source. Workflow signatures are different: a workflow's `ToDo` method returns `*workflow.Graph`, not the workflow's input/output fields, so we have to reconstruct the signature from its `In`/`Out` structs in `*api/endpoints.go`.

When doing that reconstruction, we use each struct field's **Go name** (lower-cased on the first letter) rather than its JSON tag. The two differ when an output field uses the `Out` suffix to disambiguate from an input that shares its underlying state-field name - e.g. `ChatLoopOut.ListMessagesOut json:"listMessages"`. Using the JSON tag would produce `(listMessages []Message, ...)` for the output, which collides with the input parameter name and is invalid Go. Using the Go name preserves the `Out` suffix so the rendered signature compiles.

This makes the workflow signature consistent with task signatures (which always use Go arg names with `Out` preserved) and means the signatures in `manifest.yaml` are valid Go. Outbound events still use JSON tags - their fields don't carry the input/output naming collision.

Outbound events live in `*api/endpoints.go` (Def) and `*api/client.go` (godoc on `MulticastTrigger.OnX`) - they don't appear in `NewIntermediate` because the trigger is published by the source service but registered via `Hook` on the sink side. We extract description from the godoc only for outbound events.

## The manifest is overwritten - code is authoritative

Every regen rewrites `manifest.yaml` from scratch from the source files. Hand edits will be lost; the manifest is not meant to be edited. The single exception is the leading comment block (license header, banner, anything before the first blank line), which is reused verbatim from the existing file. The tool does not synthesize one - fresh manifests, or files with no leading comments, get no header. Operators who want a license header add it once and the tool preserves it on every subsequent run.

This means:

- `general.description` comes from `svc.SetDescription(...)` in `intermediate.go`. Edits to the manifest's description field are overwritten.
- Endpoint descriptions come from `sub.Description(...)`. Same story.
- Outbound-event descriptions come from the godoc on the `MulticastTrigger.OnX` method in `*api/client.go`.
- Ticker / inbound-event-handler descriptions fall back to the first-sentence godoc on the `Service` method when the framework offers no in-code description hook.

If you want to change a description, change the code that produces it and re-run genmanifest. The OpenAPI doc and LLM tool builders read from the same source - keeping descriptions in code keeps all three surfaces in sync.

## Why a few fields are preserved across runs

The manifest is otherwise overwritten, but three `general.*` fields can't be derived from code today:

- **`general.name`**: PascalCase decorative name (e.g. `CreditFlow`, `HTTPIngress`). The package directory uses lowercase (`creditflow`, `httpingress`); recovering the operator's preferred capitalization without a dictionary is lossy. So we read the existing manifest's value through.
- **`general.frameworkVersion`**: comes from `go.mod`, not from the service source. We currently read it from the existing manifest rather than running `go list -m` per regen - but it could be computed.
- **`general.cloud`**: `"various"` is the default when `httpegressapi` is imported. Operators who want a specific value (`api.openai.com`) write it once and the tool keeps it on subsequent runs. If the egress import is removed, the field is dropped.

`modifiedAt` is bumped to current UTC **only when other fields actually change**. The tool first renders with the prior `modifiedAt` baked in; if that output byte-matches the existing file, it returns without writing. This is what makes `--check` usable - otherwise the timestamp would advance on every run and `--check` would always report a diff.

Everything else (`hostname`, `description`, `package`, `db`, all endpoint sections) is regenerated from code on every run.

If we wanted to be strict about "manifest is fully derived from code," `frameworkVersion` and `package` could be moved to `go list` lookups, and `name` could be computed from the package directory (with the loss noted above). The current code makes the practical trade-off of accepting one operator-curated field (`name`) and one operator override (`cloud`).

## Acronym handling in pascalToSnake

`LLMTokens` → `llm_tokens`, not `l_l_m_tokens`. Same for `SQLDataSourceName` → `sql_data_source_name`. The algorithm: insert `_` before a capital only if the previous rune was lowercase OR the next rune is lowercase (start of a new word after an acronym run). The substring match against the OTel metric name (e.g. `microbus_llm_tokens_total`) needs this to recover the manifest name from the recorder method.

## Round-trip with regenerate-boilerplate

genmanifest emits enough information that `regenerate-boilerplate` can reconstruct `intermediate.go`, the `*api` package, and `mock.go` from it alone. Anything not extractable from code (operator-curated `general.name`, etc.) is preserved across regeneration; anything not in the manifest (godoc on private helpers, comments inside service.go) is not part of the contract.

The two are inverses but each is standalone - neither knows about the other. If a round-trip introduces a diff, that's a bug in one of them or stale data in the input.

## Why no schema package import

`cmd/schema` is the consumer shape (just enough for `cmd/gencreds` to read hostnames + downstream package paths from a manifest). genmanifest defines its own richer internal types because it needs the full set of fields, including ones gencreds doesn't care about (signatures, metric kinds/buckets, ticker intervals). Don't fold genmanifest's types back into `cmd/schema` - that would force gencreds to depend on the full superset.

## Custom YAML emitter

We don't use `yaml.v3`'s emitter because it produces inconsistent quoting (e.g. quotes integers-as-strings inconsistently) and key ordering. `emit.go` is ~280 lines of explicit emission with deterministic key order per section. If output formatting needs to evolve, edit it there directly rather than reaching for a higher-level library.

## Testing strategy

Three layers of tests:

### Targeted unit/integration tests against real services

`TestExtract_Foreman`, `TestExtract_CreditFlow`, `TestExtract_DownstreamDependencies`, `TestExtract_InboundEventDependency`, etc. run `extract()` against committed production services and assert on specific fields. Cheap and fast - they catch regressions in detection logic that would surface as wrong manifest content for the framework's own services.

The downside: edge cases that don't happen to exist in production code (e.g. a Def with a `//host:port/path` route used by another service as a downstream call) go uncovered.

### Inline AST fixtures for narrow tests

`TestExtract_InboundEvent_ForHostChain` builds minimal source-bytes fixtures inline (`os.WriteFile` of `package …` strings), drops them in a temp dir under `testdata/`, and runs the extractor. Useful when a single edge case needs an isolated repro without a full microservice; the bytes ARE the fixture, no separate file maintenance.

The downside: the inline source isn't compiled, so it's possible to write a fixture that parses but wouldn't be valid Go. Drift between the framework's exported types and what the fixture pretends to be is invisible until production code breaks.

### Kitchen-sink fixture services under `testdata/`

`testdata/weird/` and `testdata/kitchen/` are full microservices that compile against the actual framework. They exist specifically to exercise edge cases the framework's own services don't all hit:

- **`weird/`** - the downstream. 11 Defs covering every route shape: `:port/path`, single path arg, greedy `{tail...}`, period-in-segment, `ANY` method, `:444` internal port, `:666` trust-root, `//root`, `//host:port/path`, `//host/path` with arg, `:417` outbound event. `weirdapi/clientext.go` exposes a helper method that wraps another Def, so the helper-expansion path is exercised.
- **`kitchen/`** - the consumer. `service.go` has 14 numbered patterns covering every binding shape (chain, variable, parameter-typed, named result, var decl) crossed with every dispatch shape (literal ForHost, varExpr ForHost, multicast, helper expansion, self-call, self-call with ForHost, raw `svc.Request` inline / slice / append, trust-root call). The intermediate.go subscribes to weird's outbound event so inbound-event hook resolution is covered.

Both fixtures' `manifest.yaml` files are committed and serve as goldens. `TestKitchenFixture_ManifestGolden` and `TestWeirdFixture_ManifestGolden` re-extract into a temp copy and assert byte equality against the committed file. Any change to the AST detection that flips a binding shape's resolution makes the golden test fail with a localized diff, and the operator deciding whether to update the golden is the audit trail.

When a scanner or emitter change is intentional, refresh the goldens with `go test ./cmd/genmanifest/ -update`. The test runs as usual but writes the regenerated `manifest.yaml` back over the committed file when content differs. Review the diff in `git status` and commit. Without `-update`, drift fails the test with a side-by-side want/got dump.

### Why the kitchen fixture compiles

We could have built the fixture as inline source bytes (the cheap option) or as a fully scaffolded compileable service (the expensive option). We chose compileable.

The argument: when the framework's exported types change (`connector.Connector`, `service.Publisher`, `sub.Function`'s signature, `pub.Option`'s shape), the fixture either compiles against the new shape - proving the AST detection still works on realistic code - or fails to compile, which is a much louder, more debuggable failure mode than "the test still passes but production code now drifts." The fixture is small enough that maintenance cost is bounded; one method per pattern, no real handler logic, no `mock.go` or `service_test.go` to keep in sync.

Two downstream test layers consume these fixtures and rely on them being real importable packages:

- `cmd/gencreds`'s scan-against-golden test (`TestScan_FixtureGoldens`) runs the source-driven AST scan + rule construction against each fixture and asserts the result matches the committed `nats.acl` golden in the same directory. The fixtures' nats.acl files are pinned by code review.
- `cmd/gencreds`'s end-to-end test composes both fixtures into an `application.New()` bundle, signs per-service `.creds` against an embedded `nats-server`, and drives `kitchen.MyFunc` to exercise every detection pattern through real NATS.

Both layers would be impossible if the fixtures were inline source bytes - they need to compile, link against the framework, and start up like any other microservice.

### What's intentionally NOT in the fixtures

- No `mock.go` - fixtures are not mocked anywhere; they're consumed via path, not via import.
- No `service_test.go` - fixtures are themselves test inputs, they don't have their own tests.
- Not added to `main/main.go` - fixtures aren't shipped.
- No real handler bodies - `MyFunc(ctx, input string) (output string, err error) { return "", nil }`. Behavior isn't being tested.
- No multiple instances of the same pattern - one of each is enough.

If a future test needs an edge case the fixtures don't cover, add a Def to `weird` and a call site to `kitchen`. Don't fork a new fixture pair.

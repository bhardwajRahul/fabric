# cmd/genservice

Generates a microservice's boilerplate from a single typed source of truth: the `*api/definition.go` file. From
that one file genservice emits five artifacts:

- `*api/client.go` - the client proxies (`Client`, `MulticastClient`, `Executor`, `Subflow`, `MulticastTrigger`,
  `Hook`) and their marshaling helpers.
- `intermediate.go` - the `ToDo` interface, `NewService`/`Init`/`NewIntermediate`, `Subscribe` wiring, `doXxx`
  marshaling, config/metric/ticker registration, and inbound hook wiring.
- `mock.go` + `mock_test.go` - the mockable `Mock` and a structural smoke test.
- `manifest.yaml` - the derived navigational view of what the microservice exposes.

This is the forward generator of the `definition.go` redesign. It is meant to subsume the older `cmd/genmanifest`
(manifest) and `cmd/genmock` (mock) tools, which read the previous source of truth (`intermediate.go` +
`*api/endpoints.go`). The three coexist during the migration: genservice only runs on a microservice that already
has a `definition.go`, while genmanifest/genmock keep serving the not-yet-migrated services. Once every service is
migrated and the housekeeping skill switches to genservice, the older tools are deleted.

## The pipeline: parse once, project many times

`extract.go` parses every non-test `.go` file in the api package into one `service` model: the api package name,
the `Hostname`/`Name`/`Version`/`Description` consts, the ordered list of `feature`s (one per `define.*` var), the
In/Out and domain `struct`s, and the file's import aliases. Every emitter is a pure projection of that one model,
so the five outputs cannot drift from each other - they are computed from the same tree in a single pass.

A `feature` keeps the var's raw keyed fields as `map[string]ast.Expr` (`attrs`) plus the godoc and the resolved
In/Out type names. Keeping the raw AST expressions lets each emitter render exactly what it needs (a duration as
`5s` for the manifest but as `5 * time.Second` for a `sub.TimeBudget` call) without the extractor having to
anticipate every consumer.

### Literals only

Field values in `definition.go` must be statically resolvable by AST walking: constant literals, struct composite
literals used as type carriers (`In: GreetIn{}`), `define.*` constants (`define.None`), and cross-package
references (`Source: srcapi.OnRegistered`). There is no `go/types` pass and no evaluation. This is the price of
keeping `definition.go` a declarative spec rather than code with logic, and it is why the `define` package models
config and metric value types as explicit type carriers (`Value: int(0)`) instead of inferring them.

## Why text/template for the Go files but a hand emitter for YAML

`client.go`, `intermediate.go`, `mock.go`, and `mock_test.go` are rendered from `//go:embed`-ed `.txt` templates
(`templates/`), then run through `format.Source` (gofmt). Go is gofmt-normalized afterward, so the templates only
have to be *valid* Go, not *pretty* Go: alignment, import grouping, and spacing are all fixed by gofmt. Templates
keep the code skeleton readable as code rather than buried in string concatenation.

`manifest.yaml` is emitted by hand in `manifest.go` instead. YAML has no gofmt to lean on: quoting, key order, and
block-scalar formatting all have to be exactly right in the emitted bytes. A custom emitter with deterministic key
order per section is more predictable than `yaml.v3` (which quotes inconsistently and reorders keys), and it
matches the byte-for-byte conventions the previous `cmd/genmanifest` established.

Do not embed Go (or YAML) source as string constants inside the `.go` emitters. Code skeletons live in the `.txt`
templates; the emitters build the data model and compute the small fragments (signatures, import sets) that the
templates interpolate.

## Type qualification: the `apiPkg` opt-in

In/Out struct fields are declared in the api package. `client.go` lives in that same package, so it refers to a
domain type by its bare name (`Pet`). But `intermediate.go` and `mock.go` live in the *service* package one level
up, where the same type must be written `svcapi.Pet`.

`featureView` carries an `apiPkg` field that is empty for the client (bare types) and set for the intermediate and
mock (qualified types). `Params()`/`Returns()` run field types through `qualifyTypes`, which prefixes a bare,
non-builtin identifier with `apiPkg` while leaving already-qualified selectors (`time.Time`,
`*errors.TracedError`) and builtins alone. Inbound-event views qualify with the *source* package alias instead,
since an inbound handler's parameters are the source event's types.

Because qualified field types pull in packages the api struct imported (`time`, another api package),
`featureSelectorImports` walks every In/Out field and resolves its `pkg.Type` selectors against the api package's
own import aliases, feeding the computed import set of every emitter. This is shared by client, intermediate, and
mock so the three agree on imports.

## Feature-selective emission, conditional imports, no var guards

The client emits only the proxy types a microservice actually needs: no `MulticastTrigger` without outbound
events, no `Executor`/`Subflow` without tasks or workflows, and only the `marshalXxx` helpers the emitted methods
call. Imports are computed in Go from the feature mix (see `buildClientModel`) and emitted conditionally, so every
import is referenced by real code. There is therefore no `var ( _ = pkg.Symbol )` guard block: the old guard
blocks existed only because the templates imported a fixed superset, and emitting just-what-is-used removes the
need.

genservice deliberately does not depend on `goimports` to fix up imports. `goimports` is not part of the standard
Go toolchain, so it cannot be assumed present on every machine that builds the project. Imports are computed
explicitly instead.

## Header preservation, never synthesis

No emitter writes a copyright header. Each one preserves the leading comment block of the file it is regenerating
(`existingHeader` for the Go files, `manifestHeaderOf` for the manifest), stripping any prior
`Code generated ... DO NOT EDIT` marker so it is re-emitted exactly once. A fresh file gets no header; an operator
who wants one adds it once and the generator keeps it on every subsequent run. This keeps genservice out of the
business of choosing or dating a license.

## Service-level consts live in definition.go

`definition.go` declares four consts that define the microservice's identity: `Hostname`, `Name` (the decorative
PascalCase name), `Version` (the API major version), and `Description`. The generated `intermediate.go` references
all four symmetrically:

```go
const (
    Hostname    = svcapi.Hostname
    Version     = svcapi.Version
    Description = svcapi.Description
)
```

`Description` is a `const` value rather than a doc comment for two reasons. First, a downstream project may run a
linter that forces every godoc to begin with the symbol it documents (a package doc must start `Package xxx`, a
`Hostname` doc must start `Hostname`), which makes prose-in-a-comment unusable as a clean, free-form description. A
plain string value sidesteps the convention entirely and supports multi-line text via a backtick raw string.
Second, the symmetric reference block above only compiles as const-init-from-const: `const Description =
svcapi.Description` requires `Description` to itself be a const, not a var.

`Name` cannot be derived: the package directory is lowercase (`creditflow`) and recovering the operator's intended
capitalization (`CreditFlow`) without a dictionary is lossy, so it is authored once in `definition.go`. genservice
errors if a service-directory generation finds no `Name`, `Version`, or `Description` const, rather than emitting a
file that references a missing symbol.

## The ToDo interface is generated and load-bearing

`NewIntermediate(impl ToDo)` takes an interface, not a concrete type, so the same constructor accepts both
`*Service` (production) and `*Mock` (tests). `ToDo` also doubles as the compile-time proof that `*Service` and
`*Mock` implement every handler. It is generated from the feature set (one method per function, web, task,
workflow, inbound event, ticker, observable metric, and config callback) and must stay an interface for that
polymorphism to hold.

## Manifest specifics

`manifest.go` builds the `general` block as `{name, hostname, description, package, modifiedAt}`. The decisions
behind that set:

- `name`/`hostname`/`description` come from the `definition.go` consts; `package` is computed from `go.mod` +
  directory via `importPathOf`.
- `frameworkVersion` was dropped: no Go code reads it and it is not navigational.
- `db`/`cloud` were dropped: they never appear in a committed manifest, and `cmd/gentopology` derives them itself
  by scanning service source code.
- `modifiedAt` is the only stateful field. `emitManifest` renders once with the prior timestamp, and if the bytes
  match the existing file it keeps that timestamp (content unchanged); only when other content actually changed
  does it re-render with the new timestamp. This render-twice dance is what makes regeneration idempotent and
  `-check` stable, since otherwise the timestamp would advance on every run.

Signatures use each field's Go name lower-cased on the first letter (`lowerFirst(goName)`) uniformly across
functions, tasks, workflows, inbound and outbound events. This preserves an `Out` suffix (`countOut`), which a
JSON tag would collapse (`count`) into an invalid or colliding Go signature, and it matches the param names of the
generated client stubs. Config signatures are `Name() (value Type)`, matching the generated getter (whose named
return is `value`); ticker signatures are `Name()`; metric signatures are `Name(value Type, label string, ...)`.
Types are left as declared in the api package (no qualification), because the manifest documents the api-level
contract.

## The workflow-graph mock variant

A workflow's `Mock` cannot simply call the user's handler the way a function mock does, because callers do not
invoke the graph directly: the Foreman runtime executes its tasks by posting state to subscribed task URLs.
`MockMyWorkflow(handler)` therefore subscribes a synthetic task on `:428/mock-<kebab>-<rand>` that decodes the
incoming flow, parses the workflow's In struct, calls the typed handler, writes the Out struct back, and replaces
the graph with a single transition to that task. This mirrors the variant in the old `cmd/genmock`.

## Mode detection

`emitAll` decides what to generate from the directory shape: a directory that itself contains `definition.go` is
an api package, and only `client.go` is produced. A directory with an `<x>api` subdirectory containing
`definition.go` is a service directory, and all five artifacts are produced. Import paths (api package, resources,
service package) are computed by walking up to the nearest `go.mod` (`findModule`) and doing string math against
the module root (`importPathOf`, `inModuleDir`).

## Cross-package inbound events

An `InboundEvent.Source` is a typed reference to another api package's `OutboundEvent` var. To generate the hook
wiring and the handler signature, genservice must read that source package's In/Out structs. `resolveSource` maps
the source import path to its on-disk directory (string math against the module root, no `go list`) and parses it
into another `service` model. This only works for sources inside the same module, which holds for every
intra-project event.

## -check and golden tests

`-check` regenerates in memory and compares against the files on disk: it writes nothing and exits 2 if any output
is stale (a CI guard), 1 on error, 0 when current. It shares the exact `emitAll` collection path with normal
generation, so what `-check` validates is what a write would produce.

`genservice_test.go` runs the generators against the `testdata` fixtures in place and asserts byte equality
against the committed output (`TestGoldens`, refreshable with `-update`); `TestManifestModifiedAtStable` pins the
timestamp-stability property. The goldens are compared in place rather than in a temp copy, because service-
directory generation needs `findModule`/`go.mod` resolution, which fails for a directory copied outside the module
tree. The committed fixture files therefore *are* the goldens.

## Fixtures

- `testdata/svc` is a full service exercising every feature kind: functions (with magic HTTP args and the `xxxOut`
  task suffix), a web handler, tasks, a workflow, a cross-package inbound event (sourced from
  `pressuretest/srcapi`), counter/gauge(observable)/histogram metrics, secret/callback/duration configs, a
  ticker, plus a domain type (`Pet`) and an external type (`time.Time`) to exercise qualification across the
  service-package files.
- `testdata/pressuretest/{srcapi,svcapi}` are api-only packages (client.go generation) covering every client type
  and helper, magic HTTP args, the `xxxOut` suffix, and a cross-package type alias.

## What genservice does not touch

- `*api/clientext.go` - hand-written client extensions that cannot be derived from `definition.go`. Never read or
  written.
- `service.go` handler bodies and `OnStartup`/`OnShutdown` - the hand-written half of the microservice.
- Anything outside the api package and the service directory.

## Known limitations

- The import-alias heuristic uses the last path segment as the package name, which is wrong for a `/v2`-style
  module path. This matches the assumptions of the older generators and has not bitten any project package.
- Embedded In/Out struct fields are not flattened; In/Out types are assumed to be declared in the api package.
- Metric labels are assumed to be string-typed, which holds for every current metric.

# cmd/genupgrade

The deterministic arm of the versioned upgrade skills. Each fabric version that requires a mechanical
source-code change registers an upgrade routine here, and the matching `upgrade-vX-Y-Z` skill invokes
`genupgrade -v <version> -path <microservice dir>` for the mechanical part while handling the judgment parts
itself. genupgrade operates on one microservice directory at a time.

It never calls other generators. The `upgrade-microbus` orchestrator runs genservice once, after every numbered
skill has applied its source transformation (see "Why upgrade skills may invoke only genupgrade" below):

```
step 1: genupgrade -v 1.41.0 -path .   # numbered skill rewrites the spec (per microservice)
step 2: genservice  (on each microservice directory)   # orchestrator regenerates boilerplate, once, at the end
```

## Scope: -path is whatever the version operates on

`-path` is not assumed to be a microservice directory. It is simply the directory a given version's routine
operates on. A routine defines its own scope and the paired upgrade skill invokes genupgrade to match:

- A **per-microservice** upgrade (like 1.41.0): the skill loops over microservice directories, one
  `genupgrade -v X -path <each>` per service.
- A **project-root** upgrade (rewrite `main/main.go`, bump `go.mod`, edit `config.yaml`, ...): the skill calls it
  once, `genupgrade -v X -path .`, and the routine does root-level work.

The CLI is identical for both and the registry value is already the generic `func(dir string) error`, so a
root-scoped routine drops in with no dispatch or flag changes. The one shape this does not pre-express is a
single version needing both a per-service pass and a root pass; handle that when it arrives (the skill can call
genupgrade twice with different paths, or the routine can branch on what it finds at `-path` - an `<x>api`
subdirectory means a microservice, a `main/main.go` or `go.mod` means the root - or add a `-step` selector
then). Do not add that machinery speculatively.

## Why a versioned tool, not a one-shot migrator

The endpoints.go -> definition.go migration is not one-time. Every downstream project that bumps to fabric
v1.41.0 needs the same transform, so the logic must live on indefinitely, not be deleted after this repo's own
sweep. Framing it as a versioned upgrade tool also fits the existing `upgrade-vX-Y-Z` skills: those are the
agent/judgment arm, and genupgrade is their deterministic counterpart.

## Append-only, frozen registry

`upgrades` maps a target version to its routine. A routine describes the one-time change a specific version
introduced and is **frozen once shipped** - never rewritten, only new versions appended - exactly like a
versioned upgrade skill. An unknown `-v` errors with the supported list. Once every downstream is past a
version its routine is dead weight, but it stays for the historical record (pruning is a separate decision).

## Why upgrade skills may invoke only genupgrade

A numbered `upgrade-vX-Y-Z` skill is a frozen, versioned artifact, but `upgrade-microbus` runs it against the
**target** framework version's `go.mod` (it downloads the latest `.claude` and `go get`s the latest fabric before
following any numbered skill). So every `go run github.com/microbus-io/fabric/cmd/<tool>` in a numbered skill
resolves to a *future* version's binary. That is only safe for a tool guaranteed to exist at every later version.
genupgrade is the one such tool - append-only and frozen, its routines never pruned - so the latest binary always
carries the routine a given `-v X.Y.Z` names. The boilerplate generators do **not** have this property: `genmanifest`
and `genmock` were superseded by `genservice` at 1.41.0, so a skill frozen at, say, 1.39.0 that ran `genmock`
detonates the moment its chain targets a version where `genmock` is gone. Verification has the same defect from the
other direction: `go vet`/`go test` mid-chain check a half-migrated tree against the final framework and fail.

The invariant, therefore: **a numbered skill invokes only `genupgrade` (plus pure-shell `sed`/`perl`/`grep` edits,
which depend on no fabric tool); it never runs a boilerplate generator and never verifies.** Regeneration with the
current generator and the single `go mod tidy && go vet ./... && go test ./...` are owned by `upgrade-microbus`'s
final step, the one point where the fully-migrated tree is expected to compile. When cutting a version that needs a
mechanical transform, ship it as a genupgrade routine here - not as a generator call or a verify step baked into the
frozen skill.

## The 1.41.0 routine: endpoints.go -> definition.go

It synthesizes `<x>api/definition.go` and deletes `<x>api/endpoints.go`. It reads three inputs, manifest-first:

| Source | Provides |
|---|---|
| `manifest.yaml` (primary) | the feature list and kind (by section), descriptions, route, claims/timeBudget/loadBalancing for functions/webs/tasks/outbound, and all config/metric/ticker/inbound detail |
| `endpoints.go` | In/Out and domain `type` declarations (lifted verbatim), the `Hostname` const, and the imports those types need |
| `intermediate.go` | the `Version` counter (regexp) and subscription options the manifest does not record |

The manifest is the spec; the two supplements fill its gaps.

### Why the supplements are needed

The manifest is a lossy projection. Two gaps force reading code:

- **Method for tasks and workflows.** The manifest omits it for those kinds. It is not recovered from
  endpoints.go either: tasks are always `POST` and workflows always `GET` - a foreman invariant (a task takes
  its flow state in a POST body; a workflow returns its graph via GET), verified across every `:428` endpoint in
  the tree. So the method is the manifest's value for functions/webs/outbound, and the kind constant for
  tasks/workflows.
- **Subscription options for workflows.** The manifest records `requiredClaims`/`timeBudget`/`loadBalancing` for
  functions/webs/tasks but drops them for workflows. Dropping a `requiredClaims` silently would be a security
  regression once genservice regenerates from the lossy spec, so genupgrade scans intermediate.go's
  `svc.Subscribe(...)` options directly. No workflow in this repo currently sets them, but the scan ensures a
  downstream one is never lost. `timeBudget` is captured as the verbatim Go expression (`5 * time.Second`), not
  the manifest's compact `5s`, so no round-trip is needed.

### Version is a build counter, preserved

`Version` (e.g. 353) is an auto-incremented counter, not a semantic version, and it lives only in
intermediate.go (the manifest carries `frameworkVersion`, a different thing). Resetting it to 1 would regress
`svc.SetVersion`, so it is lifted verbatim via a one-line regexp.

### Verbatim type lifting

In/Out and domain `type` declarations are copied byte-for-byte from endpoints.go (AST locates each decl,
including its doc comment, then the original source is sliced between the declaration's start and end offsets).
This preserves json/jsonschema tags, MARKER comments, and formatting exactly, which AST re-rendering would
disturb. The `Def` routing struct and its `URL()` method are dropped (replaced by the define package); the
`Def` var block is dropped (route/method come from the manifest). Domain types declared in sibling files
(`point.go`, `applicant.go`) are left untouched - only endpoints.go is rewritten.

### Inbound event options are not migrated

`define.InboundEvent` carries `RequiredClaims`, `TimeBudget`, and `LoadBalancing` (the consumer-side
`NewHook(svc).WithOptions(...)` knobs). The 1.41.0 routine does **not** recover them: the manifest never recorded
inbound-event options (its `inboundEvents` entries hold only `signature`/`description`/`package`), and they lived
solely in the hand-written `NewHook(svc).WithOptions(...)` binding in `intermediate.go`, which the routine does
not scan. No microservice in this repo ever set inbound options, so the sweep lost nothing. A downstream that
relied on `WithOptions(sub.NoQueue()/sub.RequiredClaims(...))` on an inbound hook must re-add the equivalent
`LoadBalancing`/`RequiredClaims` field to the generated `define.InboundEvent` literal after migrating - a manual
step, called out here because a silently dropped `RequiredClaims` is a security regression.

### Inbound event Source

The manifest's `inboundEvents.<name>.package` is the source **microservice** package path, not its api package.
The referenced `OutboundEvent` var lives in that microservice's api package, which by convention is
`<service>/<service>api`. So `Source: <serviceapi>.<Name>` and the api package is imported. Using the service
path directly (the original bug) both fails to resolve the var and pulls the source service into the sink,
creating an import cycle.

### Config value types

A config's value type comes from its getter signature in the manifest, whose return is named for the config,
not `value` (`TimeBudget() (budget time.Duration)` -> `time.Duration`). It becomes an explicit type carrier
(`time.Duration(0)`, `string("")`, `int(0)`, or `T{}` for a struct). A `time.Duration` config adds the `time`
import; this is `addType`'s stdlib-fallback path, distinct from the alias-only `scan` used for lifted type text
(a lifted struct always arrives with its package already imported by endpoints.go, but a config value type
comes from the getter and may reference a package endpoints.go did not import).

## Idempotency

A microservice already migrated (definition.go present, endpoints.go gone) is a clean no-op. `findAPIDir`
therefore recognizes an api subdirectory by either endpoints.go or definition.go, so a re-run finds the package
and returns rather than erroring.

## What it does not do

- Call genservice, or regenerate client.go/intermediate.go/mock.go/manifest.yaml. The upgrade skill runs
  genservice as step 2.
- Touch sibling type files, service.go, OnStartup/OnShutdown, or anything outside the api package's
  endpoints.go.
- Loop over nested directories. It migrates the one directory given; the skill (or the operator) iterates.

## Validation

The 1.41.0 routine was validated by running `genupgrade` then `genservice` in place on representative services -
calculator (functions, metrics, a sibling domain type), creditflow (tasks, workflows, a web), the
eventsource/eventsink pair (outbound, and inbound with the source-before-sink ordering), and httpingress
(config-only) - and asserting `go vet` and the services' own tests pass, then reverting via git. That sweep is
what surfaced the inbound-Source, config-value-type, duration-config-import, and method-by-kind details above,
and a genservice bug (unconditional `sub`/`http`/`httpx` imports for endpoint-less microservices).

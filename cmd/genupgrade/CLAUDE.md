# cmd/genupgrade

The deterministic arm of the versioned upgrade skills. Each fabric version that requires a mechanical
source-code change registers an upgrade routine here, and the matching `upgrade-vX-Y-Z` skill invokes
`genupgrade -v <version> -path <microservice dir>` for the mechanical part while handling the judgment parts
itself. genupgrade operates on one microservice directory at a time.

## When a version needs a routine here (vs. pure-shell, vs. manual)

A version's mechanical change lands in one of three places, chosen by how much *structure* the transform needs.
Not everything belongs here - a genupgrade routine earns its place only when a textual find/replace cannot do
the job safely.

- **Pure textual find/replace -> `sed`/`perl` in the skill, no routine here.** A symbol, config key, import
  path, or struct field renamed with no structural change (`Subflow` -> `Subgraph` at 1.42.0; a provider's
  `CompletionURL` config -> `MessagesURL`; `Usage.ThinkingTokens` -> `ReasoningTokens`). A genupgrade routine
  for a rename would just be `sed` reimplemented in Go - more code, identical result, no extra safety (it still
  cannot be verified mid-chain). Keep it in the skill's pure-shell edits.
- **Parse-and-synthesize -> a routine here.** The transform must read Go *structure* to produce its output:
  parse an AST, read one file to rewrite another, lift declarations by source offset, or emit new declarations.
  `sed` cannot do this reliably. The 1.41.0 `endpoints.go` -> `definition.go` synthesis (manifest +
  endpoints.go + intermediate.go in, a new definition.go out, type decls lifted verbatim by AST offset) is the
  canonical case. This is the only thing genupgrade exists for.
- **Semantic rewrite that needs judgment -> grep-guided manual steps in the skill, neither sed nor a routine.**
  The call's *meaning* changed and the correct new shape depends on intent - a `[]Message` conversation
  becoming an ordered `[]Item`, or choosing a subgraph vs. a graph node when a task stops being independently
  callable (1.42.0). genupgrade must stay deterministic, so it does not guess these; the skill greps for the old
  shape, the author rewrites each site, and the orchestrator's final `go vet` surfaces any miss as a loud
  compile error.

Rule of thumb: **sed renames, genupgrade parses, humans judge.** One version may need all three at once.

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

**The general rule: a numbered `upgrade-vX-Y-Z` skill may perform only work that is independent of the fabric
version it runs against, and may never assume the tree builds when it finishes.** Both constraints come from one
fact: a frozen `upgrade-vX-Y-Z` skill does **not** run against version X. `upgrade-microbus` downloads the latest
`.claude` and `go get`s the latest fabric, then runs *every* numbered skill in the chain against that **target**
version. So a skill authored for 1.35.0 executes against, say, 1.44.0's fabric, alongside every other skill in the
chain - never against the 1.35.0 world it describes.

Two consequences:

- **Tooling must be version-independent.** Every `go run github.com/microbus-io/fabric/cmd/<tool>` a skill names
  resolves to the *target* version's binary, so the tool must exist and behave identically there. genupgrade is the
  one fabric tool with that guarantee - append-only and frozen, its routines never pruned - so the latest binary
  always carries the routine a given `-v X.Y.Z` names. Pure-shell (`sed`/`perl`/`grep`) qualifies too, depending on
  no fabric tool at all. Boilerplate generators do **not** qualify: `genmanifest`/`genmock` were superseded by
  `genservice` at 1.41.0, so a skill that ran `genmock` detonates the moment its chain targets a version where
  `genmock` is gone.
- **No step may assume a buildable tree.** Mid-chain the tree is half-migrated against the *final* framework, so a
  skill that ran `go vet`/`go test` (or any tool that parses the whole project) would fail on code a later skill has
  not fixed yet. Only `upgrade-microbus`'s single final pass - after the whole chain has applied its edits and
  `genservice` has regenerated - sees a tree that is expected to compile.

The invariant, therefore: **a numbered skill invokes only `genupgrade` plus pure-shell `sed`/`perl`/`grep` edits (and
its own reasoning), never a boilerplate generator and never a verify step.** Regeneration with the current generator
and the single `go mod tidy && go vet ./... && go test ./...` are owned by `upgrade-microbus`'s final step. When
cutting a version that needs a mechanical transform, decide its home by the boundary above ("When a version needs a
routine here"): a parse-and-synthesize transform ships as a genupgrade routine, a rename stays as `sed` in the skill,
and a judgment rewrite is a grep-guided manual step - none of them a generator call or verify baked into the frozen
skill.

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

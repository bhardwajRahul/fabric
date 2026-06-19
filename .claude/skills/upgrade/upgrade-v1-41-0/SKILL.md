---
name: upgrade-v1-41-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.40.x to v1.41.0. Structural change to how every microservice is authored: the api package's hand-written endpoints.go is replaced by a typed, declarative definition.go (a define.* var per feature, plus the In/Out structs, plus the Hostname/Name/Version/Description consts), and client.go, intermediate.go, mock.go, mock_test.go, and manifest.yaml all become generated artifacts derived from definition.go by the new cmd/genservice. The migration is mechanical and per-microservice: cmd/genupgrade -v 1.41.0 synthesizes each definition.go from the existing manifest.yaml (the feature spec) + endpoints.go (the In/Out and domain type declarations, lifted verbatim) + intermediate.go (the Version counter and any workflow subscription options), then deletes endpoints.go; cmd/genservice then regenerates the five derived files. No service.go handler bodies, OnStartup/OnShutdown, sibling domain-type files, config.yaml, env.yaml, or main/main.go are touched. The genmanifest and genmock tools are superseded by genservice (the refreshed housekeeping skill already runs genservice). Nothing changes at runtime - this is a source-layout and codegen change, and the generated code is behavior-for-behavior the same.
---

## What changed

v1.41.0 flips the authoring model for every microservice. Until now the agent hand-wrote five files that had to
stay in sync (`*api/endpoints.go`, `*api/client.go`, `intermediate.go`, `mock.go`, and `manifest.yaml`). Now it
writes one typed spec - `*api/definition.go` - and a deterministic generator produces the rest.

- **`*api/endpoints.go` is replaced by `*api/definition.go`.** Instead of a `Def{Method, Route}` literal per
  endpoint plus In/Out structs (with the feature's description and options living in `intermediate.go`'s
  `svc.Subscribe(...)` call), each feature is a single typed `define.*` var that carries everything:
  `var VerifyCredit = define.Task{Host: Hostname, Method: "POST", Route: ":428/verify-credit", RequiredClaims: ...,
  In: VerifyCreditIn{}, Out: VerifyCreditOut{}}`, with its description as the var's godoc. The In/Out and domain
  type declarations move into `definition.go` unchanged. `definition.go` also declares the service-level consts
  `Hostname`, `Name`, `Version`, and `Description`.
- **`client.go`, `intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml` become generated artifacts.**
  They are projections of `definition.go` produced by the new `cmd/genservice` and carry a
  `Code generated ... DO NOT EDIT` header. They are no longer hand-edited; you change `definition.go` and
  regenerate.
- **`cmd/genservice` supersedes `cmd/genmanifest` and `cmd/genmock`.** One generator now produces all five derived
  files from `definition.go`. The refreshed `housekeeping` skill (downloaded by `upgrade-microbus`) runs
  `genservice` instead of the old pair.
- **`cmd/genupgrade` performs this migration.** It synthesizes each `definition.go` and deletes the matching
  `endpoints.go`, then leaves the rest to `genservice`.

This is a source-layout and codegen change only. The generated wiring is behavior-for-behavior identical to what
you had, so there are no runtime changes and no `config.yaml`/`env.yaml`/`main/main.go` edits.

## Workflow

```
Upgrade a Microbus project to v1.41.0:
- [ ] Step 1: Locate every microservice
- [ ] Step 2: Synthesize definition.go for each (genupgrade)
- [ ] Step 3: Regenerate the boilerplate for each (genservice)
- [ ] Step 4: Tidy and verify
```

#### Step 1: Locate Every Microservice

A microservice is a directory whose `*api` subdirectory still has an `endpoints.go`. Find them:

```bash
find . -path ./vendor -prune -o -name endpoints.go -path '*api/endpoints.go' -print
```

Each match's microservice directory is the parent of the `*api` directory (e.g. `endpoints.go` at
`./creditflow/creditflowapi/endpoints.go` means the microservice directory is `./creditflow`). A project that
returns no matches is already migrated - nothing to do.

The two passes below are kept separate on purpose: every `definition.go` must exist (Step 2) before any
`genservice` runs (Step 3), because a microservice that sinks another's event resolves the source's
`definition.go` during generation. Between the passes the project does not compile; that is expected and Step 3
restores it.

#### Step 2: Synthesize `definition.go` for Each Microservice

For each microservice directory, run `cmd/genupgrade`. It reads the microservice's `manifest.yaml`,
`*api/endpoints.go`, and `intermediate.go`, writes `*api/definition.go`, and deletes `*api/endpoints.go`. It is
idempotent (an already-migrated microservice is a no-op) and never calls another generator.

```bash
find . -path ./vendor -prune -o -name endpoints.go -path '*api/endpoints.go' -print \
  | while read -r ep; do
      svcdir=$(dirname "$(dirname "$ep")")
      go run github.com/microbus-io/fabric/cmd/genupgrade -v 1.41.0 -path "$svcdir"
    done
```

`genupgrade` lifts the In/Out and domain `type` declarations out of `endpoints.go` verbatim (preserving json/
jsonschema tags, comments, and `MARKER`s), pulls each feature's route/method/claims/description from the manifest,
preserves the `Version` counter from `intermediate.go`, and adds the new `Name`/`Version`/`Description` consts. It
does not recover inbound-event `WithOptions` (queue/claims), which the manifest never recorded; if any inbound
hook in this project set `sub.NoQueue()`/`sub.RequiredClaims(...)` via `NewHook(svc).WithOptions(...)`, re-add the
equivalent `LoadBalancing`/`RequiredClaims` field to that `define.InboundEvent` literal by hand after this step.

#### Step 3: Regenerate the Boilerplate for Each Microservice

With every `definition.go` now present, run `cmd/genservice` on each microservice directory to (re)generate
`*api/client.go`, `intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml`:

```bash
find . -path ./vendor -prune -o -name definition.go -path '*api/definition.go' -print \
  | while read -r def; do
      svcdir=$(dirname "$(dirname "$def")")
      go run github.com/microbus-io/fabric/cmd/genservice "$svcdir"
    done
```

#### Step 4: Tidy and Verify

```bash
go mod tidy
go vet ./...
go test ./...
```

`go mod tidy` pulls the dependencies v1.41.0 requires. A clean `go vet ./...` confirms every microservice now
compiles from its generated boilerplate; the tests confirm unchanged runtime behavior. Review the diff: each
microservice should have gained a `definition.go`, lost its `endpoints.go`, and have regenerated derived files
whose content matches what you had (modulo the `DO NOT EDIT` headers and the dropped `frameworkVersion` manifest
field).

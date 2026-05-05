---
name: upgrade-v1-29-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.28.x to v1.29.0. Flags reserved `id-`/`loc-` hostnames, removes redundant `iss=~"access.token.core"` predicates, drops the `microbus:"1"` bearer-token escape hatch, and regenerates manifests + topology with the new `cmd/genmanifest` and `cmd/gentopology` tools.
---

## Background

v1.29 changes the NATS subject wire format and ships per-microservice ACL
generation. Mixed-version bundles cannot interoperate over the bus, so the whole
deployment must roll forward together. Three new tools land:

- `cmd/genmanifest` regenerates `manifest.yaml` from source. Run during
  housekeeping. Replaces hand-edited manifests.
- `cmd/gentopology` regenerates `main/topology.mmd` from the bundle. Run during
  housekeeping. Replaces the old `chart-topology` skill.
- `cmd/gencreds` signs per-microservice `<hostname>_nats.creds`. Runs at deploy
  time, not housekeeping; this skill does not invoke it.

The connector now reserves `id-` and `loc-` as leading hostname segments (used
by the new wire format for instance addressing and locality slots) and pins
JWKS lookup to the framework's token services, making `iss=~"access.token.core"`
predicates redundant. Bearer-token ingress no longer honors the legacy
`microbus:"1"` claim escape hatch.

## Workflow

```
Upgrade a Microbus project to v1.29.0:
- [ ] Step 1: Flag reserved id- / loc- hostnames
- [ ] Step 2: Drop redundant iss=~ predicates from intermediate.go
- [ ] Step 3: Drop the microbus:"1" bearer-token escape hatch
- [ ] Step 4: Regenerate manifests with cmd/genmanifest
- [ ] Step 5: Regenerate topology with cmd/gentopology
- [ ] Step 6: Heads-up audits (env-var NATS auth, :666 endpoints)
```

#### Step 1: Flag Reserved `id-` / `loc-` Hostnames

Grep every `manifest.yaml` for `hostname:` values. Flag any value whose first
dot-segment starts with `id-` or `loc-` (e.g. `id-foo.example`,
`loc-us-west.example`). A microservice with such a hostname will fail
`Startup` with `invalid hostname`.

Do not auto-rename - hostnames are part of every microservice's public API.
List the offenders for the developer; renames cascade to the
`*api/endpoints.go` `Hostname` constant, every importing call site, and every
`config.yaml` block.

#### Step 2: Drop Redundant `iss=~` Predicates

The connector pins JWKS lookup to fixed issuer hostnames, so an
`iss=~"access.token.core"` (or `iss=~"bearer.token.core"`) clause inside
`requiredClaims` is now redundant.

Source of truth is `intermediate.go`'s `sub.RequiredClaims("...")` calls, not
the manifests. Grep `intermediate.go` files for that pattern and rewrite each
predicate by removing the redundant clause (and any leading/trailing `&&`).
If `iss=~"..."` is the entire predicate, remove the `sub.RequiredClaims(...)`
call altogether.

Step 4 will rewrite manifests from the updated source.

#### Step 3: Drop the `microbus:"1"` Bearer-Token Escape Hatch

In v1.28 the HTTP ingress accepted bearer tokens carrying the `microbus:"1"`
claim as if they were already framework-issued. In v1.29 every inbound bearer
token must be issued by `bearer.token.core`.

Grep the project for the literal `"microbus": "1"` (and `"microbus":"1"`).
Hits typically appear in:

- Test code minting bearer tokens directly via `jwt.MapClaims{...}` to bypass
  the bearer-token round-trip. Rewrite to mint through
  `bearertokenapi.NewClient(svc).Mint(...)`.
- Custom `Authorization: microbus://...` header construction (rare).

Tests using `pub.Actor(jwt.MapClaims{...})` are unaffected: actor JWTs flow
through the access-token verifier, not the bearer-token verifier.

List file:line for each hit and let the developer decide on the rewrite.

#### Step 4: Regenerate Manifests With `cmd/genmanifest`

For every microservice in the project (every directory containing a
`manifest.yaml`), run from inside the directory:

```bash
go run github.com/microbus-io/fabric/cmd/genmanifest --path .
```

This rewrites the manifest from the source: identity, configs, metrics,
tickers, endpoints (`webs`, `functions`, `tasks`, `workflows`,
`outboundEvents`), inbound event subscriptions. It bumps `frameworkVersion`
and updates `modifiedAt`. The dropped fields from the v1.28 schema
(`general.db`, `general.cloud`, top-level `downstream:`,
`inboundEvents.*.source`) are not re-emitted.

Common errors:
- *Missing `Hostname` constant in `*api/endpoints.go`*: every microservice's
  `*api` package must declare `const Hostname = "..."`.
- *Non-literal `sub.At(method, route)`*: genmanifest only resolves literal
  arguments. Inline the literal.

#### Step 5: Regenerate Topology With `cmd/gentopology`

From the project root:

```bash
go run github.com/microbus-io/fabric/cmd/gentopology --bundle main/main.go
```

Rewrites `main/topology.mmd`. Trust-root microservices (any endpoint on
`:666`) render in orange with `|danger|`-labeled inbound edges. Service deps,
event hooks, SQL imports, and HTTP-egress targets are derived from source.

#### Step 6: Heads-Up Audits

Two non-blocking changes worth flagging.

**a. Env-var NATS auth is deprecated.** `MICROBUS_NATS_USER`,
`MICROBUS_NATS_PASSWORD`, `MICROBUS_NATS_TOKEN`, `MICROBUS_NATS_USER_JWT`,
`MICROBUS_NATS_NKEY_SEED` still work but log a one-line deprecation. Env vars
can't carry per-microservice identity. Grep `env.yaml`, `env.local.yaml`, and
deployment manifests for these keys; recommend switching to per-microservice
`<hostname>_nats.creds` (signed by `cmd/gencreds` at deploy time) or a shared
`nats.creds`. Do not auto-edit.

**b. `:666` is now a formal trust-root tier.** HTTP ingress unconditionally
blocks `:666`, the NATS subject layout reserves a `danger` segment for it,
and per-microservice ACLs grant `:666` only to callers whose source actually
invokes a `:666` endpoint. Grep `manifest.yaml` for `route:` values
containing `:666`; for each one, ask whether the endpoint truly is a trust
root (mints credentials, executes shell commands, performs privileged writes
or role grants). If not, recommend moving it to `:443`, `:888`, or another
internal port. Do not auto-edit.

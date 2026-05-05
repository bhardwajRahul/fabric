# cmd/gencreds

Reads a microservice bundle's source code, computes per-service NATS
ACL rules from the AST, and signs them into `<hostname>_nats.creds`
files using an operator's account NKey. Runs at deploy time, not
housekeeping.

## Why this tool exists

The framework's recommended production NATS auth model is **operator-
mode**: a chained operator → account → user JWT setup, where each
service in a bundle holds its own user JWT pinning its own permissions.
This is the model `nsc` produces in real deployments and the only one
where the broker gets to enforce per-service capability isolation.

`gencreds` is the bridge between the framework's source code and that
auth model. It walks each service's source, derives the set of NATS
subjects the service needs to publish to and subscribe from, and signs
one user JWT per service in the bundle. The resulting permissions are
**true to code** - they grant exactly what the service's code does, not
what its typed clients could do, not what the operator wrote down at
some point in the past.

The operator's account NKey is the trust root the tool depends on. The
operator is a private input to deployment (typically a single seed file
owned by ops); the tool reads it at deploy time and never writes it.

Simple-user mode (server-side `authorization { users = [...] }`) and
shared-creds mode (one `nats.creds` for the whole bundle) are
documented fallbacks for environments that can't run an operator-mode
resolver, but both lose broker-pinned per-service identity. A
compromised service in those modes can publish under any of the
bundle's source segments. The framework neither requires nor blocks
them - it ships the recommended path and lets operators choose.

## When it runs

`gencreds` runs in the CD pipeline, **not** during housekeeping. It
consumes operator-sensitive material (the account NKey) and produces
deployment artifacts (`.creds` files containing private user NKey
seeds) that are never committed to the repo.

The framework deliberately commits no intermediate ACL artifact between
source and `.creds`. The rule set is computed from source at deploy
time and lives only inside the signed JWT it produces. This is what
keeps caller manifests stable across callee Def renames - there's
nothing for a renamed route to stale.

## Pipeline position

```
source code  --(scan + buildACLRules)-->  rules  --(sign)-->  <hostname>_nats.creds
                                                                      |
                                                                      v
                                                          transport.Open(ctx, hostname, ...)
                                                          resolves via per-hostname creds lookup
```

The whole pipeline lives in one tool. Manifest is read only for the
service's hostname and downstream package list; route-level data is
derived from source on every run.

`{{plane}}` template tokens emitted by the rule builder resolve at
sign time - so the same bundle source can be signed for multiple
deployments (prod, staging, etc.) by re-running with different
`--plane`.

## Bundle resolution

Two paths into the same downstream pipeline:

1. **`--bundle main.go`** - parses the file's AST for `app.Add(...)`
   calls, walks each call's argument expression to find the rightmost
   `pkg.NewService()`-shaped form (handles `.Init(...)` and other
   chained wrappers), maps the alias back to the import path via the
   file's import block, then runs `go list -f '{{.Dir}}'` from
   main.go's directory to get each package's on-disk location. Each
   service directory is the input to the source-driven scan.
2. **`--manifests dir,dir,...`** - direct list, skips AST walking.
   Used by tests and for non-standard bundle compositions.

Duplicates (e.g. `messaging.NewService()` added three times for
replicas) are deduped by hostname - the broker doesn't care how many
replicas share a user identity.

Arguments whose leading selector chain doesn't root in a known import
alias (variadic spreads `app.Add(svcs...)`, range-loop adds, factory
expressions `app.Add(makeService())`, var-bound services) are reported
on stderr with file:line and skipped. The bundle still produces creds
for the recognizable services, but a microservice that only enters the
app via an unrecognized shape silently gets no creds and fails CONNECT
at runtime. The stderr warning is the operator's signal to refactor the
add-call into a shape the resolver can match, or to switch to
`--manifests` for that specific service.

## Source-driven AST scan

`scanService(serviceDir, fromDir)` walks the service's source and
produces an `aclInput` ready for rule construction. The detection
mirrors what `cmd/genmanifest` does for its lighter purpose, but
captures the per-call detail genmanifest deliberately discards:

- **Own routes** - every `svc.Subscribe(...)` call in `intermediate.go`,
  resolved to (Method, Route) via the service's own `*api/endpoints.go`.
- **Outbound events** - Defs in `*api/endpoints.go` that appear as
  `MulticastTrigger.OnX` methods in `*api/client.go`. Builder helpers
  (`ForHost`, `WithOptions`, anything else that returns
  `MulticastTrigger` for chaining) are filtered out by return-type
  shape, not by a hand-maintained name allowlist - this keeps a future
  helper added to `MulticastTrigger` from silently becoming a phantom
  outbound event. The candidate set is then intersected with the Defs
  in `endpoints.go`, so any false positive without a matching Def is
  harmless.
- **Inbound events** - `pkgapi.NewHook(svc).OnX(...)` chains in
  `intermediate.go`. Each hook's source `*api` package is loaded via
  `go list` to resolve the event's hostname + (Method, Route).
- **Typed-client method calls** - `pkgapi.NewClient(svc).Method(...)`
  and friends, including chained, variable-bound, parameter-typed,
  and `clientext.go` helper-expansion forms. `ForHost("literal")`
  becomes a per-call hostname override; `ForHost(varExpr)` becomes
  the wildcard `*`.
- **Raw `svc.Request` / `svc.Publish`** - inline `pub.X(...)` options,
  `[]pub.Option{...}` slice composites, and `append(opts, ...)` builds.
  Non-literal URLs collapse to wildcard hostname/port/path.

Cross-package resolution: each foreign `*api` package is loaded via
`go list -json -e <pkgPath>` and its `endpoints.go` parsed for the
`Hostname` const + Def literals. The package source comes from
`go.mod`'s locked module cache - same source the binary was built
against - so the rule set is correct by construction.

## Rule construction

Once `scanService` produces an `aclInput`, `buildACLRules` emits the
NATS subject patterns:

- Six framework-implicit standard rules per service (control plane,
  reply path, broadcast). The `:666` SUB rule is added only if the
  service exposes a `:666` endpoint of its own.
- Per-route PUB rules from each downstream call.
- Per-route self-PUB rules from outbound events.
- Per-route SUB rules from inbound event hooks.
- Alt-host SUB rules for own routes registered on a different
  hostname (e.g. `//openapi.json:0`, `//root`).

`subsumptionDedup` drops any rule whose subject is strictly subsumed by
another rule in the same set (same verb). Lossless: NATS treats a
matching ACL line as authorization, so removing a rule already covered
by a broader sibling changes nothing on the wire.

## JWT signing

For each service: generate user NKey (or load from
`--persist-user-nkeys` dir), build `jwt.UserClaims` with
`Permissions.Pub.Allow` and `Permissions.Sub.Allow` from the rule set
(with `{{plane}}` substituted), sign with the account KeyPair, format
via `jwt.FormatUserConfig` to produce the canonical `.creds` shape.

The signing key must be an **account** NKey (public key starts with
`A`). Operator and user keys are rejected at load time. This isn't
pedantry - signing user JWTs with the operator key would bypass the
resolver chain at the broker, and signing with another user key
wouldn't work at all.

The account NKey is the operator's private key material in production
(a single seed file owned by ops). The tool reads it from
`--signing-key` and never writes it.

## NKey rotation

`--rotate-user-nkeys` (default) generates fresh user NKeys on every
run. Right default: there's no reason a service's broker identity
should persist across deploys, and rotation limits blast radius of an
exfiltrated `.creds`.

`--persist-user-nkeys <dir>` keeps stable NKeys per service, written
to `<dir>/<hostname>_user.nk`. For environments that allowlist user
public keys at the broker (e.g. server-side `pub_key` checks beyond
the JWT chain).

## JWT expiration

`--expiration <duration>` sets the `exp` claim on every emitted user
JWT. Default `0` = no expiration. Without an `exp`, the broker
continues to accept any leaked `.creds` indefinitely (until its public
key lands on the account's revocation list); with an `exp`, the broker
rejects the JWT at CONNECT once the time has passed. Operators who
want time-bounded creds should pick a duration comfortably longer than
their deploy cadence so rolling restarts don't race the expiry.

## Size budget

Two checks layered:

1. **Raw permission-JSON budget** (`aclBudgetSize` ≤ `aclBytesBudget`,
   3 KB). Computed before signing. Models the
   `nats.Permissions` JSON shape that nats-server parses on connect.
   The 3 KB target leaves ~1 KB for the JWT envelope (header,
   signature, standard claims) inside the 4 KB `max_control_line`
   default.
2. **Signed JWT byte budget** (`jwtBytesBudget`, 4000). Belt-and-
   suspenders post-encoding check. Any JWT seen here that breaches
   this threshold is anomalous (rule construction kept things well
   under the raw budget). The `budgetErr` type is matched by `main`
   for exit code 2.

## End-to-end test

`e2e_test.go` is the load-bearing validation that the whole chain works
against a real broker. The test takes ~1.3s and is the single fastest
signal of "is per-service identity actually wired up?"

It complements three other testing layers in this package:

- **Unit tests** (`gencreds_test.go`) - JWT roundtrip, bundle
  resolution, plane substitution, persist-NKey stability.
- **Fixture goldens** (`parity_test.go` § `TestScan_FixtureGoldens`) -
  for each of kitchen + weird, runs `scanService` + `buildACLRules` and
  asserts the output matches the committed `nats.acl` golden in the
  fixture directory. Catches regressions in either the AST detection
  or the rule construction. The test accepts a `-update` flag
  (`go test ./cmd/gencreds/ -update -run TestScan_FixtureGoldens`)
  that rewrites the goldens from the live scan output, for use when
  a scanner change is intentional. Review the diff in `git status`
  and commit. Without `-update`, drift fails the test.
- **Smoke test across real services** (`parity_test.go` §
  `TestScan_AllRealServices`) - every `coreservices/*` and `examples/*`
  service must scan successfully, produce a non-empty rule set, and
  fit the budget.

This e2e test is the only one that exercises the *broker* side: JWT
chain acceptance and subject-level permission enforcement.

### What it tests

Three sub-assertions, each pinned to a distinct security claim:

1. **Services connect via per-service creds.** Both kitchen + weird connect
   to the embedded broker using their gencreds-signed user JWTs. If the JWT
   chain is broken (operator → account → user signature), `app.Startup`
   fails. Indirect proof, but a hard one - operator-mode brokers reject
   malformed chains at CONNECT before any subject-level work.
2. **Traffic flows under ACL.** `kitchen.MyFunc("input")` exercises every
   detection pattern (chained, var-bound, parameter-typed, ForHost
   literal/varExpr, multicast, helper-expansion, self-call, raw
   `svc.Request` inline/slice/append, trust-root, outbound trigger) through
   real NATS. No ACL violations means the AST-derived PUB rules cover all
   the runtime patterns the kitchen fixture exercises. The fixture-golden
   test pins the rule set byte-for-byte against the committed `nats.acl`;
   this assertion adds "and the broker actually accepts those patterns
   when presented in a real CONNECT."
3. **Broker rejects subjects outside the allow-list.** A raw nats.go client
   built with kitchen's `.creds` tries to subscribe on
   `<plane>.safe.443.*.stranger.>` (foreign dest, no matching SUB rule).
   The broker raises permissions violation. This is the only sub-assertion
   that proves the broker is *actively* enforcing the JWT permissions, not
   just accepting the connection. Without it, sub-assertion 2 could pass
   under a misconfigured broker that ignored permissions entirely.

### Setup shape (operator-mode NATS)

The test boots an in-memory `nats-server/v2` configured for operator-mode
auth - the same shape `nsc` produces for production deployments:

```
operator NKey ──signs──▶ account JWT (user account, "MICROBUS")
                      └─▶ system account JWT ("SYS")  ← required by nats-server in op-mode
account NKey  ──signs──▶ user JWT for kitchen.fixture (gencreds output)
                      └─▶ user JWT for weird.fixture
```

Server options:

- `TrustedOperators: []*jwt.OperatorClaims{...}` - pins which operator
  the server trusts.
- `SystemAccount: sysPub` - required when operator mode is enabled.
- `AccountResolver: &MemAccResolver{}` - pre-populated with both account
  JWTs via `Store(pubkey, jwt)`. Memory resolver avoids on-disk artifacts
  for account JWTs; fine for tests, real deployments use URL or NATS
  resolver.

All NKeys are generated **fresh per test run**. The signing is fast and
the resulting `.creds` are scoped to the embedded broker that lives only
for the test process. No committed test signing key (which would be a
secret-shaped artifact in the repo).

### Why nested-tmp chdir, not global `/tmp`

Project convention: e2e test artifacts (the `.creds` files) live under
the test's own CWD, not in the global temp directory. The test creates
a nested directory via `os.MkdirTemp(origDir, "e2e-")` (i.e.
`cmd/gencreds/e2e-XXXXXX/`), chdirs into it, and registers cleanup to
restore CWD and remove the dir.

Why: `transport.Open(ctx, hostname, ...)` resolves
`<hostname>_nats.creds` from CWD. The connector is the framework's, not
the test's, so we can't pass a directory through. The chdir is the
mechanism that puts the `.creds` files where the framework looks.

Side benefit: if the test panics before cleanup, the leftover dir is
co-located with the test code, easy to spot in `git status` and clean up
manually. With `/tmp` it'd silently leak.

### Why all three assertions

Earlier draft considered just (1) and (2). Without (3), a misconfigured
broker that accepts every authenticated user (operator-mode set up but
permissions ignored, e.g. resolver returning no permissions) would pass
the test silently. (3) is the only assertion that proves the *permission
enforcement* is live, which is the entire point of per-service `.creds`.

### Why nats.go reports permissions errors asynchronously

The test reads permissions violations off the connection's
`SetErrorHandler` channel, not from the return of `SubscribeSync`. NATS
client design: the client adds the subscription to its local map and
sends `SUB <subject>` to the server, but doesn't synchronously round-trip
for an ack. The server then responds with `-ERR Permissions Violation`
which surfaces on the async error handler.

A `Flush()` after the subscribe forces a sync round-trip; if the
violation arrives during flush, some nats.go versions surface it as
the Flush error. The test handles both paths (`isPermErr(err)` after
flush, fallback to async handler). Either is acceptable proof of
enforcement.

### What gets logged

The connector logs (`Startup`, `Connected to NATS`, `Transport latency`,
`Disconnected from NATS`) survive into stderr - useful for debugging when
the test fails. They're noisy on success; consider piping to `/dev/null`
in CI if log-verbosity becomes an issue.

### Plane handling

The test sets `MICROBUS_PLANE=e2eplane` explicitly. This pins the plane
substituted into `{{plane}}` by gencreds *and* the plane the connector
uses at subscribe/publish time. They must match - drift would mean
generated rules cover one plane but the runtime publishes to another,
silent ACL miss.

Critically, the test does **not** use `application.RunInTest` - that
helper overrides the plane to a random per-test value, which would
break the plane-pinning above. The test calls `app.Startup` directly,
managing the plane via env var.

### Deployment mode

The test sets `MICROBUS_DEPLOYMENT=LAB` rather than `TESTING`. Reason:
TESTING bypasses the configurator service (none in the bundle, fine)
*and* accepts `alg=none` JWTs - but more importantly, TESTING is the
short-circuit-only happy path the framework's other tests rely on, and
this one is precisely about routing through NATS. LAB is the
production-shaped deployment that doesn't bypass anything but also doesn't
require a configurator. (PROD would also work; LAB is faster and skips
some hardening that doesn't matter for a test.)

### What `kitchen.MyFunc` actually does on the wire

`MyFunc` calls `weirdapi.NewClient(svc).Plain(ctx)` and friends. With
`MICROBUS_SHORT_CIRCUIT=0`, every call hits NATS - even though kitchen
and weird are in the same Go process. The framework's connector
publishes on `<plane>.safe.<port>.<src>.<dest>.<id>.<method>.<path>` and
the broker routes to the matching subscriber.

Patterns whose downstream isn't in the bundle (`other.host`,
`alt.host`, `dispatched`, `appended`) ack-timeout to 404 because no
service answers. The kitchen fixture intentionally swallows those errors
with `_, _ =` so the function returns success. **What matters here is
that the publish succeeded** - an ACL block would surface as a
synchronous publish error, not a 404 from the responder. The 404 path
is correct behavior; the test relies on it.

### Adding a new pattern

If a future fixture grows a new call pattern (e.g. some new typed-
client form), you don't need to update e2e_test.go directly - the
existing `kitchen.MyFunc` driver runs whatever's in kitchen's body.
Just:

1. Add the new pattern to kitchen's `service.go`.
2. Re-run `genmanifest` on kitchen (housekeeping).
3. Regenerate the fixture's `nats.acl` golden:
   `go test ./cmd/gencreds/ -update -run TestScan_FixtureGoldens`.
   Review the diff and commit if intentional.
4. The e2e test now exercises the new pattern automatically.

The test does not enumerate patterns; it trusts kitchen's body to
cover them. Pattern-by-pattern assertions live in
`cmd/genmanifest`'s manifest goldens (manifest shape) and in
`TestScan_FixtureGoldens` (subject byte-equivalence).

### Adding a new ACL-rejection case

Sub-assertion 3 currently probes one rejection (foreign dest on SUB).
If you need to add more (e.g. a PUB rejection on a forbidden method),
add a sub-test inside the same `t.Run("ACL rejects ...", ...)` block -
same NATS connection, just additional `Subscribe`/`Publish` calls and
matching `errs` reads. Don't proliferate top-level tests; the
broker-bootstrap cost (~50ms) doesn't need to be paid per-rejection.

### Test deps

Two NATS-side deps land with this package:

- `github.com/nats-io/jwt/v2` - used by gencreds proper for `UserClaims`,
  `AccountClaims`, `OperatorClaims`, and `FormatUserConfig`.
- `github.com/nats-io/nats-server/v2` - used only by the e2e test, but
  the dep is unconditional (no build tag). Roughly 5 small transitive
  deps (highwayhash, go-tpm, x/time, x/sys, antithesis-sdk). Build-tag
  isolation was considered and rejected: tagged tests bit-rot, the dep
  is small, and the test runs in <2s.

`nkeys` was already transitive (used by the connector for instance IDs).

## Things that would surprise future-me

- **`go list` requires CWD = main.go's directory** for `--bundle` mode.
  The tool sets it via `cmd.Dir = workDir` so callers don't have to.
- **`go list` failures abort with a real error.** With the `-e` flag set,
  `go list` itself succeeds for missing packages (returned `Dir` is
  empty), so an error from `goListDir` represents a genuine toolchain
  failure rather than a missing dependency. Earlier versions silently
  swallowed these, which masked broken-module errors as missing PUB
  rules — the broker would then reject the publish at runtime, far from
  the actual cause.
- **`jwt.FormatUserConfig` returns the canonical decorated form** with
  `BEGIN/END NATS USER JWT` and `BEGIN/END USER NKEY SEED` markers.
  Don't try to construct it manually - the format is what nats.go's
  `nats.UserCredentials(path)` parses.
- **`ParseDecoratedNKey` returns a `KeyPair`, not a seed `[]byte`.** Test
  code that wants to round-trip the user public key calls `.PublicKey()`
  on the returned KP directly.
- **The account JWT is signed by the operator NKey, but the user JWT is
  signed by the account NKey.** Two-level chain. Confusing the two
  produces a JWT the broker rejects with "issuer not trusted" or similar.
- **`MemAccResolver.Store(pubkey, jwt)` is the only way to pre-populate
  in-memory accounts.** Don't try to set `Options.AccountResolverPreload`
  - that's for URL resolvers fetching from an external service.
- **`nats-server`'s `Options{NoLog: true, NoSigs: true}`** - disables the
  server's stderr logging and signal handling. Without `NoLog`, the
  embedded server floods test output. Without `NoSigs`, the test's own
  Ctrl-C goes to the embedded server first.
- **The kitchen fixture's `MyFunc` is what drives the 14 patterns.** If
  someone reorders or removes patterns there, the e2e test silently
  loses coverage. The genmanifest manifest goldens would catch the
  *manifest* shape changing, but not the runtime exercise.

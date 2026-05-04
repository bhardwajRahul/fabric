## Design Rationale

### One feature option per subscription, enforced at option-application time

Exactly one of `Function`, `Web`, `InboundEvent`, `Task`, `Workflow` must be applied. Each of these options checks `if sub.Type != ""` and returns an error if a type was already set. The check happens *during option application* — not in `NewSubscription`'s post-validation — so the error surfaces at the offending call site (`sub.Function(...)` after `sub.Web()`) rather than as a generic "type already set" later.

`NewSubscription` separately validates that `sub.Type` is non-empty after applying all options, catching the "no feature option supplied" case.

### `specPath` and the host/port/route triple are not the same field

A subscription stores the route the user supplied (`specPath`, e.g. `:888/dcache` or `//all/ping`) *separately* from the parsed-out `Host`, `Port`, and `Route` fields. The reason is `RefreshHostname`: when the connector's hostname changes (e.g., testing scenarios that re-host a connector), the parsed triple can be recomputed from `specPath` against the new default host without re-running the user's options.

This is also why the connector calls `RefreshHostname(c.hostname)` in `activateSub` — subs created before `SetHostname` would otherwise hold a stale `Host`.

### `Subscription.Subs` is the connector piggybacking on this struct

The `Subs` field (slice of `*transport.Subscription`) is populated by the connector during `activateSub` with one transport subscription per locality prefix. The `sub` package itself never reads or writes it. The cleaner design would be for the connector to wrap `Subscription` in its own struct and keep transport bookkeeping there, but `Subscription` is effectively framework-internal and the wrapper was skipped for convenience. Treat the field as a connector concern even though it lives on this struct.

A subscription created via `NewSubscription` always has `Subs == nil`.

### `Inputs` and `Outputs` are reflection seeds, not request-handling state

The `Inputs` / `Outputs` fields hold zero-value struct instances supplied via the feature options (`Function(in, out)`, `InboundEvent(in, out)`, etc.). They are *only* used by the connector's OpenAPI handler, which walks them via reflection to build per-field JSON schemas. Nothing in the request-handling path consults them.

`Web()` doesn't take inputs/outputs because raw web handlers operate on `http.ResponseWriter` / `*http.Request` directly and have no typed schema to reflect.

### `sub.Type` values match `openapi.Feature*` strings by convention, not by import

`TypeFunction = "function"`, `TypeWeb = "web"`, `TypeInboundEvent = "inboundevent"`, `TypeTask = "task"`, `TypeWorkflow = "workflow"` are deliberately equal to the corresponding `openapi.Feature*` constants in another package. `connector/control.go`'s `handleOpenAPI` passes `s.Type` straight through into `openapi.Endpoint.Type` without translation.

The duplication is intentional: `sub` does not import `openapi` (which would be the wrong direction), so the constants are mirrored as plain string literals. If you add a new feature type here, mirror it in `openapi/` and grep for the existing constants to find every site that pattern-matches them.

### `Infra` / `Ultra` and `NoTrace` / `Trace` are reset pairs, not toggle pairs

`Infra()` and `NoTrace()` are the meaningful options. `Ultra()` and `Trace()` reset them — they exist to undo a prior `Infra` or `NoTrace` in a programmatically composed option list. In hand-written code you almost never call `Ultra` or `Trace`; the defaults already match.

`Infra` is currently used by the distributed cache only. See `connector/CLAUDE.md` for what the flag actually does to the activation/deactivation schedule.

### `RequiredClaims` is parsed at Subscribe-time, evaluated per-request

The `RequiredClaims(boolExp)` option calls `boolexp.Eval(boolExp, nil)` at option-application time to validate the expression syntax. A malformed expression therefore fails at `Subscribe`, not at the first request. The actual evaluation against an actor's claims happens in the connector's request handler.

### Method validation accepts only known HTTP verbs (case-insensitive) plus `ANY`

The `knownMethods` map enumerates the methods accepted on a subscription: the nine HTTP methods from RFC 9110 §9 (GET, HEAD, POST, PUT, DELETE, CONNECT, OPTIONS, TRACE, PATCH) plus the framework wildcard `ANY` meaning "match any method." Input is normalized to uppercase before lookup, so callers can write `Method("post")` or `Method("Get")` and the stored `Subscription.Method` is always uppercase.

Unknown tokens (typos like `POSTT`, made-up verbs like `INFO`, or empty strings) fail at option-application time with `405 Method Not Allowed` — `Method(...)` and `At(...)` validate inside the option closure, so the error surfaces at the offending call site rather than later in `NewSubscription`. `NewSubscription` re-checks the final `Method` value as a defensive central guard.

The connector enforces a stricter set on the inbound request path (see `connector/subscribe.go` — its own `validRequestMethods` map). That set excludes `ANY` because `ANY` is a subscription-side match-anything sentinel and should never appear on the wire. The two sets are deliberately separate: the sub package owns the registration-time vocabulary; the connector owns the wire-time vocabulary. Keeping them in lockstep is a manual concern — if you add a new method here, mirror it there.

### Path argument validation is strict and fail-fast

`validatePathArgs` rejects:

- Path arguments that don't span an entire segment (`/x{foo}` or `/{foo}x` are illegal).
- Greedy arguments (`{name...}`) anywhere but the last segment.
- Brace-enclosed names that are non-empty but not lowercase Go identifiers.

These are enforced at `NewSubscription` time, not at request dispatch. A malformed route fails the `Subscribe` call, not a later request.

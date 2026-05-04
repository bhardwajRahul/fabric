## Design Rationale

### Lifecycle phases are an atomic state machine

The connector lives in one of four phases — `shutDown`, `startingUp`, `startedUp`, `shuttingDown` — held in an `atomic.Int32`. Almost every setter (`SetHostname`, `SetPlane`, `SetDeployment`, `SetOnStartup`, `DefineConfig`, ...) refuses to run unless the phase is `shutDown`. This is what makes the connector effectively immutable once started — there is no locking around the configuration fields because the phase guard makes mutation impossible after `Startup`.

`captureInitErr` exists to defer pre-start errors. Callers can chain `Init(...)` and similar setup without checking each error; the *first* init error is stashed and re-raised by `Startup`. After `Startup` runs, `initErr` is cleared so a subsequent restart isn't poisoned.

`Startup` uses `defer phase.CompareAndSwap(startingUp, shutDown)` as a fallback so a startup that errors out before reaching `startedUp` returns the connector to a startable state. The error path also calls `Shutdown` to clean up partially-initialized state (transport, dlru, OTel providers) — the deferred CAS only fires if `Shutdown` didn't already advance the phase.

### Subscription activation order during Startup

Subscriptions activate in deliberate waves, not all at once:

0. **Response sub.** Immediately after the transport connects, the connector subscribes to its own response subject (`<plane>.r.<reverseHost>.<id>`). This must come before everything else because the connector itself makes outbound requests during the rest of Startup — most obviously to `configurator.core` to fetch config values — and those replies have nowhere to land without the response sub already in place.
1. **Infra subs** (those tagged `sub.Infra`) activate *before* the user's `OnStartup` callback runs, so framework facilities are reachable from inside `OnStartup`.
2. **Control subs** (`:888/ping`, `/config-refresh`, `/metrics`, `/trace`, `/on-new-subs`, `/openapi.json`) activate *after* `OnStartup` returns — the control plane is not exposed until the service has finished its own init.
3. **User business subs** activate after the control subs.

Tickers start only after `phase` reaches `startedUp`. The lifetime context (`lifetimeCtx`) is created between `OnStartup` and the control-sub activation, which is why `OnStartup` must use the `ctx` argument rather than `c.Lifetime()`.

### `sub.Infra` expresses a lifecycle dependency, not a category

The `Infra` flag exists because of one concrete requirement: the [distributed cache](../dlru) must be reachable from `OnStartup` *and* from `OnShutdown`. The flag therefore drives a stricter activation/deactivation schedule rather than tagging "framework-internal" subscriptions in general. At present only `dlru` uses it. New uses should be considered carefully — if a subscription doesn't need to be live during user lifecycle callbacks, it doesn't need this flag.

### Shutdown's two-phase drain and the dlru offload window

`Shutdown` reverses Startup with two important asymmetries:

- **Infra subs stay active *through* OnShutdown.** User shutdown code can still hit `DistribCache`. Only after `OnShutdown` returns are infra subs torn down.
- **Infra subs go down *before* `dlru.Close`.** Closing the distributed cache offloads its entries to peers. If this connector's own `:888/dcache` sub were still active, those entries could land back in its own (about-to-be-discarded) cache. Tearing infra subs down first guarantees the offload reaches peers only.

The 8-second pre-cancel drain plus 4-second post-cancel drain is implemented as two `pendingOps` polling loops. `pendingOps` counts in-flight requests, ticker invocations, and goroutines launched via `Go` / `Parallel`. Anything launched with the bare `go` keyword is invisible to this counter and may be killed mid-flight when the lifetime context cancels.

### Plane is a NATS-level isolation prefix

The plane prefixes every NATS subject this connector subscribes to or publishes on, allowing multiple Microbus apps to coexist on the same NATS infrastructure without crosstalk. Three uses:

- **Test parallelism.** When no plane is set and the binary is running under `go test`, the plane is derived from `sha256(testFuncName)[:8]`. Concurrent `go test` runs against the same NATS cluster therefore cannot leak messages into each other.
- **Multi-tenant prod.** Multiple production apps can share NATS infrastructure by each setting their own plane.
- **Default development.** Outside of tests the default plane is `microbus`, which is what local dev typically runs on.

`MICROBUS_PLANE` env var or `SetPlane` overrides the default. The plane is otherwise opaque — it has no parsing rules beyond `[0-9a-zA-Z]*`.

### Time budget is a duration, depth is a counter

Time budget propagates as a duration header (decremented per hop by `networkRoundtrip`), not a deadline timestamp — clocks across replicas are not assumed to be synchronized. A request whose remaining budget falls below one network round-trip errors out as `408 Request Timeout` rather than being dispatched.

Call depth is propagated similarly, incremented by `Publish` on each outbound hop. The default cap of 64 is a cycle detector — at depth 64 `Publish` returns `508 Loop Detected` synchronously without touching the bus.

### Ack-or-fail-fast and the LOCAL escape hatch

`makeRequest` waits up to `ackTimeout` (defaulting to the transport's measured latency) for any responder to ack. If no ack arrives:

- For unicast: synthesize a `404 Not Found` "ack timeout" error.
- For multicast: assume zero responders matched, drop the known-responders cache entry, return cleanly with no responses.

In LOCAL deployment only, if the ack timer fires after `8 * ackTimeout` of wall time the timer is reset once and a debug log is emitted. This is a safety net for long pauses that aren't reflective of a missing responder — most obviously a developer paused in a debugger. The threshold is not exposed; LOCAL is the gating signal.

### Multicast known-responders cache

Multicasts can finish before the full timeout when the connector recognizes "I've seen everyone I expected." `knownResponders` is keyed by NATS subject and stores the set of responder queues seen on the previous successful call. Each subsequent call compares `seenQueues == expectedResponders` and short-circuits when the set matches.

The cache is invalidated when any peer announces new subscriptions on `:888/on-new-subs`. `notifyOnNewSubs` is broadcast on every `Subscribe` call (after Startup), telling other microservices that a new subscriber is in town so they can drop stale cache entries. On a request timeout the local cache entry for that subject is also dropped.

### Locality-aware routing is a NATS subscription trick, not a router

A connector with `locality = "1.b.west.us"` subscribes its handlers to *several* NATS subjects derived from the same route: the bare subject, the ID-prefixed subject, and one prefixed subject per locality suffix (`us.`, `west.us.`, `b.west.us.`, `1.b.west.us.`). The NATS queue group then naturally favors the most specific match the publisher addresses.

`Publish` keeps a `localResponder` LRU keyed by canonical URL (sans query) recording the longest-suffix locality that successfully responded. Subsequent unicasts inject that prefix into the hostname before publishing. If the locality-prefixed request comes back `404`, `Publish` falls back to the original URL once and clears the cache entry.

Hostnames addressed by instance ID (`<id>.host`) skip locality optimization — the caller has already pinned a specific replica.

### Subject encoding

NATS subjects are derived from `(plane, port, host, method, path)`. Hostnames are reversed segment-by-segment (`com.example.www`) so that NATS's hierarchical wildcards yield suffix-based locality matching. The pipe `.|.` is the fixed delimiter between hostname-side and method/path-side. Path encoding rules:

- An empty path becomes `_`.
- A greedy path argument `{name...}` becomes `>`.
- Other path arguments and segments equal to `*` become `*`.
- Characters outside `[A-Za-z0-9-]` are escaped as `%xxxx` (4-digit lowercase hex of the rune).
- Periods inside path segments become `_` so they don't collide with NATS's segment separator.

Port `0` becomes `*` in subscription subjects (wildcard subscribe), but stays `0` when published — by convention nothing publishes to port `0`.

### Direct addressing for fragmented requests

The first fragment of a multi-fragment request publishes on the normal subject so any replica's queue group can pick it up. Once a replica acks, fragments 2..N are sent to a *direct* subject of the form `<fromID>.<host>` so they all land on the exact replica that took the first fragment. Without direct addressing the queue group would round-robin subsequent fragments across replicas, and the receiving replica would never see a complete request.

The ack op-code reflects this: a fragmented request acks with `100 Continue`, an unfragmented one with `202 Accepted`. The defragger times out a partial fragment set after `8 * networkRoundtrip` of inactivity (polled every `networkRoundtrip / 2`).

### Frame propagation in Publish is an explicit allowlist

Outgoing requests do *not* inherit the inbound request's full header set. `Publish` copies only:

- `X-Forwarded-*` (set by the ingress proxy)
- `Accept-Language`
- The clock-shift header
- The actor (access token) header
- Any header with the baggage prefix

This is a security boundary, not an oversight. Internal control headers (`Microbus-Op-Code`, `Microbus-From-*`, etc.) are set per-call from connector state, so a malicious upstream that smuggled them onto an inbound request cannot have them propagate.

### `Go` and `Parallel` decouple goroutines from the request that started them

`c.Go(ctx, fn)` clones the *frame* of `ctx` (so baggage, clock shift, and actor flow), copies the trace span, but parents the new goroutine on `c.Lifetime()` rather than `ctx`. The goroutine therefore outlives the originating request but is cancelled by Shutdown. `Parallel` follows the same accounting via `pendingOps` so the shutdown drain waits for it.

### `:888` control surface

Control endpoints subscribe twice: once on the connector's own host, once on `//all/<route>` so they are reachable via the broadcast hostname `all`. The OpenAPI handler skips the `//all` mirror entries when rendering the document because they are not separate operations.

The OpenAPI handler is *actor-aware*: operations whose `RequiredClaims` the caller cannot satisfy are omitted from the document. The response is `Cache-Control: private, no-store` because the rendered document varies per caller. Only `function`, `web`, and `workflow` subscription types are included; tasks, events, and infra/control subs are filtered out at this boundary, which is what allows `llm.core` to safely use the document for LLM tool resolution.

### Trace sampling — `selectiveProcessor` is a tail sampler

In `PROD`, spans are not exported eagerly. `selectiveProcessor` buffers ended spans in a ring; when a trace ID is `Select`ed (either by `ForceTrace` locally, or by an inbound `:888/trace?id=...` from a peer that hit an error), the buffer is scanned and matching spans are flushed downstream. Future spans on that trace ID are also flushed.

The selected-trace-IDs map uses a two-generation rotation (`selected1`, `selected2`) primarily as a memory cap — old entries roll out without an explicit TTL sweep. Buffer capacity is fixed at ~8192 spans / ~10MB per microservice.

`ForceTrace` broadcasts to `https://all:888/trace?id=...` so every microservice with spans on that trace exports its share — without this fan-out only the connector that hit the error would flush.

### OTLP exporter resilience

Telemetry export is best-effort sideband — its failure must never affect service health. Two specific configuration choices in `tracer.go` and `metrics.go` enforce this:

1. **`WithRetry(RetryConfig{Enabled: false})`.** With retries on, the OTLP gRPC client retries failed exports with internal timeouts that can exceed 75 seconds per call. A flaky or down collector therefore stalls the connector's batch span/metric processor flushes, which in turn stalls `Shutdown` (the SDK contract is to drain before returning). Disabling retries means each export attempt is single-shot — succeed or drop on the floor with a log line.

2. **No `WithBlock`, no `WithTimeout` option set explicitly.** The SDK's default constructor connects lazily (first export, not at `New(...)`), so `Startup` is never blocked on the dial. The per-export timeout is governed by `OTEL_EXPORTER_OTLP_TIMEOUT` (or the signal-specific `OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` / `OTEL_EXPORTER_OTLP_METRICS_TIMEOUT`) per the OTel spec, in milliseconds. The SDK auto-reads it from `os.Getenv`; values set in `env.yaml` or via `env.Push` (in tests) reach the SDK because the env package writes through to the OS env. The SDK's spec-default 10s applies when unset.

The combined effect: a service with a configured-but-unreachable collector starts up immediately, makes one bounded export attempt per flush (giving up after the configured timeout), and shuts down within `timeout × N` worst case (one final flush per exporter type). Without these settings, the same misconfiguration would hang both startup and shutdown indefinitely — observed pre-fix as orchestrator-killed pods on rolling deploys.

The regression tests live in `metrics_test.go` (`TestConnector_OTLPMetricsUnreachable` for the fast connection-refused path, `TestConnector_OTLPSlowEndpoint` for the slow timeout path that actually exercises the spec-defined timeout).

### `alg=none` JWTs in TESTING

`verifyToken` accepts unsigned tokens (`alg=none`) only when `deployment == TESTING`. Required-claim evaluation still runs against the unsigned payload — TESTING relaxes the *signature* check, not the *authorization* check. This is what lets test code use `pub.Actor(claims)` without standing up a signing key.

### Configurator is disabled in TESTING

`refreshConfig` skips the call to `configurator.core` when `deployment == TESTING` and uses YAML defaults plus values set via `SetConfig`. Tests that want to override config call `SetConfig` / `ResetConfig` directly; outside of TESTING those calls error out.

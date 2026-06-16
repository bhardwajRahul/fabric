# foreman.core

## Agent Instructions

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Overview

The foreman is the orchestrator for agentic workflows in Microbus. It is a **thin Microbus adapter over the
embedded [dwarf](https://github.com/microbus-io/dwarf) workflow engine** (`dwarf/engine`). The engine owns
all orchestration logic — scheduling, execution, fan-out/fan-in, transitions, retries, subgraphs, breakers,
backpressure, the SQL schema, metrics, and tracing. This service owns only the Microbus seam:

- **Bus endpoints** delegate 1:1 to engine methods (`Create`, `Run`, `Resume`, `Cancel`, …). The service
  struct holds the engine as a member (`svc.engine`), built in `OnStartup`, drained in `OnShutdown`.
- **`engine.Host` implementation** (`host.go`): `LoadGraph` (GET the graph over the bus), `ExecuteTask`
  (mint the actor token from baggage, POST the flow to the task URL, classify the error into the engine's
  backpressure/breaker wrappers), `FlowStopped` (fire `OnFlowStopped` to the notify host read from the flow's baggage), and
  `SignalPeers` (multicast the opaque `(op, payload)` to peer replicas via the single `Signal` endpoint).
- **`Signal` inbound endpoint** filters self/foreign delivery, then hands `(op, payload)` to
  `engine.DeliverSignal`.
- **Identity**: actor claims + tenant are read from `frame.Of(ctx)` at `Create`/`Run` and passed to the
  engine as opaque `FlowOptions.Baggage`; the engine never interprets them.
- **Config / lifecycle**: `OnStartup` builds the engine from the config (`SQLDataSourceName`, `Workers`,
  `TimeBudget`, `DefaultPriority`, `NumShards`, `SQLConnectionPool`), injects the connector's
  meter/tracer/slog providers, and starts it.

The engine's internal design rationale lives in dwarf's own `CLAUDE.md`; this file covers only the adapter
and the decisions that are non-obvious *because* the foreman is now a delegating shell.

## Non-obvious decisions

**Error classification is the host's job, not the engine's (`classifyTaskError` in `host.go`).** The engine
never inspects a task's HTTP status code or error text — it only understands two opaque disposition
wrappers. The foreman maps transport errors to them: `429 → workflow.ErrRateLimited` (rate-cut + bounce
the step), and `404 "ack timeout…" / 503 / 529 → workflow.ErrUnavailable` (park the backlog + exponential
probe), tagging the trip with an opaque `cause` label (`ack_timeout` / `unavailable` / `overloaded`) that
the engine forwards only as a metric dimension. Any other error is returned **undecorated**, which the
engine treats as an ordinary failure (routed via `onError` or failed). This keeps the engine
transport-agnostic; the 429-vs-breaker distinction (rate-limited vs. can't-serve-right-now) and the `529`
constant (`statusOverloaded` — net/http has none) live here because only the host knows the wire protocol.

**Cross-replica coordination rides one `Signal` multicast endpoint.** The engine's
`SignalPeers(ctx, op, payload)` carries every kind of peer signal (work doorbell, valve-rate gossip,
breaker trip, status-change wake) as a single opaque `(op, payload)` pair. `op` is an opaque routing key
the engine parses on the receiving side via `DeliverSignal`; the foreman never branches on it, so a new
signal kind needs zero adapter changes — one endpoint covers all of them.

**Self-delivery filter on inbound `Signal`: `FromHost==Hostname && FromID!=svc.ID`.** Microbus multicast
echoes to the sender. The engine's contract is that a signal is applied *locally* before `SignalPeers` is
called, so the originating replica must drop the echo or it double-applies (a re-trip would reset the
breaker's accumulated backoff, a re-gossiped valve would distort the rate). The `FromHost==Hostname` half
restricts processing to genuine foreman peers; the `FromID!=svc.ID` half excludes self. Both are required.

**Telemetry providers are set before `Startup` — and only there.** `SetLogger` / `SetMeterProvider` /
`SetTracerProvider` are construction-time-only on the engine (it resolves them once at `Startup`; calling
them on a running engine returns an error). So `OnStartup` calls `eng.SetLogger(svc.Logger())` etc. *before*
`eng.Startup(ctx)`. (The engine's config API is `Set*`, each returning an `error`; the pre-`Startup` sets
here can't fail, so the real error surfaces from `Startup`.) The foreman defines **no** microbus metric
endpoints: the engine emits `dwarf_*` instruments and spans straight through the connector's OTEL pipeline.
(The Grafana dashboard at `setup/grafana/dashboards/workflow-overview.json` reads the `dwarf_*` names. Note
the metric label split: step-disposition keys by **`task_name`** (graph node), the adaptive/breaker metrics
key by **`task_url`** (downstream endpoint).)

**`StartupInTest` under the TESTING deployment, `Startup` otherwise.** The foreman runs under
`app.RunInTest`, so it has no `*testing.T` to hand the engine's `RunInTest`. `StartupInTest(ctx, svc.Plane())`
is the `*testing.T`-free path: it opens isolated throwaway databases keyed by the Microbus **plane**, which
every replica in a test app shares — so a multi-replica shared-state fixture resolves to the same isolated
DBs. The resolved DSN is used only as a base; the engine creates the throwaway databases off it.

**DSN resolution is per deployment.** `LOCAL` with no configured DSN falls back to
`file:shard_%d.local.sqlite` (one SQLite file per shard); PROD/LAB use the `SQLDataSourceName` secret. The
engine enforces the `%d`-required-when-`NumShards>1` rule itself.

**Actor token is minted per `ExecuteTask` from baggage (`mintActorToken`).** The original caller's actor
claims are captured into `FlowOptions.Baggage` at `Create`/`Run` and ride the dispatch ctx for the flow's
whole life. Each task dispatch mints a *fresh* access token from those claims so the downstream task runs as
the original actor, even hours later. The `iss`/`idp` swap sets the minted token's issuer to the actor's
original identity provider, so the downstream authorizes against the right issuer. No claims → empty token →
unauthenticated dispatch.

**Fairness key defaults to the caller's tenant (`resolveOptions`).** When the caller does not set an
explicit `FairnessKey`, the foreman fills it with the frame's tenant id, so cross-tenant fairness works
out of the box without the engine knowing what a tenant is.

**Stop-notification delivery target rides in baggage, not in an engine column.** When a caller sets
`FlowOptions.NotifyOnStop`, `resolveOptions` stamps the caller's host (`frame.FromHost()`) into baggage
under `baggageNotifyHost` ("notifyHost"); on stop the engine fires `FlowStopped(ctx, outcome)` with that
baggage on the ctx, and `FlowStopped` reads the host back to `ForHost(...)` the `OnFlowStopped` event. The
engine carries no delivery address (a hostname it would merely store and echo is exactly what baggage is
for), which is why notification is a `Create`-time flag (`NotifyOnStop`) and not its own start endpoint.
Because `mintActorToken` mints from *all* baggage keys, it deletes `baggageNotifyHost` first so the foreman
bookkeeping never leaks into the minted token's claims.

**`NumShards` changes apply live.** `OnChangedNumShards` calls `svc.engine.SetNumShards(n)`, which records
the new target and, on a running engine, opens+migrates the added shards in place — new flows spread onto
them immediately, existing flows stay put. Growth only: a decrease records the target but removes nothing
(old shards drain; an actual reduction takes effect on restart).

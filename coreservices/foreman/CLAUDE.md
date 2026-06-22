# foreman.core

## Agent Instructions

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Overview

The foreman is the orchestrator for agentic workflows in Microbus. It is a **thin Microbus adapter over the
embedded [dwarf](https://github.com/microbus-io/dwarf) workflow engine** (`dwarf/engine`). The engine owns
all orchestration logic — scheduling, execution, fan-out/fan-in, transitions, retries, subgraphs, the SQL
schema, metrics, and tracing. This service owns only the Microbus seam:

- **Bus endpoints** delegate 1:1 to engine methods (`Create`, `Run`, `Resume`, `Cancel`, …). The service
  struct holds the engine as a member (`svc.engine`), built in `OnStartup`, drained in `OnShutdown`.
- **`engine.Host` implementation** (`host.go`): `LoadGraph` (GET the graph over the bus), `ExecuteTask`
  (mint the actor token from baggage, POST the flow to the task URL, return any transport error undecorated),
  `FlowStopped` (fire `OnFlowStopped` to the notify host read from the flow's baggage), and
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

**The foreman does NOT classify task errors.** `ExecuteTask` returns every transport error **undecorated**
(`errors.Trace(err)`), which the engine treats as an ordinary failure - routed via the graph's `onError`
transition if one exists, else it fails the step. There are no engine-side backpressure dispositions to wrap into:
dwarf has no adaptive valve or circuit breaker (they were removed). Backpressure is owned by the layer that holds
the resource identity, not the engine: a task that wants to back off reads its own signal (e.g. an LLM provider's
`retryAfter`) and arms `flow.Retry` itself (carrying the wait in Retry's `initialDelay`) - see `coreservices/llm`
`CallLLM`. The status code is preserved on the error (a 429 stays a 429) - it is just not a control signal.

**The one exception: the foreman retries a `404` ack-timeout itself.** A `404` ack-timeout means *no
microservice acked the task dispatch* - the task host is absent (a deploy gap, a not-yet-started replica), not a
task that ran and returned `404`. Because the task body never executed, it could not arm its own `flow.Retry`,
so the foreman arms it on the carrier (`isAckTimeout` gates on status `404` plus the connector's `"ack timeout"`
message) and returns `nil`. The engine then re-dispatches exactly as if a task had retried. **The retry horizon
is the step's own time budget** - not a config. The engine bounds the dispatch ctx
with the step's `TimeBudget` (foreman config, default 2m, capped 15m, or a task's shorter `sub.TimeBudget`);
`ExecuteTask` reads it from `ctx.Deadline()` (not `frame.TimeBudget` - the engine sets a ctx deadline, not a
frame header) and passes it as `flow.Retry`'s `giveUpAfter`, measured from the step's creation. So "how long to
keep probing a missing host" rides on the same knob that says "how long is this task worth running," with **zero
new configs** - and a task that declares a short `sub.TimeBudget` automatically gets a short ack-timeout horizon.
The re-probe *cadence* is fixed: a missing host is absent, not overloaded (a probe is a cheap ack-timeout that
runs no handler), so there is nothing to back off from - we want fast, uniform recovery detection. The interval
is `budget / ackTimeoutRetryProbes` (const, 8), so ~8 evenly-spaced probes across the budget regardless of its
length; `flow.Retry`'s next-delay give-up stops one probe short of overshooting. This is the **only**
adapter/engine-level retry; every other backoff is task-owned. It replaces the deleted breaker's missing-host
handling, and the give-up is an ordinary step failure, so a recovery query/alert can find it by the failed status
and ack-timeout error text. For longer-than-budget tolerance, the *caller* owns the retry (re-run with a longer
budget or its own loop) - the framework deliberately does not park work past the task's stated worth.

**Cross-replica coordination rides one `Signal` multicast endpoint.** The engine's
`SignalPeers(ctx, op, payload)` carries every kind of peer signal (the work doorbell and the status-change
wake) as a single opaque `(op, payload)` pair. `op` is an opaque routing key the engine parses on the
receiving side via `DeliverSignal`; the foreman never branches on it, so a new signal kind needs zero adapter
changes — one endpoint covers all of them.

**Self-delivery filter on inbound `Signal`: `FromHost==Hostname && FromID!=svc.ID`.** Microbus multicast
echoes to the sender. The engine's contract is that a signal is applied *locally* before `SignalPeers` is
called, so the originating replica must drop the echo or it double-applies (e.g. a doubled work doorbell or
status-change wake). The `FromHost==Hostname` half restricts processing to genuine foreman peers; the
`FromID!=svc.ID` half excludes self. Both are required.

**Telemetry providers are set before `Startup` — and only there.** `SetLogger` / `SetMeterProvider` /
`SetTracerProvider` are construction-time-only on the engine (it resolves them once at `Startup`; calling
them on a running engine returns an error). So `OnStartup` calls `eng.SetLogger(svc.Logger())` etc. *before*
`eng.Startup(ctx)`. (The engine's config API is `Set*`, each returning an `error`; the pre-`Startup` sets
here can't fail, so the real error surfaces from `Startup`.) The foreman defines **no** microbus metric
endpoints: the engine emits `dwarf_*` instruments and spans straight through the connector's OTEL pipeline.
(The Grafana dashboard at `setup/grafana/dashboards/workflow-overview.json` reads the `dwarf_*` names. Note
the metric label split: step-disposition keys by **`task_name`** (graph node), while `dwarf_task_concurrency_running`
keys by **`task_url`** (downstream endpoint).)

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

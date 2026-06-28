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

The retry horizon is the task's **time budget**, not a config and not a flow-wide deadline. A config was rejected
because the real bound is *staleness* ("is this work still worth finishing?"), which the time budget already
expresses, so reusing it means zero new knobs, per-task granularity (a short `sub.TimeBudget` yields a short
horizon), and one unified rule for the 404 and 429 paths. A flow-wide deadline (a `FlowOption` "fail the whole flow
after T") was rejected because under a wall-clock-since-created clock a HITL flow parked on an interrupt for days, or
a legitimate long `flow.Sleep`, is indistinguishable from one stuck retrying - the engine cannot tell "still
waiting" from "still failing." Splitting ownership sidesteps that: the engine owns the **short-term** retry (bounded
by the per-task budget, which only counts a task that is actively trying), and the **caller** owns the long-term,
staleness-aware retry (it alone knows whether the work is still relevant). The 429 path in `coreservices/llm`
`CallLLM` resolves identically - same budget-as-horizon, caller-owned long retry via `Chat` returning its partial
messages plus `llmapi.RetryAfter`.

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
here can't fail, so the real error surfaces from `Startup`.) The engine emits `dwarf_*` instruments and spans
straight through the connector's OTEL pipeline. (The Grafana dashboard at
`setup/grafana/dashboards/workflow-overview.json` reads the `dwarf_*` names. Note the metric label split:
step-disposition keys by **`task_name`** (graph node), while `dwarf_task_concurrency_running` keys by
**`task_url`** (downstream endpoint).)

**The foreman emits exactly one microbus metric of its own: `AckTimeouts`** (OTel name
`microbus_foreman_timeout_requests`; the Prometheus exporter appends `_total`, so it queries as
`microbus_foreman_timeout_requests_total`, labeled `task_url`, `outcome`). It records the 404 ack-timeout path in
`ExecuteTask` - `outcome="retry"` on each re-probe, `outcome="giveup"` when the horizon is spent and the step is
failed (the alertable "a microservice is missing" signal). It lives here, not in the engine, because the engine is
host-agnostic and never sees the ack-timeout: it only observes that a `flow.Retry` was armed (indistinguishable
from a task-armed 429 retry) or that an error came back (indistinguishable from any other failure). General
retry-dispatch churn, by contrast, *is* an engine signal - `dwarf_steps_executed_total{status="retried"}` - and
needs no foreman metric. Label `task_url` matches the value dwarf uses for `dwarf_task_concurrency_running` (the
dispatch URL `ExecuteTask` receives), so the two join on the same key.

The name parallels the framework's own `microbus_client_timeout_requests`, which the connector already increments
for *every* downstream timeout. We do not just filter that by `service="foreman.core"` because (a) it conflates the
ack-timeout with the foreman's other downstream calls (token mint, graph load) and carries no `retry`/`giveup`
split, and (b) the `service` label is the deploy-time hostname, so an alternative-hostname deployment would silently
break a dashboard hard-coded to `foreman.core`. The dedicated metric is hostname-independent and pre-split.

**`SetInTest(plane)` + `Startup` under the TESTING deployment, plain `Startup` otherwise.** The foreman runs
under `app.RunInTest`, so it has no `*testing.T` to hand the engine's `RunInTest`. `eng.SetInTest(name)` is
the `*testing.T`-free hook: it keys the engine's isolated, auto-dropped test databases by `name`, and the
following `Startup` opens them. The foreman passes the Microbus **plane**, which every replica in a test app
shares — so a multi-replica shared-state fixture resolves to the same throwaway DBs. (`SetInTest` is exactly
what the engine's `RunInTest(t)` calls with `t.Name()`; the foreman just supplies a plane instead of a test
name.) The resolved DSN is used only as a base; the engine creates the throwaway databases off it.

**DSN resolution is per deployment.** `LOCAL` with no configured DSN falls back to
`file:shard_%d.local.sqlite` (one SQLite file per shard); PROD/LAB use the `SQLDataSourceName` secret. The
engine enforces the `%d`-required-when-`NumShards>1` rule itself.

**Actor token is minted per `ExecuteTask` from baggage (`mintActorToken`).** The original caller's actor
claims are captured into `FlowOptions.Baggage` at `Create`/`Run` and ride the dispatch ctx for the flow's
whole life. Each task dispatch mints a *fresh* access token from those claims so the downstream task runs as
the original actor, even hours later. The `iss`/`idp` swap sets the minted token's issuer to the actor's
original identity provider, so the downstream authorizes against the right issuer. No claims → empty token →
unauthenticated dispatch.

**Only the genesis endpoints route through `resolveOptions`.** Policy (`FlowOptions`) is authored once at
genesis - `Create` and `Run` - so only those two inject the caller's actor claims, notify host, and tenant
fairness key into baggage. The derived operations take no options and inherit their source's policy:
`Continue` inherits the thread's, `Fork` inherits the origin's. Their foreman handlers therefore delegate
straight to the engine with no `resolveOptions` call - re-injecting the *current* caller's identity would
be wrong, since a derived flow runs as the original actor (the inherited baggage already carries those
claims). A new caller who wants their own identity/policy on a thread uses `Create` with
`Opts.ThreadKey`, which is the explicit-policy path and does run through `resolveOptions`.

**There is no `Start` endpoint; `Create` auto-runs.** The engine folds create-and-run into one
transaction (dwarf v0.8.0), so the foreman exposes no `Start` and `Create`/`Continue`/`Fork` all return a
flow that is already `running`. `Run` remains `Create` + `Await`. A deferred start is authored in the
workflow itself - an entry task that calls `flow.Interrupt`, released by `Resume` - not via a foreman flag.

**Terminal flows are immutable; the bus surface offers no in-place re-run.** A `completed`/`failed`/
`cancelled` flow is frozen - the only operations on it are read (`Snapshot`/`History`) and removal
(`Delete`/`Purge`). The old mutate-in-place endpoints (`Restart`/`RestartFrom`/`Recover`) and the
breakpoint endpoints (`BreakBefore`/`ResumeBreak`) are gone with no replacement; recovery is `Fork`, which
clones the chosen prefix into a *new* flow and never touches the original. The invariant itself is the
engine's (its rationale lives in dwarf's `CLAUDE.md`); the adapter consequence is simply that no foreman
endpoint can move a terminal flow off its frozen outcome.

**Fairness key defaults to the caller's tenant (`resolveOptions`).** When the caller does not set an
explicit `FairnessKey`, the foreman fills it with the frame's tenant id, so cross-tenant fairness works
out of the box without the engine knowing what a tenant is.

**Stop-notification delivery target rides in baggage, not in an engine column.** When a caller sets
`FlowOptions.NotifyOnStop`, `resolveOptions` stamps the caller's host (`frame.FromHost()`) into baggage
under `baggageNotifyHost` ("notifyHost"); on stop the engine fires `FlowStopped(ctx, outcome)` with that
baggage on the ctx, and `FlowStopped` reads the host back to `ForHost(...)` the `OnFlowStopped` event. The
engine carries no delivery address (a hostname it would merely store and echo is exactly what baggage is
for), which is why notification is a `Create`-time flag (`NotifyOnStop`) and not its own endpoint.
Because `mintActorToken` mints from *all* baggage keys, it deletes `baggageNotifyHost` first so the foreman
bookkeeping never leaks into the minted token's claims.

**`NumShards` changes apply live.** `OnChangedNumShards` calls `svc.engine.SetNumShards(n)`, which records
the new target and, on a running engine, opens+migrates the added shards in place — new flows spread onto
them immediately, existing flows stay put. Growth only: a decrease records the target but removes nothing
(old shards drain; an actual reduction takes effect on restart).

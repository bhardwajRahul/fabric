## Agent Instructions

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Overview

The foreman is the orchestrator for agentic workflows in Microbus. It executes workflow graphs by dispatching tasks to microservices, managing state between steps, and handling fan-out/fan-in, interrupts, retries, and failure recovery.

### Core Concepts

**Workflow graph** - A directed graph that defines the structure of a workflow: which tasks to execute, in what order, and under what conditions. Graphs are defined in code using the `workflow.Graph` API and served as JSON via a dedicated endpoint. Each graph has a name, an entry point, a set of tasks, transitions between them, and optional reducers for fan-in state merging.

**Task** - An addressable endpoint on the bus (port `:428`) that performs a unit of work within a workflow. Tasks receive state via a `workflow.Flow` carrier, read input from state fields, perform work, and write output back to state fields. Tasks are reusable across workflows and can live on any microservice.

**Flow** - A single execution of a workflow graph. Each flow has a unique ID, tracks its current position in the graph, and maintains a state map that evolves as tasks execute. Flows progress through statuses: `created` → `running` → `completed`/`failed`/`cancelled`, with `interrupted` as a parked state for human-in-the-loop scenarios.

**Step** - A single task execution within a flow. Each step captures an immutable state snapshot (input), the changes produced by the task (output), and metadata like status, error, and timing. Steps are numbered sequentially (`step_depth`); parallel fan-out siblings share the same `step_depth`. Once a step reaches a terminal status (`completed`, `failed`, `cancelled`), it is immutable.

**Reducer** - A merge strategy for state fields during fan-in. When parallel branches converge, each branch's changes are merged using the reducer chosen for that field: `replace` (last write wins, default), `append` (concatenate arrays), `add` (sum numbers), `union` (deduplicate arrays), or `merge` (combine objects, new key wins). A field with no registered reducer uses `replace`; every non-default fan-in field is wired explicitly with `graph.SetReducer(name, reducer)` (the older `sum*`/`list*`/`set*` name-prefix inference was removed).

**Thread** - A chain of flows linked by `Continue`. Each flow has a `thread_id` column that groups it with other flows in the same multi-turn conversation. By default, `thread_id = flow_id` (each flow is its own thread). When `Continue` creates a new flow, it inherits the thread's `thread_id`. Subgraph flows always start their own thread (`thread_id = flow_id`) to avoid contaminating the original thread's continuation chain. The flowKey returned by the initial `Create` doubles as the threadKey - callers can pass it (or any flowKey in the chain) to `Continue` or `List` to address the thread.

### Flow Lifecycle

```
Create ──► created ──► Start ──► running ──► completed
                                   │  ▲
                                   │  │ Resume
                                   ▼  │
                               interrupted
                                   │
                                   ▼
                         failed ◄──┘
                           │
                           │ Restart / RestartFrom
                           ▼
                         running ──► ...

                         cancelled (via Cancel)
```

1. **Create** (or `CreateTask`) inserts a flow and its first step in `created` status
2. **Start** transitions the flow to `running` and its steps to `pending`, then enqueues for execution
3. A worker picks up the step, executes the task, and evaluates transitions to create next steps
4. This repeats until no transitions match (flow completes), a task returns an error (flow fails), or the flow is cancelled
5. Tasks can call `flow.Interrupt()` to pause for external input; `Resume` continues execution
6. Terminated flows can be re-run with `Restart` (from the entry step) or `RestartFrom` (from a chosen step), optionally with state overrides

### Foreman Operations

**Create** - Creates a new flow without starting it. Fetches the workflow graph via a direct GET to the workflow's endpoint URL, creates the flow row, and inserts the entry-point step - both in `created` status. The graph JSON is frozen at creation time. `CreateTask` is a convenience variant that accepts a single task name and wraps it in a trivial graph.

**Start** - Transitions a `created` flow to `running`. Atomically updates all `created` steps to `pending` and the flow to `running` within a transaction, then enqueues the current step for execution. Does not set up notifications.

**StartNotify** - Like `Start`, but also stores a `notify_hostname` on the flow. When the flow stops (completed, failed, cancelled, or interrupted), the foreman fires an `OnFlowStopped(flowKey, status, snapshot)` event targeted at that hostname. The subscriber must use `foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(handler)` to receive these events.

**Snapshot** - Returns the current state, status, task name, and step number of a flow. During fan-out (`step_id=0`), it queries the microbus_steps table directly to find the active steps.

**Resume** - Continues a flow paused by `flow.Interrupt`. Walks up the surgraph chain (`surgraphChain`) and down the interrupted subgraph chain (`interruptedSubgraphChain`) to find the leaf interrupted step. Records resume data on the leaf step's `resume_data` column (the leaf already has `interrupt_done=1`, set when the task armed `flow.Interrupt`); on re-dispatch the task's `flow.Interrupt` call returns that data with `yield=false`. Resume data is no longer merged into `state`/`changes` - the task receives it as the return value, not by field name. Re-parks intermediate surgraph steps, resets the leaf to `pending`, and transitions all flows in the chain to `running`. Can be called on any flow in the chain (root or subgraph) - propagation goes both directions. If multiple fan-out siblings interrupt, each `Resume` handles one sibling; the flow only transitions to `running` when no more interrupted steps remain.

**ResumeBreak** - Continues a flow paused at a `BreakBefore` breakpoint. Shares all of `Resume`'s chain-walking and re-park machinery (both are thin wrappers over the private `resume`), but instead of recording resume data it merges the caller's `stateOverrides` onto the leaf step's `state` (replace semantics via `MergeState(..., nil)`) so the about-to-run task observes the edits - the breakpoint pauses *before* the task runs, so injecting state is the only way to influence it, and editing state is the point of a debug breakpoint. `breakpoint_hit=1` is left set so the re-dispatch skips the breakpoint rather than re-triggering it.

**Resume and ResumeBreak are strictly separated, never auto-routed.** A breakpoint pause and a `flow.Interrupt` pause both carry flow status `interrupted`; the discriminator is the *leaf step*, where exactly one of `interrupt_done=1` (a `flow.Interrupt` park) or `breakpoint_hit=1` with `interrupt_done=0` (a breakpoint) holds. The private `resume` reads those flags and fails with 409 when the leaf's kind does not match the entry point: `Resume` refuses a breakpoint pause, `ResumeBreak` refuses an interrupt pause. This is deliberate - detection tells you *which leaf you are at*, not *what the caller intended*, and the two operations have different observable effects (a resume payload is delivered to `flow.Interrupt`'s return and is **not** merged into state; breakpoint overrides **are** merged into state). Auto-routing a mismatched call would silently merge an interrupt answer into state under field names the task may not read, let the task re-arm its interrupt and re-pause, and strand the caller. The method name is the caller's assertion of the pause kind; a mismatch surfaces loudly instead of doing the wrong thing. The kind stays a *step*-level distinction (not a flow status) because fan-out branches can pause for different reasons at once, which a single flow `status` column cannot represent.

**Cancel** - Aborts a created, running, or interrupted flow. Uses `surgraphChain` to walk up to the root and `allSubgraphFlows` to walk down to all descendants. Atomically cancels all steps across all flows, computes `final_state` for each flow, and cancels all flows with per-flow `final_state` via CASE - all within a single transaction. Can be called on any flow in the chain (root or subgraph).

**Restart** - Re-runs a terminated flow from its entry step. Deletes every step except the entry step (and any descendant subgraph flows), resets the entry step to `pending` with `state = merge(originalEntryState, stateOverrides)`, and flips the flow row to `running`. The flow must be in a terminal status (`completed`/`failed`/`cancelled`/`interrupted`). `stateOverrides` is a top-level shallow replace over the original entry state: keys present win, keys mapped to `nil` are deleted, absent keys are preserved (pass `nil` for none). A fresh attempt, so `created_at`/`started_at` are reset. This replaces the old `Retry` (re-run last failed step) and the old `Fork` (rewind-with-overrides) use cases.

**RestartFrom** - Re-runs a flow from a chosen step (a surgical rewind, not a whole-flow restart). Sweeps the target step's downstream DAG subtree (every step reachable via `successor_id`), decrements the affected spawns' cohort counters for the swept members, cascade-deletes descendant subgraph flows spawned by any swept step, UPDATEs the target step in place to `pending` with `state = merge(originalStepState, stateOverrides)`, and flips the flow to `running`. When the target lives inside a subgraph child flow, the sweep cascades up the surgraph chain around each caller step (the parent's downstream consumed the now-stale child result), but the caller's own child flow is not deleted. The target step must be terminal; its flow can be in any non-cancelled status. The flow's run continues, so `started_at` is *not* reset.

**History** - Returns the step-by-step execution history as a slice of `FlowStep` records. Each `FlowStep` includes the step's key, depth, task name, state, changes, status, error, and timestamp. Steps that executed a subgraph have `Subgraph=true` and their `SubHistory` field contains the nested execution history of the subgraph flow.

**List** - Queries flows by status, workflow name, or thread key. Supports cursor-based pagination via `cursorFlowKey` (newest first). Defaults to 100 results. Returns `ThreadKey` in each `FlowSummary`. When filtering by `ThreadKey`, only flows in that thread are returned (scoped to the thread's shard).

**BreakBefore** - Sets or clears a breakpoint that pauses execution before the named task runs. Breakpoints are stored in a `breakpoints` JSON column on the flow row as a `map[string]string` keyed by task name. During `processStep`, if the current task name matches a breakpoint and the step's `breakpoint_hit` flag is false, the foreman interrupts the flow (using the same interrupt propagation as `flow.Interrupt()`) and sets `breakpoint_hit=1` on the step. The flag prevents the breakpoint from re-triggering when the flow is resumed. A breakpoint pause is continued with `ResumeBreak`, not `Resume` (which is for `flow.Interrupt` and rejects a breakpoint pause with 409). Breakpoints are inherited by subgraph flows.

**Continue** - Creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers. The `threadKey` parameter accepts any flowKey belonging to the thread - `Continue` resolves the thread via the flow's `thread_id` column, finds the latest flow in that thread (`ORDER BY flow_id DESC`), validates it is completed, and creates the new flow in the same thread. The new flow uses the same workflow graph and is returned in `created` status. This enables multi-turn patterns where each turn is a separate flow: the caller holds a single threadKey (the flowKey returned by the initial `Create`) and calls `Continue` repeatedly without needing to track intermediate flowKeys. The new flow's `final_state` from the prior turn passes through unfiltered as its initial state, so any field present at the end of one turn is visible at the start of the next; a workflow author who wants narrower turn-to-turn carryover puts an adapter task at the workflow's entry that uses `flow.Delete`/`Transform` to scrub. The new flow's scheduling (priority, fairness) comes from caller-supplied `FlowOptions` like a fresh `Create`; a nil opts uses fresh defaults rather than inheriting from the prior flow.

**Enqueue** - Internal endpoint (port `:444`) that rings the work **doorbell**: it signals that a step is pending. The
receiving replica does one PK lookup for the announced step's `priority` and `not_before`. If `not_before` is in the
future the doorbell defers to the poll timer (`shortenNextPoll(not_before)`) - the work cannot run now, so there is
nothing to preempt and the cache stays untouched. If `not_before` is due, the priority drives the cache offer
(refill or head-insert; see "Execution Model"). It does not enqueue a specific step. Load-balanced across foreman
replicas. Fire-and-forget - errors are ignored because `pollPendingSteps` rings the local doorbell each cycle and
recovers any missed work.

### FlowOutcome and side-channel signals

`Snapshot`, `Await`, and `Run` all return a single `*foremanapi.FlowOutcome` (defined in `foremanapi/flowoutcome.go`). The same struct is the payload of the `OnFlowStopped` outbound event. The shape is:

```go
type FlowOutcome struct {
    FlowKey          string
    Status           string
    State            map[string]any
    Error            string         // populated when Status == "failed"
    InterruptPayload map[string]any // populated when Status == "interrupted"
    CancelReason     string         // populated when Status == "cancelled"
}
```

Side-channel fields are populated only for the matching status. `Run`'s Go-level `error` return is reserved for transport/foreman/timeout failures; a *workflow failure* surfaces as `outcome.Status == "failed"` with `outcome.Error` set, so callers don't have to disambiguate "the workflow rejected my input" from "the foreman is down."

The interrupt path is deliberately split from `State`: `Snapshot` of an `interrupted` flow returns `State` as the merged step snapshot *at the time of the interrupt* and `InterruptPayload` as the raw `flow.Interrupt(payload)` argument. The previous behavior (merging the payload into `State` via the graph's reducers) is gone because the merge was lossy: once folded in, the caller could not tell which fields were the workflow's own state and which were the resume request. Callers that want the merged view can call `workflow.MergeState(out.State, out.InterruptPayload, graph.Reducers())` themselves.

### Flow Stop Notifications

When a flow is started via `StartNotify(flowKey, notifyHostname)`, the foreman stores the hostname and fires `OnFlowStopped(*FlowOutcome)` events targeted at that hostname when the flow stops - i.e. reaches a terminal status (`completed`, `failed`, `cancelled`) or is `interrupted`. This matches the statuses that `Await` returns on.

The outcome carries the same fields as a `Snapshot`/`Await` return at the stop point: `State` is the flow's `final_state` for terminal statuses, the leaf step's accumulated state for `interrupted`. `Error`, `InterruptPayload`, and `CancelReason` are populated per Status as documented above.

The notification is fire-and-forget - flow execution never blocks on notification delivery. The `notify_hostname` is set only on the root flow (via `StartNotify`); subgraph flows do not receive direct notifications. Interrupt notifications query the root flow's hostname from the surgraph chain.

Subscriber pattern:
```go
foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(svc.OnFlowStopped)
```

Where `svc.OnFlowStopped(ctx context.Context, outcome *foremanapi.FlowOutcome) error` is the handler.

### Execution Model

The foreman uses a **queue-as-cache execution model** with a configurable pool of worker goroutines
(`Workers`) and a single refiller goroutine per replica. The in-memory structure (`queue.go`) is a bounded
`candidateCache`, not a work queue: it holds *hints*, not ownership. Each worker pops a candidate and calls
`processStep`:

1. Reserve the step (atomic CAS `UPDATE ... WHERE step_id=? AND status='pending' AND not_before<=NOW AND lease_expires<=NOW`)
2. Check for terminal flow status (abort if cancelled/failed/completed)
3. Load the flow's graph, config, and actor claims
4. Execute the task via HTTP call to the task endpoint with a time budget
5. Persist changes, evaluate transitions, create next steps (in a transaction), ring the doorbell

Acquisition stays the atomic CAS, so a stale or duplicated candidate is harmless: the CAS loser returns `nil`
and the worker pops the next one. The cache holds hints, never ownership; only the CAS grants a step.

**Selection (two-level priority + fairness).** The refiller, not the worker, decides *what* runs.
(1) Each shard is scanned for *its* strict-minimum `priority` band's due pending rows in a single SQL
statement - the band is a `priority=(SELECT MIN(priority) ... due)` subquery, so band and candidates are
self-consistent within the statement (still not transactional vs concurrent worker CAS claims, which is
self-correcting via the post-completion refill and the backlog poll). (2) The shards' rows are aggregated:
the *global* minimum band across shards is taken (strict priority is cluster-wide, not per shard) and only
rows at that global band form one `fairness_key` population - shards with a worse band contribute nothing
this batch (only the global-min band is ever materialized - lower-priority bands are never selected until
the higher one drains, by design). (3) Repeatedly weighted-random pick a key (Efraimidis-Spirakis over the
*keys*, not the rows) and take that key's *oldest* remaining step until the batch is full - FIFO within a
`fairness_key`. `created_at` (read as an age, comparable across shards) does two things, both per-key: it
fixes the key's `fairness_weight` from the key's oldest step (so a tenant cannot self-promote by submitting
newer high-weight tasks), and it orders dispatch oldest-first within the key - the same step does both. It
is the only ordering signal comparable across shards: `step_id` is a per-shard auto-increment, so a
`(shard, step_id)` order would let a brand-new task on a low shard jump an old task on a high shard for the
*same* tenant (unbounded intra-tenant starvation). The age is `DATE_DIFF_MILLIS(NOW_UTC(), created_at)`
evaluated on each shard, and `created_at` defaults to that same shard's `NOW_UTC()` at insert (no INSERT
sets it explicitly) - both terms on one shard clock, so the per-shard clock offset *cancels exactly*: there
is no inter-shard clock-skew term in `ageMs`. The only residual is the few-millisecond dispersion in *when*
each shard runs its age query (the per-shard scans run in parallel within one refiller pass via
`svc.Parallel`), a soft, self-correcting reordering of one tenant's own queue - not a fairness violation
(the weighted *key* pick, not step order, governs cross-tenant fairness) and not a correctness issue (the
`processStep` CAS arbitrates acquisition); same-age ties break by `(shard, step_id)` for determinism. The
pick is re-rolled per step so expected dispatch share is
proportional to weight and independent of backlog depth or shard layout - fairness is over `fairness_key`,
never over shards (the reason flat per-row weighted random was rejected: it rewards backlog, the opposite
of tenant isolation).
Strict priority means no aging: a fed higher-priority band starves lower bands by design.

**Queue-as-cache, doorbell, single-slot refiller.** `Enqueue` no longer carries a step to run in a queue;
it is a **doorbell** (`candidateCache.offer`). It resolves the announced step's priority *and* `not_before` in
one PK lookup (off the selection path). If `not_before` is in the future, the doorbell short-circuits into
`shortenNextPoll(not_before)` and returns - the work is not due, so there is nothing to preempt and the cache
stays untouched; the local poll timer wakes at the right moment and the normal scan picks the step up. This
is also how cross-replica delayed-start propagates: every replica that receives the multicast doorbell pulls
its poll timer forward, with no need for a separate "wake at T" message. Otherwise the priority drives one of
three cache paths. (1) Empty cache: this replica is idle - request a
refill so the refiller selects the strictly-best pending step. It deliberately does **not** head-insert the
first arrival, because an arbitrary-priority step jumping the queue on an idle replica can run before a
more important one (this exact inversion was observed and is why the empty case defers to the refiller; the
cost is one refiller scan of idle-wake latency, not zero). (2) Non-empty and not strictly more important
than the cached band (priority >= floor, equal included): no-op - a steady same-or-lower-priority stream is
pure cache hits with no blanket requery. (3) Non-empty and strictly more important (priority < floor):
**head-insert that exact step** so the next pop runs it without waiting a refiller scan, lower the floor,
wake one waiter, and request a refill to top up the rest of the band. Case 3 is the valuable one - an
urgent arrival preempting cached lower-priority work - and it deliberately does **not** flush the existing
candidates: a guiding principle of this dispatcher is that high throughput trumps exact priority ordering.
Flushing would idle every worker through the refill scan to guarantee zero lower-priority executions after
a higher-priority arrival; instead the workers keep draining (or run the head-inserted urgent step next)
and the refiller's wholesale replace re-establishes strict band order within one refill cycle. Exact
priority ordering is soft anyway - with N replicas each draining independently there is no global order to
preserve. The CAS in `processStep` still arbitrates acquisition, so a head entry that is stale or
duplicated across replicas is harmless. The cache is bound to `size` by trimming the tail on insert; a
trimmed step stays `pending` and is re-selected.
Each shard is scanned for its own strict-minimum band, but the refiller then aggregates: it takes the
*global* minimum band across shards and only rows at that global band form one fairness-key population
(shards with a worse band wait), so strict priority is cluster-wide and fairness is over `fairness_key`,
never over shards. The cache floor is that single global band. A single refiller goroutine plus a
buffered(1), never-closed, non-blockingly-sent `refillTrigger` is the single-slot selection gate
(`MaxClaims` reinterpreted): concurrent requests (worker low-water, timer poll, doorbell) coalesce into at
most one pending scan, and the send can never panic, even during the shutdown drain.

**One pioneer is sufficient; the head-insert is a bridge, not a per-job fast path.** A head-insert is
accepted at most once per band-opening: it lowers `floor` to the pioneer's priority, so every subsequent
arrival at that same band hits `priority >= floor` and is rejected (case 2). That is deliberate and
correct, not a starvation bug. The pioneer's only job is to bridge the single refiller-cycle gap so the
*first* urgent step does not eat a refiller scan of latency. Its `requestRefill` makes the refiller scan
band MIN = the pioneer's priority and `refill()` **wholesale-replace** the cache with the strict, weighted
batch of that band, *evicting* the cached lower-priority candidates (they stay `pending` and are
re-selected only when the band drops back). After that one cycle every further high-priority step is served
correctly and fast by the refiller's strict + weighted selection - no further head-inserts needed. So a
stream of escalated work is handled well: pioneer #1 bridges, the refiller takes over the whole band. A
non-pioneer high-priority step that misses the head-insert (stale `floor` in the pre-refill window) is
**not** stuck behind the low-priority backlog: the refiller selects band MIN, so it is picked up after at
most ~`lowWater` lower-priority pops plus one scan - a bounded fast-path *miss*, never priority starvation.

**Bounded bridge-window leakage and the weighted-fairness bypass are deliberate, bounded, and
self-healing.** Between the pioneer head-insert and the asynchronous `refill()` landing, workers keep
popping the still-cached lower-priority steps. The leak is bounded by ~the worker count, not by the
backlog: a refiller scan is one DB round-trip while a worker that pops a step is then busy executing it for
its full duration (much longer than a scan), so each worker leaks at most ~one lower-priority step before
the replace evicts the rest; the pioneer itself is at the head and is never delayed. Separately, the
head-insert bypasses the weighted Efraimidis-Spirakis fairness for exactly that one pioneer step: it is the
*first* work of a just-opened band on this replica (no established weighted distribution there yet to
violate), it is bounded to one step per escalation by the equal-rejected rule (a burst at the new band
does not multiply it), and the refiller's next batch restores weighted fairness for the band bulk. Both
costs are strictly smaller than the cross-replica fairness softness the design already accepts (N replicas
weighted-sampling independently never sum to an exact global ratio), and both are the deliberate extension
of "throughput trumps exact ordering" to within-band ordering and fairness. Do not "fix" these by flushing,
by per-item priority tracking in the cache, or by re-floor-on-pop: each trades the latency win the
head-insert exists for, adds hot-path state or nondeterminism, and only shaves an already-bounded refiller
cycle off a path the refiller already backstops.

**Liveness guarantee.** A worker requests a refill *after* `processStep` returns - i.e. after the step has
left `pending` (acquired or completed) - not at pop time. This is load-bearing: requesting before the CAS let
the refiller re-select the in-flight step and, under single-slot coalescing, never scan post-completion state,
wedging a single-worker replica with a backlog. Post-completion the next refiller scan always reflects every
freed slot, so a coalesced trigger can never wedge while work remains. The worker also requests at the
low-water mark so draining overlaps refilling on a wide pool. The cache holds 2x the worker count and the
low-water mark is half of that (one worker count): the 2x buffer keeps workers fed across a refiller scan,
and the refiller fills a batch up to the full cache capacity.

`pollPendingSteps` no longer enumerates the backlog onto the queue. It recovers expired-lease steps, detects
orphaned flows, sizes the wake timer to the nearest future `not_before`, and rings the local doorbell each
cycle. If a due-pending backlog exists it caps `nextPoll` at `backlogPollInterval` (1 minute) so an idle
replica that received no doorbell still re-scans - the poll-driven liveness net the old enumerate-and-push
model provided, without per-step queue stuffing. This is a coarse safety net, not the primary wake path: due
work is normally picked up immediately by the completion doorbell, and `nextPoll` is shortened to anything
sooner (a future `not_before`, a valve-window boundary).

The timer waits on **two independent deadlines**, `nextPoll` (above) and `nextProbe`, and wakes on whichever is
sooner. `nextProbe` is the soonest tripped-breaker probe time, owned end to end by the breaker subsystem
(`refreshNextProbeLocked`, recomputed only on the infrequent breaker trip/commit/close transitions and read by
the timer as a lock-free atomic). Keeping it separate from `nextPoll` is load-bearing: an earlier design folded
the breaker probe into `nextPoll`, but `pollPendingSteps` resets `nextPoll` to the backlog cadence every cycle,
which clobbered the probe wake whenever a poll landed as the probe came due - collapsing the breaker's 100ms..1m
exponential schedule to the 1-minute backlog cadence (observed as a ~60s stall before a reachable-again task
recovered). Because the breaker now owns `nextProbe` outright, no `pollPendingSteps` reset can touch it.

The refiller/`MaxClaims` slot and the crash-recovery lease are orthogonal: the slot is an in-memory admission
gate on running the selection SQL; the lease is the DB-side crash-recovery timer on the `running` step.

A **timer loop** (`timerLoop`) runs `pollPendingSteps` on the adaptive interval (nearest future `not_before`,
the soonest tripped-breaker probe, the 1-minute backlog cap, or `maxPollInterval`).

### Query Parallelism

`processStep` is the hot path - it runs for every task in every workflow. To minimize latency when the database is remote, independent queries within `processStep` are executed in parallel using `svc.Parallel`:

- **Claim UPDATE + step SELECT** - the lease-acquiring UPDATE and the step-data SELECT run concurrently. The UPDATE only mutates `status` and `lease_expires`; the SELECT reads stable columns (`step_depth`, `task_name`, `state`, `changes`, `breakpoint_hit`, `attempt`, `lineage_id`, `flow_id`, `time_budget_ms`) that the UPDATE never touches, so they race-read safely. The lease size comes from the foreman's in-memory `TimeBudget` config rather than from the step row, removing the dependency that previously forced a serial pre-SELECT.
- **Flow data** - runs after the parallel claim+SELECT block returns, since it depends on the `flow_id` the step SELECT just produced. Net: two serial DB round-trips on the hot path (claim+step in parallel, then flow), down from three.
- **Fan-in sibling counts** - the unfinished and failed sibling COUNT queries run concurrently
- **Fan-in sibling counts** - the unfinished and failed sibling COUNT queries run concurrently
- **Subgraph status counts** - the active and completed subgraph COUNT queries run concurrently

Outside the hot path, `completeSurgraphFlow` and `surgraphChain` also parallelize independent queries.

**Transaction constraint:** Functions that receive a `sequel.Executor` parameter (which may be a transaction) cannot use `svc.Parallel` because SQL transactions are not safe for concurrent use. This applies to `computeFinalState` and any code running inside `failStep` or `Cancel` transactions.

### Fan-Out and Fan-In

**Static fan-out** occurs when multiple transitions match from a single task. All target tasks execute in parallel, sharing the same `step_depth`. The flow's `step_id` is set to `0` to indicate fan-out.

**Dynamic fan-out** uses `forEach` on a transition to iterate over a state array and spawn one task instance per element. Each instance receives the element under the `as` key. If the array is empty, no tasks are spawned for that transition. When a `forEach` transition is the only outgoing transition from a task, an empty array causes the flow to complete at that point - downstream tasks (including the fan-in target) are never reached.

**Branch state strip on dynamic fan-out.** When the foreman spawns branches for a `forEach` transition, it removes
the source array field from each branch's local `state` (and only the local state - the spawn step's immutable
`state` snapshot still carries it). Without this, an N-element forEach feeding a sub-chain `forEach -> A -> B -> C
-> J` would write N copies of the array into every step row in every branch, blowing storage up by N times the
chain length. With the strip, each branch carries only its single element. The fan-in step rebuilds its own state
from the spawn step's `state + changes`, so the source array reappears in the fan-in step and everything
downstream of it - the absence is local to the cohort, not propagated past the join. The strip is skipped when
`as == forEach` (the user named the alias the same as the source field), so the element write is not clobbered.
Alongside the strip, the foreman injects two read-only fields on each branch's `state`: `<as>Index` (this
element's position in the forEach array) and `<as>Count` (the cohort size), so the branch task can read its
ordinal context without the source array being present.

**Downstream suppression via explicit clear.** A branch that wants to suppress the source array past the fan-in
calls `flow.Set(<forEach>, nil)` (or any value) in its task body. That writes the new value into the branch's
`changes`, the replace reducer at fan-in folds it over the spawn-step base, and the field is absent (or whatever
the branch wrote) past the fan-in. The use case is a forEach over a very large array where downstream tasks only
care about the per-element transformation and not the original source.

**Fan-in strip on dynamic fan-out.** `insertFanInStep` deletes `<as>`, `<as>Index`, and `<as>Count` from the merged
state after the cohort converges. The injected per-branch bookkeeping has no meaning past the fan-in: with the
Replace reducer, one branch's element value and index would otherwise win arbitrarily and ride forward in every
downstream step. The strip is the symmetric counterpart to the source-array strip on the way *down* into branches:
together they keep forEach scratch fully scoped to the per-element subgraph. The names to delete are recovered by
walking the spawn task's outgoing `forEach` transitions in the graph (`tr.As`); static fan-outs have no `as` and
trigger no deletion. Multiple `forEach` transitions from one source (rare) all contribute their `as` names to the
deletion set. A workflow that wants the element value past the fan-in must explicitly forward it under a different
key (e.g. `flow.Set("chosenOpportunity", flow.GetString("opportunity"))`); the per-branch alias is gone.

**Fan-in** is implicit. When the last sibling at a `step_depth` completes, the foreman merges all siblings' changes using reducers and creates the next step(s) within a transaction. The transaction prevents duplicate next steps when multiple workers finish siblings simultaneously.

**Fan-in does not escalate on cancelled or failed siblings.** If a sibling at the current depth is in `failed` or `cancelled` status when fan-in evaluates, the flow is already being driven by another path: a sibling's `failStep` has cascaded the flow to failed, an external `Cancel` has cancelled it, or an `OnError` sibling-cancel has handed the depth off to an error handler. The fan-in worker bails out with `return nil` instead of calling `failStep` on its own step. Calling `failStep` here races with the OnError handler and incorrectly fails an otherwise-recoverable flow: the OnError handler's next step (e.g. an error-routed alternative path) is in flight at depth `N+1` while the fan-in worker is still finishing depth `N`, and a redundant `failStep` from the fan-in worker would mark the parent flow failed before the OnError path ever runs.

**Fan-in merge order and contribution (lineage `SetFanIn` path).** `insertFanInStep` reads cohort members
(`lineage_id = cohortSpawnID`) `ORDER BY fan_out_ordinal, step_id`. `fan_out_ordinal` is stamped on each branch at
fan-out from its position in the spawn loop, which is the `forEach` array index (or static fan-out declaration order),
so `list`/`append`/`sum`/`set` reducers fold in input-array order rather than non-deterministic completion order;
`step_id` only breaks ties. The firing gate is unchanged - fan-in still fires when `cohort_arrivals >= cohort_size`,
which is a counter on the spawn step independent of the merge query, so the merge's status filter cannot deadlock
fan-in. Only members in `completed` status contribute their `changes`; `failed`/`cancelled`/`pending`/
`running` members contribute no state and are skipped. Excluding `cancelled` from the merge matches the long-standing
"cancelled members count as arrivals but contribute no state" intent; before this, the unfiltered merge folded their
empty/partial `changes` in.

**The fan-in does not escalate on failed/cancelled members.** It records a normal `pending` fan-in step regardless of
cohort composition - it never marks the fan-in terminal or cascades `failStep`. A `cancelled` cohort member is the
*expected* OnError sibling-cancel case (one branch errored and routed to its `OnError` handler, which cancelled the
others; the flow must still recover via the handler -> fan-in path and complete). Genuine terminal outcomes are
already handled elsewhere and do not rely on the fan-in: an unhandled task error cascades via `failStep`, the Cancel
API sets the flow terminal directly, and the terminal-flow check in `processStep` catches siblings. An earlier
revision had the fan-in *poison* itself (mark terminal + `failStep`) when any member was failed/cancelled; that
regressed the OnError recovery invariant and made `verify/fanouterrorflow` flaky (it failed iff the erroring branch
lost the race to its still-running siblings), so it was removed. `verify/failedfanoutflow` and
`verify/cancelledfanoutflow` still pass without it, confirming poison was never load-bearing for them.

**Retry rejoins its cohort naturally.** `flow.Retry` rewinds the failed step in place - same `step_id`, same
`lineage_id`, same `fan_out_ordinal`, just `status='pending'` and the prior error/park slot cleared. The merge query
sees one row per branch regardless of how many attempts that row went through, so retry can't double-count.

### Execution-DAG edges (`predecessor_id` / `successor_id`)

`lineage_id` is a cohort-counting device, not a DAG: a `forEach` source applies one `childLineageID` to every
branch, so an entire per-element sub-pipeline (e.g. `forEach -> {A -> B -> C}`) collapses into a single lineage. It
therefore cannot reconstruct the true parent/child structure - which is why history rendering used to draw an
all-to-all bipartite join by `step_depth` and connected every `H` to every `A`/`B`.

`microbus_steps.predecessor_id` and `successor_id` (migration `7.sql`) record the actual execution edges so the DAG
is *recorded*, not *reconstructed*. Every edge lands on at least one endpoint:

- **Linear** `X->Y`: `Y.predecessor_id=X` (at insert) and `X.successor_id=Y` (the post-loop UPDATE in `processStep`).
- **Fan-out** `X->{Yi}`: every `Yi.predecessor_id=X`; `X.successor_id` = the first child only. The full set is
  recovered from the children's `predecessor_id`.
- **Fan-in** `{Yi}->Z`: `Z.predecessor_id` = the last cohort member to finish (the step that triggered fan-in);
  every cohort *exit* step gets `successor_id=Z`. The exit set is `lineage_id == cohortSpawnID AND task_name IN`
  the graph-predecessor tasks of the fan-in (`fanInPredecessorTasks`) - **not** the whole lineage, so `A`/`B` in
  `forEach->{A->B->C}->J` are excluded and only the `C`s point at `J`.
- **Retry / RestartFrom**: rewind the step in place (same row), so `predecessor_id` is preserved and the step stays wired into the DAG.
- **Entry / subgraph-entry steps**: `predecessor_id` defaults to 0 (no in-flow predecessor).

`renderMermaidSteps` ignores `step_depth` and `lineage_id` entirely: it draws the deduped union of
`{predecessor_id -> step}` and `{step -> successor_id}`, which is exact for arbitrary nesting (validated by
`mermaid_test.go` for the per-element pipeline and `forEach`-of-chain shapes). Heads are nodes with no incoming
edge, tails are nodes with no outgoing edge.

**Visual grouping via Mermaid `subgraph` blocks.** Subgraph (child workflow) steps render as a brand-pink
block at 5% fill opacity wrapping the recursively-rendered `SubHistory` — that one is meant to be seen. Fan-out
cohorts (2+ steps sharing one `predecessor_id`) also get a `subgraph` wrapper, but invisible: empty label, no
fill, no stroke. The wrapper is there purely as a Mermaid layout container — without it the cohort siblings
stretch across the full diagram width because Mermaid has nothing telling it they belong together; with the
container they cluster near their parent. Either kind of block is purely structural — edges go between actual
task node IDs, never to the block ID. A subgraph step's inner heads become the targets of any incoming edge,
and its inner tails become the sources of any outgoing edge, so a cross-boundary edge naturally expands N x M
across multi-head/multi-tail sub-flows. The earlier `_enter` / `_exit` dot markers are gone.

`computeFinalState` also reads the DAG, not `step_depth`. The flow's terminal state is the merge of the tail
steps - completed steps with `successor_id = 0` (`mergeTerminalSteps`). The earlier `MAX(step_depth)` heuristic
was wrong for any graph where an intra-thread `flow.Goto` self-loop sits inside a fan-out: each loop iteration
pushes `step_depth + 1`, so the looping branch can outrun the fan-in/terminal step in depth. When `normalC`
(shallow branch) is the last cohort member to arrive, the fan-in `taskD` is created at `normalC.step_depth + 1`,
which is *less* than the dangling final loop iteration's depth, so `MAX(step_depth)` selected the loop step
(no `finalResult`) and the empty step state was returned. The bug was order-dependent (only when the
shallow branch lost the fan-in race), which is why `verify/intrathreadgotoflow` failed intermittently. The
tail-step merge is depth-agnostic: the loop iterations carry `successor_id = taskD` (set by the fan-in cohort-exit
UPDATE), so only the real terminal step qualifies. `mergeTerminalSteps` is the *only* final-state path - there is no
`step_depth` fallback. It is two-tier and depth-free: the completed tail (`successor_id = 0 AND status = completed`)
for a normally finishing flow; if none, the non-completed tail (`successor_id = 0`, any status) for a flow
force-terminated by `Cancel`/`failStep` before any step completed - those are the in-flight/entry steps whose
immutable `state` snapshot is the flow's last-known input (their `changes` are empty), so merging them reproduces
the snapshot the old `MAX(step_depth)` merge produced, without consulting `step_depth`. An empty map is returned for
a flow with no steps. Flows predating `7.sql` (no `predecessor_id`/`successor_id`) are not supported - migrations
run at startup, so every live flow has the DAG columns.

### Time Budgets

Each step has a `time_budget_ms` that controls the `pub.Timeout` on the task execution HTTP call. It is the
foreman's single `TimeBudget` config (default 2m), applied uniformly to every step - `taskTimeBudget()` no
longer consults the graph. The graph carries no timing: per-task budgets are now declared by the task endpoint
itself via the framework's `sub.TimeBudget` option, and the connector shortens that handler's inbound context
deadline to `min(caller budget, declared budget)`. So `TimeBudget` is the *ceiling* the foreman imposes on the
dispatch call, and `sub.TimeBudget` is the *shorter* bound a task may impose on itself - exactly the
ingress-proxy request-timeout-ceiling shape, generalized to every task endpoint. `graph.SetTimeBudget` was
deleted outright; the workflow graph no longer encodes any duration.

The worker lease duration is the foreman's current `TimeBudget` config + `leaseMargin` (30s), read from
in-memory config rather than from the step row. This lets `processStep` claim the step without an upfront
SELECT to read the step's `time_budget_ms`: the dispatch-timeout value is read in the parallel block below
the claim UPDATE, but the lease size is set from the live config. The lease always outlasts execution,
preventing premature recovery by `pollPendingSteps`. Accepted trade-off: because the lease is sized to the
ceiling rather than per task, a *crashed* worker that was running a task with a tight `sub.TimeBudget` is
recovered no sooner than `ceiling + leaseMargin` instead of `tightBudget + leaseMargin`. The common case is
unaffected - a hung (not crashed) task is cancelled at its declared bound by the connector, its outbound
call returns, and the foreman acts immediately; only the rare worker-crash path pays the slower recovery,
which is acceptable for the simplification of a single uniform lease. A separate trade-off applies if the
operator decreases `TimeBudget` mid-flight below the largest in-flight step's stored `time_budget_ms`: the
new (shorter) lease may expire before the (longer) dispatch completes, causing `pollPendingSteps` to
re-dispatch the step. Steps that are not idempotent under re-dispatch should not coexist with such
config changes - the same constraint that already applies to any other lease-expiry recovery path.

`sub.TimeBudget` is endpoint self-protection, not upstream propagation: the foreman never learns a task's
declared budget, and nothing on the wire carries it back. A responsive-but-slow task is cancelled at its
declared bound, returns promptly, and the foreman acts at once - so the dispatch call effectively completes
in `declared budget + one network hop`, far inside the foreman's ceiling. A task that never responds at all
(a crashed or wedged replica) is bounded solely by the foreman's `TimeBudget` ceiling on the `pub.Timeout`
of the dispatch call, which is exactly the hung-downstream behavior that predates `sub.TimeBudget`. The
option therefore adds no new dispatch-latency hazard; it only lets an endpoint bound its own handler.

### Flow lifetime is workflow-author's responsibility

The framework does not impose or enforce a flow-level deadline. Picking a max-lifetime value that
fits both a 1-hour batch and a 30-day human-approval workflow is impossible, and a knob whose
default value is "no deadline" is just surface area without a customer. Workflows that need a
lifetime bound implement it in workflow-author space: a guard task that reads `flow.CreatedAt()`
and returns `errors.New("…", 408)` when too much time has elapsed; a `flow.Retry` loop that
exhausts after a chosen bound; an `OnTimeout` transition to a "give up" handler; or an external
caller scheduling a `Cancel` at the cutoff. All four are strictly more flexible than a hard
framework-level "deadline exceeded" failStep.

`Flow.CreatedAt()` and `Flow.UpdatedAt()` are populated by the orchestrator on every dispatch, so
the elapsed-time guard pattern is one method call away inside any task.

### State Model

Each step has three JSON columns: `state` (input snapshot), `changes` (output delta produced by task execution), and `interrupt_payload` (data from `flow.Interrupt(payload)`). The `state` column is set at step creation and is normally immutable. The `changes` column is written after task execution.

The next step's `state` is computed as `merge(currentState, changes)` - the merged result overwrites matching fields. This immutability model enables checkpointing, restart, and recovery: the foreman can always reconstruct a step's output by reading its `state` + `changes`.

**State mutation on retry:** On `flow.Retry()`, the foreman merges `state + changes` back into the `state` column so the task sees its own prior output on the next attempt; `changes` is preserved as-is. `Resume` no longer mutates `state`: it writes the caller's data to the leaf step's `resume_data` column, which `flow.Interrupt` returns on re-dispatch (the task no longer reads resume data by field name from `state`).

**Reducer delta convention:** Tasks writing to reducer-managed fields (append, add, union, merge) must set only the **delta**, not the full accumulated value. For example, if `messages` is wired to the append reducer via `graph.SetReducer("messages", workflow.ReducerAppend)`, the task should set `flow.Set("messages", []string{newMessage})` - not the entire message history. Violating this causes duplicates during fan-in merge.

**forEach element injection:** When a `forEach` transition spawns task instances, the current element is injected into `state` only (under the `as` key), not into `changes`. This makes the element available to the task but prevents it from participating in fan-in merge.

### Task-Initiated Control Signals

Tasks can signal the foreman via control methods on the `Flow` carrier. These are distinct from the API-level operations:

- **`flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay) bool`** - Requests the foreman to re-execute this task with exponential backoff. Returns `true` while attempts remain (the task should return `nil`), `false` once `maxAttempts` is reached (the task should return its error). Call it inside an error branch. Pass zero delays for an immediate retry and `math.MaxInt32` for unlimited attempts. The step row is reused (not a new row). The foreman tracks the attempt count in the step's `attempt` column and computes the delay as `min(initialDelay * multiplier^attempt, maxDelay)`. Changes from this execution are preserved, and the foreman merges `state + changes` back into the step's `state` column so the task sees its own prior output on the next attempt via `ParseState`/`GetInt`/etc. Both `flow.Retry` (task-initiated) and the operator-facing `RestartFrom` rewind a step row in place; `RestartFrom` additionally sweeps the step's downstream subtree and merges optional state overrides, and is meant for operator-driven recovery after a flow has already terminated. **A retry clears the step's park slot** (`interrupt_done`/`subgraph_done` reset to `0`, `resume_data`/`subgraph_result` to `'{}'`, `subgraph_error` to `''`), so a retry after a resolved `flow.Subgraph` re-runs the child and a retry after a resolved `flow.Interrupt` re-interrupts. `flow.Retry` is the **single** retry primitive - it carries no condition, so the task writes the retryable condition explicitly in the surrounding `if` (retry-on-any-error is usually wrong; validation/4xx/business rejections should not be retried). To retry only on a timeout, gate on `errors.StatusCode(err) == http.StatusRequestTimeout` - both a foreman-side `pub.Timeout` expiry and a subscriber-side handler timeout surface as 408.

**No jitter on retry backoff:** The foreman does not add jitter to the computed backoff delay. The worker pool (`Workers` config, default 64) already throttles concurrency per replica - even if multiple retries fire simultaneously, they queue up in the worker pool rather than overwhelming downstream resources. Adding jitter would increase latency for no throughput benefit in this architecture.
- **`flow.Sleep(duration)`** - The task tells the foreman to delay the next step's execution. Sets the next step's `not_before` timestamp. Useful for rate-limit delays. The timer loop adapts its poll interval to wake up when the sleep expires. Note: sleep delays the *transition to the next step*, not the current step's re-execution. In fan-out, only the last sibling's sleep affects the fan-in point.
- **`flow.Goto(target)`** - Overrides transition routing. The foreman skips normal condition evaluation and follows the `withGoto` transition to the specified target, if one exists in the graph. Goto transitions are never taken during normal evaluation - only when explicitly requested.
- **`flow.Interrupt(payload)`** - Pauses execution and parks the flow. The payload is stored in the step's `interrupt_payload` column and propagated up the surgraph chain. The task should return normally after calling Interrupt. The foreman sets the flow to `interrupted` and fires `OnFlowStopped` on the root flow's `notify_hostname`.

**Single-park guard (server side).** A step parks at most once - interrupt XOR subgraph, never both and never the
other kind on re-entry. `processStep` enforces this after the task returns: the existing competing-signals check
fails the step if more than one control signal (interrupt, subgraph, retry, goto) is set in one dispatch, and a
second check fails the step when the returned flow arms a park (`InterruptRequested`/`SubgraphRequested`) while the
step row's materialized `interrupt_done`/`subgraph_done` shows the *other* kind already resolved. The `workflow`
package's parkers already reject a conflicting second park at the call site (returning an error in their `err`
return), so a well-behaved task never reaches this guard; it is the trust boundary for an untrusted returned flow
(a task that bypasses the parker methods could present both signals). A same-kind re-arm cannot occur because a
resolved parker returns its cached value without re-arming.

### Transition Evaluation

Transitions are evaluated after a task completes successfully. The rules are:

1. If the task called `flow.Goto(target)`, only `withGoto` transitions matching that target are taken
2. Otherwise, all non-goto, non-error transitions from the current task are evaluated: transitions without `when` are always taken; transitions with `when` are taken if the expression matches against the merged state
3. `forEach` transitions iterate over a state array and spawn one task per element. `forEach` cannot be combined with `withGoto`
4. When multiple transitions match, all are taken in parallel (fan-out)
5. When no transitions match, the flow completes

**Error transitions** are evaluated when a task returns an error. Only `onError` transitions from the failed task are considered. If a matching error transition exists, the error is serialized as a `TracedError` (with stack trace and properties) into the state field `onErr`, and the error handler task is created as the next step. The failed step is marked `completed` with its changes preserved. If the task was in a fan-out, all siblings are cancelled. If no error transition matches, the flow fails as usual via `failStep`. Error transitions can have `when` conditions for conditional error routing but cannot be combined with `forEach` or `withGoto`.

**Fan-out sibling constraint:** `Graph.Validate()` enforces that fan-out siblings (tasks spawned in parallel from the same source) must have the same set of non-goto, non-error outgoing transition targets. This is because the foreman evaluates outgoing transitions from only the last sibling to complete - if siblings had different outgoing transitions, the result would depend on which finished last.

### Graph Resolution

At `Create` time, `fetchGraph` does a direct `GET` to the workflow's endpoint URL to fetch the graph definition. The graph JSON is frozen at creation time and stored with the flow.

### State Across Subgraphs

**Subgraph is a function call.** Only the explicit `input` map passed to `flow.Subgraph(url, input)` crosses
into the child as its initial state; only the explicit `out` map (the child's `final_state`) crosses back to
the calling task. The parent's state and accumulated changes do NOT auto-cross in either direction.

**Into the child:** `execution.go`'s `SubgraphRequested` branch passes `subgraphInput` directly to
`createSubgraphFlow` as the child flow's initial state (with `nil` normalized to an empty `{}` so an
unargumented call is well-defined). No `MergeState` with parent state. If the caller wants the parent's full
state to cross, the calling task passes `flow.Snapshot()` (or a derived map) as `input` - that's the explicit
opt-in to inheritance.

**Back to the parent:** `completeSurgraphFlow` writes the child's `final_state` to the surgraph step's
`subgraph_result` column and sets `subgraph_done=1`, then re-dispatches the parent task. The task's
`flow.Subgraph` call returns that `final_state` as `out` (yield=false), and the task adopts the fields it
wants. The child output is **not** merged into the parent's `changes` - the calling task scopes it explicitly
(e.g. returns the wanted fields as its own typed outputs).

The function-call shape eliminates the "what does the child see?" surprise surface that the inheriting model
carried: the answer is exactly what's in `input`. State hygiene is now a matter of choosing what to put in
`input` and what to read from `out`, not of building adapter tasks around the subgraph node.

### Surgraph Step Identification

Each subgraph flow's `microbus_flows` row stores `surgraph_flow_id`, `surgraph_step_depth`, *and* `surgraph_step_id` - the primary key of the parked surgraph step it belongs to. `completeSurgraphFlow` looks the surgraph step up by primary key, so it can never match a sibling at the same `(flow_id, step_depth)`.

This matters in two scenarios that broke earlier (when the lookup was by `flow_id + step_depth + status='running'` only):

1. **Fan-in race:** while `completeSurgraphFlow` is running, a non-subgraph sibling at the same step_depth might be momentarily in `running` status (e.g. mid-execution). The legacy lookup could match that sibling and corrupt its row.
2. **Parallel subgraphs at the same step_depth:** if a fan-out produces multiple subgraph siblings, every parked surgraph step at the depth carries `parked=1` so a status-based filter alone can't disambiguate them.

The `surgraph_step_id` PK lookup eliminates both cases. Migration `3.sql` adds the column. (A pre-`11.sql` lease-threshold fallback for legacy rows was removed once parked semantics took over.)

When parallel subgraph caller tasks fan out at the same step_depth, each parks its own `flow.Subgraph` step at `parked=1`; the `surgraph_step_id` PK scoping is what keeps each child flow bound to the step that launched it, so `completeSurgraphFlow` re-dispatches the correct parent and one sibling's completion never resolves another's park.

### Interrupt/Resume Propagation Across Subgraphs

**Interrupt propagation (up):** When a step inside a subgraph flow is interrupted (breakpoint or `flow.Interrupt()`), the foreman uses `surgraphChain` to walk from the subgraph flow up to the root surgraph, collecting all flow IDs and parked surgraph step IDs. It then interrupts all flows and steps in the chain with bulk `UPDATE ... WHERE flow_id IN (...)` and `UPDATE ... WHERE step_id IN (...)` statements. This ensures the caller awaiting the top-level flow ID sees the `interrupted` status.

**Resume propagation (both directions):** When `Resume` is called on any flow in the chain, the foreman uses `surgraphChain` to walk up and `interruptedSubgraphChain` to walk down. It re-parks intermediate surgraph steps (restore to `status='running', parked=1`), records resume data on the leaf step's `resume_data` column (returned to the task via `flow.Interrupt` on re-dispatch, not merged into `state`), resets the leaf to `pending`, transitions all flows in the chain to `running`, and enqueues the leaf step. All writes are in a single transaction.

**Fan-out interaction:** During fan-out, one sibling may interrupt while others continue running. The flow is marked `interrupted` by the first sibling, but other siblings run to completion. `Resume` handles one interrupted sibling at a time; the flow only transitions to `running` when no more interrupted steps remain at any level in the chain.

### Actor Identity Propagation

The actor's JWT claims are captured at `Create` time and stored in the flow's `actor_claims` column. Before each task execution, the foreman mints a fresh access token from these claims and attaches it to the request via `pub.Token`. This ensures tasks run with the original caller's identity even when executed long after the flow was created.

### Await

`Await` blocks until a flow stops (i.e. is no longer `created`, `pending`, or `running`). It returns on `completed`, `failed`, `cancelled`, or `interrupted`. It registers a buffered channel in the `waiters` map, then loops: check `State`, return if stopped, otherwise `select` on the channel or context cancellation. Non-terminal notifications (e.g. `running` from `Start`) cause the loop to re-check `State` rather than returning early. This is important because `NotifyStatusChange` fires for all status transitions, not just terminal ones - without the loop, a `running` notification could consume the channel's buffer and cause `Await` to return prematurely with an empty status.

### SQLite Testing Support

When `SQLDataSourceName` is empty in `TESTING` or `LOCAL` deployment, the foreman falls back to a SQLite DSN
(`file:shard_%d`). In `TESTING` the shard DSN is routed through `sequel.CreateTestingDatabase` to materialize a
per-test database; in `LOCAL` the file is opened directly so the dev loop keeps its data across restarts. Both
paths land on SQLite via `sequel.Open`. Key differences from server-based databases:

- **Write-first transactions** - `advanceFlow` uses an `UPDATE` as the first operation in its transaction to immediately acquire a write lock. On MySQL/Postgres, this serializes concurrent workers (equivalent to `SELECT ... FOR UPDATE`). On SQLite with `cache=shared`, this prevents deadlocks: deferred transactions that start with reads both acquire SHARED locks, and neither can upgrade to write when the other holds a read lock. Starting with a write acquires the lock immediately, causing the second transaction to block until the first commits.
- **Busy timeout** - `sequel` (since v1.5.7) automatically applies `_pragma=busy_timeout(1000)` to SQLite DSNs that don't already set one. Without this, concurrent workers hitting a write lock immediately fail with `SQLITE_BUSY`; with it, they wait up to 1 second for the lock. This is essential for the foreman because four worker goroutines routinely write to `microbus_steps` in parallel during fan-out.
- **Lock contention recovery** - `processStep` defers a check on its return value: if the error is a database
  lock contention (`sequel.IsLockContentionError`), the deferred handler first resets the step it had leased
  (`status='running'` → `pending`, `lease_expires=NOW_UTC()`), then calls `shortenNextPoll(time.Now())` to wake
  `pollPendingSteps` immediately. Both halves are load-bearing: `pollPendingSteps` only recovers running steps
  whose lease has *already* expired, and a freshly leased step holds its `time_budget+leaseMargin` lease (minutes,
  e.g. `TimeBudget` 2m + `leaseMargin` 30s = 2.5m). Without the lease reset, the immediate poll finds
  nothing to do and the step - and the whole flow's fan-in, if the step sat inside a fan-out branch - stalls until
  the lease lapses on its own. The reset is guarded by `WHERE status='running'`: a no-op if contention struck
  before the lease was acquired (still `pending`) or after the step already completed (a later statement failed),
  so only the leased-and-uncommitted case is rewound. Re-execution after this rewind is the same idempotency
  contract the lease-expiry recovery path already imposes - this change only shortens the latency from minutes to
  the next poll, it does not introduce a new re-run hazard.
- **Migration scripts** - all `.sql` files in `resources/sql/` include `-- DRIVER: sqlite` variants.

### MySQL Column Defaults

In the `-- DRIVER: mysql` schema sections, `TEXT`/`BLOB`/`JSON` columns cannot take a bare literal `DEFAULT`. MySQL
rejects `DEFAULT '{}'` on these types with error 1101; the value must be written as a parenthesized expression default,
`DEFAULT ('{}')` (supported since MySQL 8.0.13). The same applies to function defaults other than `CURRENT_TIMESTAMP`,
which is why the `NOW_UTC()` macro must also expand parenthesized. `VARCHAR`/`CHAR` columns are unaffected and keep bare
literal defaults. The Postgres, SQL Server, and SQLite sections all permit bare literal defaults on their text/JSON
types, so this constraint is MySQL-only. When editing the schema files, mirror the existing parenthesized form on every
MySQL `TEXT`/`JSON` column or fresh MySQL deployments will fail to migrate.

### Database Choice and Configuration

The foreman speaks four SQL dialects via the `sequel` package: SQLite, MySQL/MariaDB, PostgreSQL, and SQL Server. All
four pass the verify suite, but they behave very differently under the foreman's concurrent INSERT/UPDATE load. Pick
the engine by deployment shape, not by familiarity.

**PostgreSQL — recommended for production.** MVCC means concurrent INSERTs do not lock each other on secondary
indexes; there are no gap locks at the default `READ COMMITTED` isolation; and the foreman's fan-out/fan-in pattern
(many parallel INSERTs against the same shard's `microbus_steps`) runs without deadlocks at any concurrency the
workers themselves can produce. Use Postgres 13+ for `JSONB` and the framework's partial indexes
(`idx_microbus_steps_status`, `idx_microbus_steps_selection`, etc.). No special tuning required for correctness; for
throughput, raise `max_connections` to at least `(NumShards * MaxOpenConnsPerShard)` (each foreman replica multiplies
its open connections by the shard count) and `shared_buffers` to about 25% of host RAM.

**MySQL / MariaDB — supported, but expect tuning.** InnoDB at the default `REPEATABLE READ` isolation takes next-key
locks (row + gap) on every secondary-index touch. Two concurrent flow creations on the same shard can deadlock when
their INSERTs lock overlapping ranges of `idx_microbus_steps_selection (status, priority, fairness_key)` or
`idx_microbus_steps_saturation (status, task_name)` in different orders; InnoDB then aborts one with
`Error 1213 (40001) Deadlock found when trying to get lock`. This is normal InnoDB behavior, not a foreman bug -
`createWithGraph` retries on `sequel.IsLockContentionError` so the deadlock is invisible to callers most of the time,
but a sustained deadlock rate degrades throughput and inflates p99 latency. To minimize it:

- **`transaction-isolation = READ-COMMITTED`** in `my.cnf` (`[mysqld]` section). Drops gap locks entirely; the only
  cost is that range queries no longer see a frozen snapshot, which the foreman does not rely on. This single setting
  is the largest deadlock-rate reduction available and is the recommended default for any MariaDB/MySQL deployment
  hosting foreman.
- **`innodb_autoinc_lock_mode = 2`** (interleaved). Removes the table-level AUTO-INC lock on plain INSERTs. Safe
  with `binlog_format = ROW` (the only sane modern setting). If you must keep statement-based replication,
  leave at the default `1` and accept the auto-inc serialization.
- **`innodb_lock_wait_timeout`** at 5-10s (default 50s). The foreman's lock-contention retry path needs the engine
  to give up promptly; a long timeout turns a transient contention into a stall.
- **`innodb_deadlock_detect = ON`** (the default). Do not disable - deadlocks are how InnoDB recovers from the lock
  cycles the foreman's workload creates.
- **Per-shard databases.** `SQLDataSourceName` must contain `%d` when `NumShards > 1` (e.g. `microbus_%d`), and
  every `microbus_N` database must exist before foreman startup. The framework creates the schema via migrations
  but does not `CREATE DATABASE`. Sharding raises aggregate throughput by partitioning the auto-inc and secondary
  index contention across N independent tablespaces.
- **MariaDB version.** 10.5+ is required for `JSON` column type compatibility with the schema in `1.sql`. 10.6+
  is recommended; older versions accept the DDL but have less mature deadlock detection.

**SQL Server.** Snapshot isolation (`READ_COMMITTED_SNAPSHOT ON` at the database level) gives Postgres-like
non-blocking reads and eliminates most deadlock risk. Enable it once on each shard database:
`ALTER DATABASE microbus_N SET READ_COMMITTED_SNAPSHOT ON;`. Without this, SQL Server's default pessimistic locking
behaves like MySQL's REPEATABLE READ for INSERT contention. No other tuning is mandatory.

**SQLite — testing and single-instance dev only.** Single-writer at the database level means deadlocks are
structurally impossible (the engine serializes all writes), but throughput tops out around one transaction at a
time. `sequel.OpenTesting` is used automatically when `Deployment == TESTING` and `SQLDataSourceName` is empty;
the DSN is `file:shard_%d` so each shard is a separate in-memory database. The `_pragma=busy_timeout(1000)` that
sequel injects is what keeps the four foreman workers from immediately failing on `SQLITE_BUSY` during fan-out;
do not remove it. Do not run SQLite in production - the single-writer ceiling will be the bottleneck before
anything else.

**Sharding guidance.** `NumShards` partitions flows across separate databases (or separate schemas on the same
engine). Shard count should equal or exceed the steady-state number of concurrent flow-creating threads divided by
the per-shard write contention the engine tolerates. Rough sizing:

| Engine | Concurrent INSERT/sec per shard before contention | Suggested NumShards |
|---|---|---|
| PostgreSQL | 1000+ | 1-4 |
| SQL Server (RCSI) | 500-1000 | 2-4 |
| MariaDB/MySQL (RC) | 200-500 | 4-8 |
| MariaDB/MySQL (RR) | 50-200 | 8-16 |

`NumShards` can grow at runtime via `OnChangedNumShards`; it cannot shrink (old shards drain naturally as their
flows complete). Increasing it adds new shards immediately, but existing flows stay on their original shard.

**Connection pool sizing.** The `sequel` package opens a connection pool per shard. The foreman's hot path uses
~3-5 connections per active step (claim UPDATE + step SELECT + flow SELECT + transition INSERTs). Size each
shard's `MaxOpenConns` to roughly `Workers * 4 / NumShards`, with a floor of 10. Undersized pools cause workers
to serialize on `db.BeginTx` waits, which looks like throttling but is just connection starvation.

### Flow Scheduling (priority / fairness)

`8.sql` adds `priority`, `fairness_key`, `fairness_weight` to **both** `microbus_flows` (authoritative) and
`microbus_steps` (denormalized). The flow row is the source of truth; the step copy exists so the two-level
selection query never has to join `microbus_flows` on the hot path - the same split already used for
`time_budget_ms`/`actor_claims`.

`resolveFlowOptions` resolves a caller's `*workflow.FlowOptions` against the foreman defaults: priority falls
back to the `DefaultPriority` config, the fairness key falls back to the `tid`/`tenant` actor claim then the
`""` bucket, and the weight falls back to `1`. The three values are immutable for the life of a flow (no
`Reprioritize` in v1; switching a flow's fairness key mid-run would be a self-promotion abuse vector).
`Create`/`CreateTask` resolve from options/claim; `Continue` and `createSubgraphFlow` **inherit** the
parent/thread flow's three values (read from its row) rather than re-resolving, so a high-priority parent never
silently spawns default-priority descendants.

Propagation onto step rows takes one of two shapes, both correct because the flow's values are immutable and
authoritative:

- Where the resolved values are already in hand (the entry step in `createWithGraph`), they are written as
  literal bind parameters. `Restart`/`RestartFrom` rewind their target step in place via UPDATE, so the row's
  immutable priority/fairness values are already present and need no re-propagation.
- In the deep `processStep` paths (next-step fan-out and the two fan-in inserts), the values are read once
  per step execution as part of the parallel flow-row SELECT at the top of `processStep`, then threaded
  through the call chain into the INSERTs as literal bind parameters. The previous design used scalar
  subqueries `(SELECT priority FROM microbus_flows WHERE flow_id=?)` inline in each INSERT to keep the call
  chain narrow, but at N-way fan-out that meant 3N PK lookups per step. Threading the values through pays
  three extra columns on the flow SELECT (which already runs in parallel with the step SELECT) and zero
  per-child overhead.

The scheduler *reads* `priority` and `fairness_key`/`fairness_weight` via the two-level selection in the
refiller (see "Execution Model"), backed by `idx_microbus_steps_selection`. `fairness_weight` is read only
from each key's oldest candidate step. `idx_microbus_steps_saturation` is read by the adaptive backpressure
controller (`limiting.go`, see "Adaptive Per-Task Concurrency" below) to compute the cross-shard
`SUM(running)` per task that gates the saturated-set exclusion in `runRefill`.

#### Why the scheduling design is shaped this way

These are the load-bearing design decisions behind the mechanics above; the *what* is in "Execution Model"
and the resolution above, this is the *why*.

- **Priority is a property of the flow, not the task or the workflow type.** A per-task priority is
  incoherent: step order *within* one flow is dictated by the graph, not by relative urgency. Priority only
  arbitrates *between* flows competing for workers, so it is resolved once at `Create` and is immutable
  (`workflow.FlowOptions` is flow-level for the same reason - see `workflow/CLAUDE.md`).
- **Fairness weight is denormalized at `Create`, never resolved on the selection path.** A resolver hook in
  the claimer would put synchronous I/O on the hot selection critical section. Instead the weight rides on
  the step row; when a key's steps carry inconsistent weights the oldest candidate step's weight is used
  (single-valued selection with no side state), and keeping weights consistent for a key is the caller's
  responsibility - the framework does not reconcile them.
- **No per-tenant / per-key metric labels.** Production fairness observability is aggregate-only: the
  per-priority backlog/age gauges plus a single distinct-fairness-key count. Per-key labels would be
  unbounded-cardinality (one series per tenant). Empirical weight-share correctness is a test concern (the
  `fairnessflow` / `shardedflow` fixtures assert the ratio), deliberately not a production metric.
- **`Workers` is a generous static cap, not the backpressure mechanism.** A worker blocked on an outbound
  task call is just a goroutine stack plus a socket, so over-provisioning the pool is cheap; throttling a
  pressured downstream is the adaptive per-task dispatch-rate control in `backpressure.go`, not a function
  of pool size (see "Adaptive Per-Task Concurrency" below).
- **`SQLConnectionPool` defaults to 8 per shard, with `MaxIdle == MaxOpen`.** Workers spend most of their
  time waiting on the outbound task HTTP call, not holding a SQL connection - the DB-only segments of
  `processStep` are short. So the per-shard pool needs only a small absolute number of connections to keep
  the workers fed without queuing on the pool, even at higher worker counts. `MaxIdle == MaxOpen` matters
  more than the absolute number: Go's `database/sql` closes connections back down to MaxIdle when load
  drops, and under bursty load the close/reopen churn (TCP+TLS+auth per cycle) dominates over the time
  spent actually executing queries. Empirically on the creditflow parallel benchmark, pool=4 hits ~67% CPU
  utilization, pool=8 hits ~85%, pool=16 hits ~92%, and pool=32 regresses (added pool-mutex contention
  with no usable extra concurrency); the curve is also nearly flat between Workers=32 and Workers=64 at
  pool=8, which is why the default is a fixed 8 rather than proportional to `Workers`. The override is
  effective in practice because the foreman usually owns its own DSN (refCount stays at 1, so sequel's
  sqrt-based auto-sizing does not re-fire after init), but if another microservice in the same bundle
  opens the same DSN as foreman, sequel's `adjustConnectionLimits` runs again and silently undoes the
  override - a fragility worth knowing about. Operators with a different workload mix (longer DB-hold,
  larger shards, more aggressive throughput target) should tune the config explicitly.
- **Completion writes are deliberately *not* gated by the refiller/`MaxClaims` slot.** That slot bounds
  selection only; finishing in-flight work must outrank starting new work, so the post-execution advance
  (persist changes, transitions, fan-in) is never serialized behind selection. The real contention axis is
  same-flow fan-in, already serialized by the fan-in transaction plus the lock-contention rewind path. No
  explicit completion bound is needed - lock contention on the completion write is essentially a SQLite-only
  phenomenon (MySQL/Postgres transactions are short and steps-first-then-flow lock-ordered), and the rewind
  path in `processStep`'s defer already recovers it without an admission gate.

### Adaptive Per-Task Concurrency

A soft, cluster-wide cap on per-task **dispatch rate** (ops/sec), with the rate discovered from
observed backpressure rather than configured. Lives in `backpressure.go`; the `runRefill`
integration sits in `scheduling.go`; the 429 ingestion in the dispatch error path of
`execution.go`. The controller controls *rate*; concurrent in-flight count is incidental (the
worker pool sets a separate hard ceiling, and concurrency for a given rate is governed by
Little's Law: `c = rate * latency`). The section title says "concurrency" for historical
continuity - the original design controlled concurrent count; the unit is ops/sec everywhere
below.

**Why a controller and not a config.** A static per-task cap is wrong the moment the downstream
autoscales: too low and the cluster underutilizes; too high and the cluster overwhelms. An operator
who *does* know the right number per task has no way to keep it current. `MaxTaskConcurrency` and the
related `sub.MaxConcurrency` lever were both rejected for this reason - a hardcoded value defeats the
controller it would feed (the downstream would emit 429 at exactly that count forever, so the
controller could never discover that capacity grew). A wrapper microservice that wants to self-limit
can do so internally using its own Microbus config (which is runtime-tunable), emitting 429 above its
threshold; from the foreman's side this looks like any other organic 429.

**Why rate and not concurrent count.** Most real-world 429-emitting backends are rate-bounded
(third-party APIs publishing "100 req/sec per key", etc.), not concurrency-bounded. A concurrent-count
controller has a structural floor problem with short tasks against tight rate budgets: at rate budget
`R` req/sec and dispatch latency `L`, the matching concurrent cap is `c = R*L`. For `R=10`, `L=10ms`,
`c=0.1` - below the controller's `minLimit=1` floor. The controller stays pinned at 1 concurrent and
issues 100 req/sec, ten times the budget. Switching to a rate controller pushes the floor into
*ops/sec* units, which matches the typical lower bound on real APIs (1 RPS) gracefully.

The symmetric failure mode of a rate controller is **variable-latency concurrency-bounded backends**:
a DB pool of 10 sees its task latency double; the rate that fit yesterday now over-saturates
concurrency. The controller still recovers via 429 feedback (re-cuts the rate to compensate), but its
model is wrong-shaped for that case. We accept this trade as a better default.

**The throttle library does the per-call gating.** Each task has its own
`*github.com/microbus-io/throttle.Throttle` (sliding-window counter, 1s window, ~64 bytes per task).
The throttle has two jobs: count emitted dispatches so a 429 can be anchored to the actual emission
rate, and gate further dispatches when the rate exceeds the controller's current opinion. The
throttle is created lazily at first dispatch (`valveCommit`); the gating limit is `SetLimit`ed lazily
at each peek/commit from the CUBIC curve's current value. Allows are at commit time; peeks gate the
filter pass in `runRefill`. With the throttle owning admission, the previous design's cross-shard
`countTasks` is off the hot path; it now feeds only the `TaskConcurrencyRunning` observable gauge.

**Additive-decrease per 429, re-anchored at burst start.** `valveRegulate` decrements `wCong` by
exactly 1 on each observed 429. A burst of N 429s in tight succession compounds linearly to a cut
of N - the exact overage the downstream just rejected. The throttle's `Peek().observed` is read at
each call, but only the *first* 429 of a fresh burst (tCong older than one throttle window) uses
it to re-anchor wCong upward to the actual emission rate; subsequent 429s in the same burst just
decrement. The re-anchor matters because the CUBIC curve has been growing the effective rate above
the last cut's `wCong` while the task was running cleanly - a 429 at the recovered rate would
otherwise cut from a stale-low wCong by 1, barely a cut.

**The increase is a pure function, not an action.** N replicas running independent additive-
increase loops over one downstream do not sum to the true limit; they oscillate. Electing one
writer is its own coordination problem (queue groups dedupe consumers per message, not the ticker
that owns the increase). Resolution: make the increase a pure function of `(wCong, tCong)` and
wall-clock. Every replica computes the same value from the same anchor; "doing the increase" is
not an action anyone performs.

**The convergent register.** The only shared mutable state is `(wCong, tCong)` per task, gossiped
over `SyncValve`. The merge is "latest `tCong` wins, smaller `wCong` on tie" - associative,
commutative, idempotent, so it tolerates reorder, duplication, and loss. A missed cut self-heals:
the over-admitting replica eats its own 429 and re-cuts. No periodic `Reconcile` broadcast in v1;
add only if logs show staleness is a problem.

**Sender-stamps `tCong`, not receiver.** The whole design rests on every replica computing the same
`recover(wCong, tCong, now)`. Receiver-local stamping would make each replica's curve advance by a
different `now - tCong`, defeating convergence. Cross-replica clock skew on a single foreman cluster
(typically same datacenter, NTP-synced) is milliseconds; the curve operates over seconds. The skew is
negligible relative to the curve.

**Cross-task priority inversion is accepted.** When a high-priority flow's next task is rate-limited,
that flow is delayed (its step stays pending) while lower-priority flows whose next task is
unconstrained proceed. The `runRefill` multi-band loop walks past a wholly-saturated band rather than
idling the cluster. Unbounded priority would defeat the purpose: an urgent flow cannot conjure
downstream capacity. The flow is delayed, not failed - the 429 path bounces the step to `pending`
with `not_before = NOW_UTC()`, never to `failed`.

**Each replica is independent; no peer count, no rate division.** With the per-replica throttle,
each replica sees only its own dispatches and its own 429s, so the `wCong` it cuts to is *already*
the per-replica rate. There is nothing to divide. The cluster aggregate is the sum of independent
per-replica controllers, each of which converges to its own share of the downstream's budget via
its own 429 feedback. Asymmetric load across replicas produces asymmetric `wCong` values, which is
correct - a replica that emits more should have a tighter rate. The earlier `Sonar` / `peerCount`
ticker that fed an explicit rate division was vestigial from the cluster-wide DB-count design and
has been removed.

**Why no `Retry-After` parsing in v1.** The Microbus error reconstitution converts non-2xx responses
to errors and does not preserve response headers across that boundary. A wrapper microservice that
genuinely wants per-step backoff timing for a known third-party rate limit should call
`flow.Sleep(retryAfter)` inside the task handler, which routes through the existing `not_before`
machinery and is already correct. From the foreman side a 429 just cuts the valve; the throttle
gates the next dispatch. Retry-After parsing becomes feasible if the framework grows header
preservation, or via a sidecar mechanism.

**No startup snapshot pull.** The map starts empty; a restarted replica admits unconstrained
until its first 429 anchors a point (and the gossip joins it to the cluster view). The transient
over-admission window is bounded by the worker pool. Adaptive limiters re-learn from observed
backpressure; they don't persist state across reboots.

**The recovery curve is TCP CUBIC** (RFC 9438). `recoverRate` evaluates
`w(t) = cubicC*(t-K)^3 + wMax`, where `K = cbrt(wMax * cubicBeta / cubicC)` and
`wMax = wCong / (1 - cubicBeta)`. Below `K` the curve is concave (fast reclaim back toward the
known-good `wMax`); above `K` it is convex (gentle probing for new capacity above the pre-cut
estimate). At `t=0` it equals `wCong`; at `t=K` it equals `wMax`. The shape and the parameter pair
`(cubicBeta, cubicC)` come straight from TCP CUBIC for the same reason: aggressive recovery of
known headroom, cautious exploration of unknown headroom. The `cubic` prefix on both constants is
the provenance signal - any reader who needs the math can read the RFC.

**The recovered rate is clamped to `[1, MaxInt]`.** The floor prevents recovery deadlock (with
the rate at zero, no step can dispatch, no 429 can fire, and the cube term never lifts the curve).
The ceiling is overflow insurance only - a long-idle valve's convex `cubicC * delta^3` term grows
without bound, and the `int(...)` conversion wraps. The ceiling is deliberately not a backpressure
mechanism: a task that genuinely has no downstream limit must remain effectively unconstrained.
The Per-Task Rate-Limit Grafana panel uses a log y-axis so the cut state (1-1000) and the
unbounded state (limit at the ceiling) both render legibly on one chart.

**Curve constants are package-level constants, not configs.** `cubicBeta` (0.01) and `cubicC`
(0.05) live as `const` in `backpressure.go`'s `recoverRate`. `cubicBeta` is now used only as the
curve's `wMax / wCong` ratio (recovery target sits just above wCong); the *cut depth* is a fixed
1 op/sec per 429. The throttle window is similarly fixed at 1 second - the controller converges
on whatever rate the downstream tolerates regardless of the downstream's stated unit (per-second,
per-minute, per-hour are all just rate ÷ second after enough convergence). Earlier configs
`CubicBeta`/`CubicC`/`CongestionDebounce` were a coupled triple without an operator story for
picking any one in isolation; demoting them removed the footgun.

**The bounced step has no added `not_before` delay.** A 429 returns the step to `pending` with
`not_before = NOW_UTC()`. The throttle is the gate, not the row: the re-attempted dispatch hits
`valvePeek` which consults the throttle, and unless the throttle has room the step waits. The
throttle stays full for the rest of the current window after a burst, so the bounced step won't
keep eating dispatches against a still-overrun ceiling.

**`runRefill`'s band loop is unbounded.** The loop walks priority bands ascending until either
admittable work is found (admit and break) or `scanBand` returns `MaxInt` (no more pending work
anywhere). An earlier `BandFallthroughLimit = 4` cap was removed because it didn't actually bound
worst-case behavior - giving up early just deferred the same scan to the next refill, while
introducing a stall failure mode if more than the cap's worth of bands were simultaneously
saturated (workers would idle indefinitely while bands deeper than the cap still had admissible
work). Each iteration is one parallel `scanBand` call (one indexed query per shard), and the
total work scales with the number of distinct priority values in the system.

**Fairness within a key is FIFO among admissible steps, not absolute FIFO.** Within a fairness key
(per-tenant bucket), the refiller walks the bucket's steps in oldest-first order but skips past any
step whose task's throttle currently refuses. The skipped step stays pending and is re-considered
on the next refill once the throttle has room (window rotation, ceiling raised by recovery, or
in-flight work drained). This preserves the existing "strict-FIFO within a fairness key" invariant
for the admissible subset, while preventing head-of-line blocking: one rate-limited task in a
tenant's queue does not freeze the tenant's admissible work for other tasks. The weight assigned
to a fairness key follows the same rule - taken from the oldest *admissible* step.

### Step Parking (`parked` column)

`microbus_steps.parked SMALLINT NOT NULL DEFAULT 0` is a discriminator that takes a step
out of the selection band without changing its `status`. The selection index
`(status, parked, priority, fairness_key)` and the saturation index `(status, parked, task_name)`
both lead with the partitioning columns, so parked rows are physically excluded from every
hot-path scan - no in-memory filter is needed at refill time. The `parked` value labels *why*
the step is held:

- `parked=0` (`parkedNone`, the default) - active. Selection sees the row; `pollPendingSteps`
  recovers it if its lease expires; saturation counts it as one in-flight slot for its task.
- `parked=1` (`parkedSubgraph`) - the step called `flow.Subgraph` and is waiting for the child
  flow to resolve. `status='running'` (the step IS running, logically; it's just blocked on its
  child) but the parked discriminator excludes it from the selection scan, the saturation count,
  AND the lease-expiry recovery in `pollPendingSteps`. There is no lease deadline on a parked
  surgraph step - the row sits indefinitely until `completeSurgraphFlow` flips it back to
  `(status='pending', parked=0)`. This replaces an earlier design that used `lease_expires =
  NOW + 7 days` as the "park" indicator, which broke for subgraphs that ran longer than 7 days
  (the lease lapsed, `pollPendingSteps` recovered the parent, and the task re-ran - launching a
  duplicate child).
- `parked=2` (`parkedBreaker`) - the step's task has a tripped breaker and this step is on the
  held-back backlog. `breakerBulkPark` (in `breaker.go`) parks every pending step for the task
  to parked=2 in a single UPDATE, then elevates the oldest per shard back to parked=0 as the
  probe. Both UPDATEs run inside one per-shard tx, so the elevated probe is guaranteed
  status=pending at commit time (the park UPDATE's row locks block concurrent Cancel/Fail
  until the tx commits). The probe's `not_before` carries the exponential probe schedule. New
  steps inserted for the task while the breaker is tripped are born parked=2 (via
  `initialParkedFor` consulted at INSERT time) so they join the held-back set without burning
  a wasted dispatch attempt. On probe success `breakerClose` runs `breakerBulkUnpark` scoped
  to *that shard only* - the parked steps on other shards stay held until their own probes
  succeed. Symmetric with trip (which is also per-shard), and produces a rolling-wave
  recovery instead of a unison flood.

The two reasons coexist in the same column because they share the same selection-time
exclusion. New park reasons (e.g. dependency-wait, manual operator hold) can take additional
small-int values without schema migration; only the SQL-side `parked=0` predicate and the
in-memory enum-to-name mapping for logs/metrics would need updating.

**Terminal status implies `parked=parkedNone`.** The park value is meaningful only while a step
is actively waiting for something - a subgraph child to finish (`parkedSubgraph`), or a tripped
breaker to admit the next probe (`parkedBreaker`). Once the step is in any terminal status
(`completed`/`failed`/`cancelled`), the park slot is by definition gone, and the column
must read `parkedNone`. Every code path that transitions a step into a terminal status resets
`parked` in the same UPDATE: the `failStep` originating-step write (`completion.go`), the
`deliverSubgraphError` child-leaf write, the `Cancel` cascade UPDATEs across the flow chain and
the surgraph caller chain (`service.go`), and the `processStep` terminal-flow guard
(`execution.go`) all include `parked=parkedNone`. Without this invariant, a step that was
`parkedSubgraph` when its flow was cancelled (or `parkedBreaker` when its breaker held it during
a cancel) would sit in a terminal state with non-zero `parked` - invisible to the selection
index but never re-leased. The selection index `(status, parked, priority, fairness_key)` would
then strand any subsequent `Restart` or `RestartFrom` on that step: the rewind sets
`status=pending` but the partitioned index still excludes the row because `parked != 0`. The
flow looks `running` but no worker ever picks the entry up - the user-visible "stuck after
Restart" symptom. `Restart` and `RestartFrom` also re-zero `parked` in their target-step
UPDATE as defense-in-depth, so a future code path that introduces a third cascade and forgets
the rule still doesn't wedge the rewind.

**Insert paths consult the in-memory breaker map** via `initialParkedFor(taskName)` and bind
the result into the `parked` column. The PENDING-inserting sites are the fan-out children
(`processStep` in `execution.go`) and the linear and fan-in transitions (`transitions.go`). The
CREATED-inserting site (`createWithGraph` entry step) leaves `parked=0` because the row isn't yet
selectable; `Start` runs `parkTrippedSteps` inside its transition transaction to honor the breaker
at the moment CREATED becomes PENDING. `Restart`/`RestartFrom` rewind their target step via UPDATE
rather than insert, re-zeroing `parked` (see above).

**No stale-dispatch race during trip.** `handleBreakerTrip` bounces the failed step to
`parked=parkedBreaker` (not `parkedNone`), so the refiller's `parked=0` filter cannot pick
up the just-bounced row during the window before `breakerBulkPark` runs. The tx that wraps
`breakerBulkPark`'s park-then-elevate then holds row locks on every pending step for the
task across the SELECT and the elevate UPDATE; concurrent Cancel/Fail paths wait on those
locks, so the chosen probe is guaranteed `status=pending` when the tx commits. The
insert-time consult (`initialParkedFor`) prevents a brand-new insert from landing at
parked=0 while the breaker is tripped.

The park-then-elevate tx is the only thing that leaves a `parked=parkedNone` probe behind, so
if it rolls back on lock contention the task is left with *no* probe: nothing dispatches, no
further 404 re-trips, and nothing re-runs bulk-park - recovery wedges permanently.
`handleBreakerTrip` therefore retries `breakerBulkPark` on `sequel.IsLockContentionError`
(a few short attempts) so a probe is re-elected before it returns. This matters on SQLite,
where a contended write fails fast rather than blocking; the server dialects usually block on
the row locks instead, but the retry is harmless there.

### Per-Task Breaker (404 ack-timeouts, 503 unavailable, 529 overloaded)

A separate per-task circuit breaker rides alongside the valve in `breaker.go`. The valve handles
429 backpressure ("the downstream is rate-limiting"); the breaker handles 404 ack-timeouts ("no
one is home"), 503 Service Unavailable ("downstream not ready / in maintenance"), and 529
overloaded ("the downstream is collapsed"). The valve gates at the `admittable()` closure in
`runRefill` via `valvePeek`; the breaker doesn't participate in admittable() at all - it gates
via the `parked` column (see "Step Parking" above), which takes blocked rows out of the
selection index entirely. The valve and breaker operate independently; a task with both signals
tripped has its rate cut AND its backlog parked.

**Why 503 and 529 go to the breaker, not the valve.** 503 Service Unavailable (the standard
"I'm here but unavailable" signal - in-flight deploy, startup not finished, key store not
loaded, etc.) and 529 (Site Overloaded; non-standard, used by Cloudflare and some third-party
APIs) both mean the downstream is unavailable, not that it is serving requests at a reduced
rate. The valve's additive-decrease-with-CUBIC-recovery is designed for the latter: cut the
rate by 1, keep dispatching at a slower pace, ride the recovery curve back up. Applied to a
downed downstream, every dispatch keeps tripping 503/529, the rate drives toward zero, and the
recovery curve then re-floods the still-down service. The breaker's trip + exponential probe
schedule (100ms, 200ms, 400ms, ..., capped at 1m) lets the downstream actually recover before
we send the next request. The `executeTask` check is symmetric to the 404 case: a switch on
`StatusCode` routes 404-with-`ack timeout:`-prefix, 503, and 529 all into `handleBreakerTrip`
with the same trip-and-bounce mechanics; only the cause label distinguishes them in metrics
and logs. A handler-emitted 503 is a deliberate "back off and try later" signal from the
downstream - the breaker's exponential schedule is the right response. Other 5xx (500, 502,
504) remain task-level decisions; see "5xx is not the breaker's business" below.

**Why per-task and not per-step.** A per-step exponential backoff (the obvious first answer) is
structurally wrong because it scales with backlog depth: with 100 flows blocked on a 404'ing task,
every refill cycle would dispatch all 100 and bounce them all on their individual exponential
schedules - an avalanche that displaces work for healthy tasks. The throttle has to live at the
*task name* level so one schedule governs all blocked flows.

**The parked column is the gate; the probe rides on `not_before`.** When a breaker trips,
`breakerBulkPark` runs a per-shard tx that parks every pending step for the task to
`parked=2` (single UPDATE), then SELECTs the oldest of those parked rows and elevates it
back to `parked=0` as the probe (second UPDATE). The selection index
`(status, parked, priority, fairness_key)` is partitioned on parked, so parked rows are
physically excluded from the candidate scan - the refill never sees the held-back backlog and
needs no in-memory `breakerAdmits` filter. The surviving probe step's `not_before` is set to
`nextProbeAt`, so the existing selection machinery (which already honors `not_before` for
`flow.Sleep` and retry backoff) naturally schedules the probe at the exponential cadence.
When the probe succeeds, `breakerClose` runs `breakerBulkUnpark` scoped to the success
shard - one UPDATE on that shard only. Other shards stay parked until their own probes
succeed. While tripped, *new* pending steps inserted for the task consult the in-memory
breaker map at insert time (via `initialParkedFor`) and are born `parked=2` so they join
the held-back backlog without burning a wasted dispatch attempt.

**404 detection is the literal `ack timeout:` prefix.** The connector's unicast publisher
synthesizes the `404 Not Found` for an unacknowledged request as
`errors.New("ack timeout: %s", req.Canonical())` in `connector/publish.go`. A 404 from a
subscriber that actually ran (e.g. a wrapped REST API returning "not found") does *not* carry
that prefix, so the breaker only fires for genuine transport-level "no subscriber" 404s. The
detection check in `executeTask` is therefore the
`StatusCode == 404 && strings.HasPrefix(err.Error(), "ack timeout")` pair, and a handler-emitted
404 falls through to OnError/failStep like any other error. 503 and 529 need no payload sniff
because their legitimate meanings ("unavailable" and "overloaded") align with the breaker's
behavior - any handler emitting them is signaling "back off", which is exactly what the breaker
provides; all 503/529 responses route to the breaker.

**Trip on the first signal.** Any single `ack timeout:` 404, 503, or 529 flips the breaker
to tripped - no consecutive-nack counter, no threshold to tune. The cost of an over-trip is
exactly one `breakerInitialProbeDelay` (100ms) of added latency on the next dispatch attempt
for that task: trip -> 100ms wait -> probe -> success (when the blip was transient) -> close.
The cost of an under-trip would be the avalanche. With the cost of a false trip pinned to
~one probe interval, there is no reason to delay tripping; matches the TCP analogy where one
loss event triggers congestion response. A flaky downstream produces frequent trip/close
oscillation which shows up cleanly in `microbus_task_breaker_trips_total` - useful
operational signal, not a bug.

**Probe schedule is local, exponential, capped.** When tripped, exactly one step per shard
per task is left unparked as the probe, gated on `not_before <= NOW`. The schedule advances
on each genuine probe DISPATCH, not on failure: `processStep` calls `breakerCommit(taskName)`
just before `executeTask` for any step whose task has a tripped breaker, and `breakerCommit`
advances only when the probe's `nextProbeAt` has actually arrived (`now >= nextProbeAt`),
no-op otherwise. That probe-time gate is load-bearing because not every dispatch of a tripped
task is a real probe: a burst of dispatches admitted just before the trip stays in flight and
returns 404s afterward, and the candidate cache can still hand a worker a step that was parked
after being offered (the claim CAS rejects it, but only after a pop). Counting those as probe
attempts would ramp `probeAttempt` straight to the cap and stall recovery. On a genuine probe,
`breakerCommit` advances `probeAttempt` and recomputes `nextProbeAt = now +
probeBackoff(probeAttempt)`; the failure path then writes that deadline to the probe's
`not_before` when `breakerBulkPark` re-runs. Advancing at admission - one bump per probe -
matches the rate-limiting the original `breakerAdmits` gate provided; advancing on
*failure* would over-penalize because a burst of in-flight 404s from pre-trip dispatches
would each bump `probeAttempt` and ramp the backoff to its cap in one window. `nextProbeAt`
and `probeAttempt` are deliberately *not* gossiped; each replica advances its own state on
observed admissions, and the probe's `not_before` is a last-writer-wins UPDATE across
replicas - converging within clock skew. All four constants (`breakerInitialProbeDelay` =
100ms, `breakerProbeMultiplier` = 2.0, `breakerMaxProbeDelay` = 1m, trip-after-first) live in
`breaker.go` rather than configs: the 100ms initial makes one-off blips nearly invisible, the
1m cap bounds recovery-detection latency, and the values are short enough that test fixtures
run in real time without config overrides. Sequence: 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s,
6.4s, 12.8s, 25.6s, 51.2s, 60s, 60s, ... ; full convergence to the cap takes ~2 minutes of
continuous failure.

**`breakerTrip` arms the schedule; `breakerCommit` advances it; `refreshNextProbeLocked`
syncs the timer.** There's no separate `breakerAdmits` predicate (it would do nothing now
that parked rows are physically out of the selection index). `breakerTrip` is called for
every observed 404/529 - on a fresh trip it arms the first probe (`probeAttempt = 0`,
`nextProbeAt = now + 100ms`); on a repeat trip it is a no-op so concurrent in-flight 404s
from pre-trip dispatches don't ratchet the schedule. `breakerCommit` (called from
`processStep` at probe admission) advances `probeAttempt` and `nextProbeAt`. `breakerClose`
zeroes the state. All three paths call `refreshNextProbeLocked`, which recomputes the
cluster's soonest probe into the timer's `nextProbe` deadline and nudges the timer. The
timer wakes at the next probe time regardless of the 1-minute `backlogPollInterval`.
`nextProbe` is a *separate* deadline from `nextPoll` precisely so `pollPendingSteps`'
per-cycle `nextPoll` reset cannot clobber the probe wake; an earlier design that shortened
`nextPoll` for the probe collapsed the breaker's exponential schedule to the backlog
cadence whenever a poll landed as the probe came due (see the timer's two-deadline note
above).

**`TripBreaker` gossips only the trip event, payload-free.** The wire is just `(taskName)` -
no timestamp, no state field. Each peer stamps `time.Now()` on receipt, seeds its local
probe schedule from that moment, AND runs `breakerBulkPark` so its view of the task's
pending set is consistent with the originating replica's. Cross-replica clock skew
(NTP-synced, same datacenter; milliseconds) is negligible against the probe timescale (100ms
to 1m). `breakerBulkPark` is idempotent SQL - multiple replicas converging on the same trip
all run it; the result is a single shared probe step per shard with the rest of the
pending backlog at `parked=2`. A trip received while already tripped is a no-op (we don't
re-park or refresh the schedule, because that would arbitrarily reset `probeAttempt` and
undo accumulated backoff).

**Closures are deliberately NOT gossiped, and the unpark UPDATE is shard-scoped.** A replica
that probes successfully drops its local `trippedAt` to zero and runs `breakerBulkUnpark`
against the success shard only - the parked rows on other shards stay parked=2 until each of
those shards' own probes succeeds. This is symmetric with the per-shard trip (which leaves
one probe per shard at parked=0) and produces a rolling-wave recovery: each shard's released
backlog hits downstream as that shard's probe succeeds, spread over hundreds of milliseconds
or seconds of natural dispatch variance instead of every shard's backlog flooding in unison.
If downstream is flaky and 404s again after a shard recovers, the next dispatch on that shard
re-trips the breaker and bulk-park runs again - independently of other shards' state. The
TCP slow-start analogy still holds: aggressive on cut, gentle on recovery.

**Small consistency window across replicas.** Closures aren't gossiped, so when replica R1's
probe on shard X succeeds and unparks shard X's rows, R2's in-memory breaker for the task
is still tripped. Until R2 also probes successfully (which happens naturally when R2 picks
up one of the newly-unparked rows and dispatches it), R2's `initialParkedFor` would insert
new steps at parked=2 even though shard X is now healthy. This stuck-parked-2 row sits
invisible until R2's own close fires and bulk-unparks - one-dispatch-per-replica bounded
window. Acceptable per the design (matches the original "locally self-correcting" closure
philosophy) and not worth a gossip RPC.

The state is collapsed to a single `trippedAt time.Time` field that doubles as the indicator
(zero = closed). No bool, no constants for the state value. The gossip merge is just "set my
trippedAt if I was closed" - idempotent under re-receipt, no convergence-rule reasoning needed.

**Reconstitution on startup.** A restarting replica's in-memory breaker map is empty, but
`parked=parkedBreaker` rows from a prior replica survive in the DB and are invisible to the
selection index. Without intervention those rows would sit forever - `pollPendingSteps` only
recovers `parked=0` steps, and the breaker's own probe-elevation only runs on a fresh trip.
`reconstituteBreakers` (called inside `OnStartup` between the breakers-map init and the worker
goroutine starts) scans each shard for `SELECT DISTINCT task_name FROM microbus_steps WHERE
parked=parkedBreaker AND status=pending` and calls `breakerTrip(taskName)` for each result. The
schedule starts fresh on this replica (`probeAttempt=0`, first probe at `now+100ms`), independent
of where peers are in their own schedules - the breaker subsystem is per-replica by design. **No
multicast.** The gossip merge rule is "latest tCong wins"; sending our `time.Now()` would clobber
peers' accumulated `probeAttempt` with a newer timestamp, undoing their backoff. Reconstitution
is strictly a local fix-up for this replica's in-memory state. The DB-side state (parked rows +
the probe row per shard) is already what `breakerBulkPark` would produce, so no SQL writes are
issued. If the task has actually been fixed during downtime, the first reconstituted probe
(at `now+100ms`) succeeds and `breakerClose` un-parks the backlog normally - recovery latency is
one probe interval. The alternative considered and rejected - a blanket `UPDATE parked=0 WHERE
parked=parkedBreaker` at startup - would mass-unpark every replica's held backlog in a
multi-replica deployment, causing a dispatch flood into a known-broken endpoint before each
replica's breaker re-trips and re-parks. Local reconstitution avoids that thrash entirely.

**No auto-give-up on a forever-tripped breaker.** Flow lifetime is workflow-author's
responsibility - same argument as the "Flow lifetime is workflow-author's responsibility"
section. A breaker that has been tripped for hours might be a long maintenance window, in which
case the workflow's own elapsed-time guard (if any) is the right arbiter. Operator visibility
comes from `microbus_task_breaker_state{task_name}` and `microbus_task_breaker_probes_total
{task_name, outcome}`; bulk remediation (cancel everything blocked on task X) is operator
surface area that's not yet built, and is deliberately out of scope for the breaker itself.

**Other 5xx (500, 502, 504, ...) are not the breaker's business.** A 5xx response other than
503/529 means the downstream exists, served the request, and answered with a server error -
that is a task-level decision (use `flow.Retry`, an `onError` transition, or just propagate
as flow failure). The breaker is specifically for "I cannot serve right now" signals: 404
ack-timeout via the `ack timeout:` prefix (no subscriber acknowledged), 503 (handler emitting
"unavailable"), or 529 (Cloudflare/proxy "overloaded"). Everything else falls through.

## Schema Column Catalog

The `resources/sql/*.sql` migration files carry **no prose comments by design** - only the functional
`-- DRIVER: <dialect>` directives the `sequel` runner parses to select the per-dialect statement (it splits on
`;\n`, then strips every `--` line before executing, so descriptive comments were dead weight). All schema
rationale and the meaning of every column live here.

#### `microbus_flows`

| Column | Meaning |
|---|---|
| `flow_id` | Per-shard auto-increment primary key. The external flowKey is `{shard}-{flow_id}-{flow_token}` |
| `flow_token` | Random token component of the flowKey, guards against id guessing |
| `workflow_name` | URL/name of the workflow graph this flow runs |
| `graph` | JSON of the workflow graph, frozen at `Create` time |
| `actor_claims` | JSON of the caller's JWT claims captured at `Create`; a fresh access token is minted from these before each task |
| `status` | Flow lifecycle: `created`/`running`/`interrupted`/`completed`/`failed`/`cancelled` |
| `step_id` | The flow's current step; `0` during fan-out (multiple steps active at one depth) |
| `surgraph_flow_id` | Parent (surgraph) flow id if this is a subgraph flow; `0` otherwise |
| `surgraph_step_depth` | The parent's `step_depth` that spawned this subgraph |
| `surgraph_step_id` | (`3.sql`) PK of the parent's parked surgraph step, so a subgraph flow identifies its surgraph step unambiguously when parallel parked surgraph steps coexist at one depth. `0` = legacy row, falls back to the lease-threshold search |
| `thread_id` | Groups flows in a `Continue` thread; defaults to `flow_id` (each flow its own thread) |
| `thread_token` | Token component of the thread's flowKey |
| `trace_parent` | W3C `traceparent` for distributed-trace continuity across the flow |
| `notify_hostname` | Set by `StartNotify`; `OnFlowStopped` fires here when the flow stops. `''` = no notification |
| `final_state` | JSON state computed at termination - the full merged state of the terminal step(s), unfiltered. Any narrowing happens in the workflow's terminal task via `flow.Delete`/`Transform` |
| `breakpoints` | JSON `map[taskName]string` of `BreakBefore` breakpoints |
| `created_at` | UTC creation time. Append-only and PK-correlated; `idx_*_created_at` relies on this for time-window queries. Surfaced to tasks via `Flow.CreatedAt()` for elapsed-time guards. Reset by `Restart` (which is a fresh attempt); NOT reset by `RestartFrom` (which is a surgical rewind) |
| `started_at` | (`15.sql`) UTC time this attempt began dispatching. Stamped by `Start` when the flow transitions `created` → `running`. Reset by `Restart` (fresh attempt). NOT reset by `RestartFrom` (the flow's run continues, only a subtree is rewound). Distinct from `created_at` because a flow can sit in `created` indefinitely waiting for `Start`, or be parked behind a tripped breaker for hours before its entry step actually dispatches; `started_at` skips that pre-run delay. Drives `FlowSummary.Duration()` (`updated_at - started_at`) so the duration column in operator UIs reflects execution time, not row age |
| `updated_at` | UTC time of the last status transition; bumped on every state change. Surfaced to tasks via `Flow.UpdatedAt()` |
| `priority` | (`8.sql`) Scheduling priority, integer >= 1, lower runs first. Resolved at `Create` from `FlowOptions` (an explicit priority is >= 1; a 0/unset `FlowOptions.Priority` falls back) else the `DefaultPriority` config; inherited unchanged by `Continue`/subgraph. Immutable after creation |
| `fairness_key` | (`8.sql`) Fairness bucket, typically a tenant. From `FlowOptions`, else the `tid`/`tenant` actor claim, else `''`. Immutable after creation |
| `fairness_weight` | (`8.sql`) Relative dispatch share of the `fairness_key`. From `FlowOptions`, else `1` |
| `error` | (`9.sql`) Task error string for `failed` flows. Written by `failStep` to every flow in the surgraph chain in the same UPDATE that sets `status='failed'`; the existing `WHERE status NOT IN (terminal)` clause makes the write first-failure-wins for the flow row even under concurrent sibling failures. `''` for non-failed flows. Surfaced as `FlowOutcome.Error` |
| `cancel_reason` | (`9.sql`) Reason string passed to `Cancel(flowKey, reason)`. Written to every flow in the cancellation chain (root + descendants) in the same UPDATE that sets `status='cancelled'`, gated by the same WHERE-clause for first-cancel-wins semantics. `''` when no reason was given or for non-cancelled flows. Surfaced as `FlowOutcome.CancelReason` |
| `tenant_id` | (`9.sql`) Tenant identifier read from `frame.Of(ctx).Tenant()` at flow creation. `0` is the no-tenant sentinel (returned by `Tenant()` when no actor is on the frame or the actor has no `tid`/`tenant` claim). Inherited by subgraph (worker context has no frame). `Continue` and `CreateTask` read from the caller's frame like `Create`. Immutable after creation. Filterable via `Query.TenantID` for operator multi-tenant queries. |

#### `microbus_steps`

| Column | Meaning |
|---|---|
| `step_id` | Per-shard auto-increment primary key, globally unique within a shard across flows. External stepKey is `{shard}-{step_id}-{step_token}` |
| `flow_id` | Owning flow |
| `step_depth` | Sequential transition depth; fan-out siblings share it. Used for history ordering and legacy depth-based fan-in, **not** for the execution DAG (see "Execution-DAG edges") |
| `step_token` | Random token component of the stepKey |
| `task_name` | Graph node name of the task this step executes |
| `state` | JSON input snapshot. Immutable except on retry/resume (see "State mutation on retry and resume") |
| `changes` | JSON output delta the task produced |
| `interrupt_payload` | JSON outbound payload from `flow.Interrupt()` - what the awaiting caller sees |
| `interrupt_done` | (`10.sql`) `1` once the interrupt park armed by `flow.Interrupt` has been resumed; drives `flow.Interrupt`'s return-vs-arm decision on re-dispatch. `0` for breakpoint pauses (so a task resumed past a breakpoint arms its interrupt fresh) |
| `resume_data` | (`10.sql`) JSON inbound payload recorded by `Resume`; returned by `flow.Interrupt` on re-dispatch. `'{}'` until resumed |
| `subgraph_done` | (`10.sql`) `1` once a `flow.Subgraph` park has resolved (child completed/failed); drives `flow.Subgraph`'s return-vs-arm decision on re-dispatch. A retry clears it back to `0` to re-run the child |
| `subgraph_result` | (`10.sql`) JSON child `final_state` returned by `flow.Subgraph` on re-dispatch. `'{}'` until resolved |
| `subgraph_error` | (`10.sql`) child error text for a failed `flow.Subgraph` park, returned as the `err` from `flow.Subgraph`. `''` when none |
| `status` | Step lifecycle: `created`/`pending`/`running`/`interrupted`/`completed`/`failed`/`cancelled` |
| `goto_next` | Task-requested `flow.Goto` target; `''` = none |
| `error` | Error text when `failed`; `''` otherwise |
| `time_budget_ms` | Execution budget; the HTTP timeout on the task call |
| `breakpoint_hit` | `1` once a breakpoint on this step has fired, so it does not re-trigger on resume |
| `attempt` | Task-initiated retry (`flow.Retry`) attempt counter, drives the backoff |
| `not_before` | Earliest UTC time the step may execute (`flow.Sleep` / retry backoff) |
| `lease_expires` | Crash-recovery lease; `pollPendingSteps` reclaims `running` steps past this |
| `created_at` | UTC creation time |
| `started_at` | (`14.sql`) UTC time the *current attempt* of this step first dispatched - i.e. the first lease of attempt 0, or the first lease after a Restart/RestartFrom rewind. The lease UPDATE uses a CASE expression to stamp `started_at = NOW_UTC()` only when this is a fresh attempt's first dispatch (`attempt = 0 AND subgraph_done = 0 AND interrupt_done = 0`), and **preserves** the existing value when the lease is a *continuation* within the same attempt: a `flow.Subgraph` re-dispatch (`subgraph_done = 1`), an `Interrupt`/`ResumeBreak` re-dispatch (`interrupt_done = 1`), or a `flow.Retry` re-dispatch (`attempt > 0`). The "retry stays inside the step" rule matters: a retried step's duration includes every attempt's task body, not just the last attempt's. Defaults to `NOW_UTC()` at INSERT so a never-leased row carries a valid value; consumers ignore it for non-`HasStarted` statuses (`created`/`pending`), where it has no operational meaning. Drives per-step body duration (`updated_at - started_at`) and inter-step wait labels (`next.started_at - prev.updated_at`) in `FlowRenderer`; the latter measures only the *transition gap* (DB commit + queue + dispatch overhead), not the inside-the-step retry/park time |
| `updated_at` | UTC time of the last status transition |
| `lineage_id` | (`5.sql`) Cohort frame: the spawn step's `step_id` on a push, else inherited. Drives explicit `SetFanIn` arrival counting and the merge. A cohort-counting device, **not** a DAG. `0` = no `SetFanIn` |
| `cohort_size` | (`5.sql`) On a fan-out spawn step: number of branches spawned |
| `cohort_arrivals` | (`5.sql`) On a fan-out spawn step: branches that reached the fan-in; fan-in fires when `arrivals >= size` |
| `fan_out_ordinal` | (`6.sql`) This branch's index in its fan-out (forEach array index / static declaration order); fan-in merges in this order so list/sum reducers are deterministic. Preserved across an in-place rewind (`flow.Retry`/`RestartFrom`). `0` = not part of a fan-out |
| `predecessor_id` | (`7.sql`) Step that ran immediately before this one in the execution DAG. `0` = none (entry/subgraph-entry) |
| `successor_id` | (`7.sql`) Step that runs immediately after this one. `0` = none (exit) |
| `priority` | (`8.sql`) Denormalized copy of the owning flow's `priority` for the hot selection path. Copied at every step-insert path so selection never joins `microbus_flows`. Read by the two-level selection |
| `fairness_key` | (`8.sql`) Denormalized copy of the flow's `fairness_key` |
| `fairness_weight` | (`8.sql`) Denormalized copy of the flow's `fairness_weight` |
| `parked` | (`11.sql`) Selection discriminator. `0` = active (visible to selection); `1` = surgraph park (the step called `flow.Subgraph` and is waiting for the child); `2` = breaker park (the task has a tripped breaker and this row is on the held-back backlog). The selection and saturation indexes lead with `(status, parked)` so non-zero rows are physically excluded from the hot-path scan. See "Step Parking" above for the full design |

## Database Indexing Strategy

The foreman's `microbus_flows` and `microbus_steps` tables grow indefinitely as workflows execute. The indexing strategy is designed to keep the hot-path queries fast without introducing fragmentation or excessive write amplification.

### Design Principles

1. **Append-only terminal sections.** Indexes that include `status` as the leading column naturally partition the B-tree by status value. Terminal statuses (`completed`, `failed`, `cancelled`) are append-only because entries arrive with a monotonically increasing `updated_at` timestamp (always `NOW_UTC()` at the time of the status transition). This means the terminal sections of the B-tree stay well-ordered - no mid-tree page splits, no fragmentation.

2. **Small transient sections.** The `pending` and `running` sections of status-based indexes churn as steps are created and processed, but they remain small (proportional to active work, not historical volume). Page reuse within these sections is efficient because the working set fits in a few leaf pages.

3. **Partial indexes for PostgreSQL.** Where only non-terminal statuses are queried through an index (e.g., `pollPendingSteps`), PostgreSQL uses a partial index filtered to `status IN ('pending', 'running')`. This keeps the index tiny regardless of table size. MySQL and SQL Server use the full composite index since they lack partial index support.


### Index Catalog

#### `microbus_flows` table

| Index | Columns | Purpose |
|---|---|---|
| PK | `(flow_id)` | Row lookups by flow ID |
| `idx_microbus_flows_status` | `(status, updated_at)` | `List` by status. Append-only terminal sections as described above |
| `idx_microbus_flows_workflow_name` | `(workflow_name)` | `List` by workflow name. Low cardinality (hundreds of distinct values) but high volume. Actual key length is ~40 bytes despite the 512-char column definition since all three databases store variable-length strings at actual length in B-tree indexes |
| `idx_microbus_flows_thread` | `(thread_id, flow_id)` | `Continue` (find latest flow in thread) and `List` by thread. Point lookups by thread_id with flow_id ordering for "latest" queries |
| `idx_microbus_flows_surgraph` | `(surgraph_flow_id)`, partial `WHERE surgraph_flow_id > 0` on pgx/sqlite | Walking the subgraph chain (parent → child subgraph flows) |
| `idx_microbus_flows_created_at` | `(created_at)` (migration `4.sql`) | Time-window queries (e.g. "flows created in the last hour"). `created_at` is append-only/monotonic in practice so new rows land at the rightmost leaf page - minimal B-tree maintenance, no random page splits |

#### `microbus_steps` table

| Index | Columns | Purpose |
|---|---|---|
| PK | `(step_id)` | Row lookups, lease acquisition in `processStep` |
| `idx_microbus_steps_flow_id` | `(flow_id, step_id)` on MySQL; `(flow_id)` on pgx/mssql | Per-flow step queries (history, fan-in siblings, cancel) |
| `idx_microbus_steps_status` | `(status, updated_at)` - partial on pgx: `WHERE status IN ('pending', 'running')` | `pollPendingSteps` recovery and pending step discovery. Only non-terminal statuses are queried through this index |
| `idx_microbus_steps_created_at` | `(created_at)` (migration `4.sql`) | Time-window queries; append-only, same minimal-maintenance rationale as the flows index |
| `idx_microbus_steps_selection` | `(status, parked, priority, fairness_key)` (migrations `8.sql` + `11.sql`) - partial `WHERE status IN ('pending', 'running')` on pgx/mssql/sqlite, full on mysql | Two-level priority+fairness candidate selection. Read by the refiller (band MIN(priority), then candidate rows by key). The `parked` second column physically excludes parked rows (surgraph park, breaker park) without an in-memory filter |
| `idx_microbus_steps_saturation` | `(status, parked, task_name)` (migrations `8.sql` + `11.sql`) - partial as above | Per-task in-flight count for the adaptive backpressure controller. Parked rows are excluded so a surgraph parent doesn't inflate the executing-slot count for its task |

### Data Retention

The foreman does not auto-purge flows. Workflows are durable and every flow row remains
potentially-resurrectable: an `interrupted` flow via `Resume`, a `completed` flow via `Continue`
into the same thread, a `failed` flow via `Restart`/`RestartFrom`. Killing rows on a fixed timer would silently
break any of these patterns, and no single retention policy fits both a 1-hour batch job and a
30-day human-approval workflow.

For operator-driven retention the foreman exposes two endpoints:

- **`Delete(flowKey)`** removes one flow and its steps in a transaction. Refuses to act on a
  running flow (409). Subgraph children and thread descendants are left dangling (their parent
  references become stale rows); this matches the framework's stance that `Continue` lineage is
  best-effort and not guaranteed across operator-driven retention.
- **`Purge(Query)`** bulk-deletes flows matching the query, except running. Same `Query` shape
  as `List` (Status, WorkflowName, ThreadKey, TaskName, OlderThan, Shard, Limit). Capped at
  10000 flows per call; returns the count of flows actually deleted. The non-running guard is
  enforced inside the DELETE so a flow that transitions to running between SELECT and DELETE
  cannot be deleted out from under its workers.

Both endpoints share `queryClauses` with `List` so the filter semantics are identical across
read and write paths. The typical operator pattern is `Purge(Query{Status: "completed",
OlderThan: 30*24*time.Hour})` on a cron. DB-side partitioning/TTL remains an alternative for
high-volume environments.

The `Query.TaskName` filter joins `microbus_steps` on `step_id` and matches the current step's
`task_name`. Useful for "list/purge flows blocked on tripped-breaker task X." Excludes
fan-out flows (`step_id=0`) since they have no single current task.

The `Query.OlderThan` and `Query.NewerThan` filters are database-anchored: they become
`f.updated_at < DATE_ADD_MILLIS(NOW_UTC(), -ms)` and `f.updated_at >= DATE_ADD_MILLIS(NOW_UTC(),
-ms)` respectively. No client/database clock-skew reasoning. They compose: `OlderThan: 1*h,
NewerThan: 24*h` yields "updated between 1 hour and 24 hours ago."

The `Query.TenantID` filter narrows to a single tenant. Useful for multi-tenant operator views.
Tenant 0 (no-tenant) is the framework's sentinel; the filter treats 0 as "no filter," so operators
wanting only the no-tenant flows have to grep client-side. In practice, tenant=0 flows are background
or unauthenticated operations and rarely the target of a tenant query.

## Concurrency and Crash Recovery

The foreman uses SQL transactions for multi-statement operations and `lease_expires` for crash recovery.

### Worker context: `svc.Lifetime()`

Workers, the timer goroutine, and the refiller all share `svc.Lifetime()` as their root context.
The connector creates the lifetime ctx before `OnStartup` runs and cancels it only *after*
`OnShutdown` returns and the soft drain elapses (see `connector/CLAUDE.md` "Shutdown's ordering,
drain budget, and the dlru offload window"). The foreman drains its workers, timer, and refiller
inside `OnShutdown` before returning, so by the time the connector cancels the lifetime ctx every
DB operation has already committed. In-flight writes are never interrupted by ctx cancellation.

The only place a *cancellable*, time-bounded ctx is used is the outbound task HTTP call: `executeTask`
derives `taskCtx` from `svc.Lifetime()` with the step's `time_budget_ms`. That's `pub.Timeout` on the
dispatch, not a guard on the foreman's own DB writes.

### Shutdown ordering: drain workers, then timer, then refiller

`signalTimer` (the sender behind `shortenNextPoll` / `refreshNextProbeLocked`) nudges the timer via
`select { case svc.wakeTimer <- struct{}{}: default: }`. The `default` only guards against a *full* channel -
a send on a *closed* channel still panics, even inside a `select`. The senders are not just worker goroutines:
the breaker subsystem and the `Enqueue` / `TripBreaker` RPC handlers also signal the timer, so there is no
drain point after which a `wakeTimer` send is guaranteed impossible. `wakeTimer` is therefore **never closed**
(the same rationale as `refillTrigger`), and `timerLoop` is terminated by a dedicated `timerStop` channel it
selects on. `OnShutdown` drains the worker pool, then stops the timer, then the refiller, and returns:

```
cache.close()        // unblocks blocked candidate pops independently of any channel
workers.Wait()       // no shortenNextPoll / requestRefill worker caller remains
close(timerStop)     // timerLoop's termination signal (wakeTimer is never closed)
timerWorker.Wait()   // timerLoop fully exited (last requestRefill caller gone)
close(refillStop)    // refiller's termination signal
refiller.Wait()      // refiller fully exited; its DB ops complete
// OnShutdown returns; connector cancels svc.Lifetime() after the soft drain.
```

The timer and refiller each live on their own WaitGroup (`timerWorker`, `refiller`), separate from the
worker pool, so the close-then-wait order can be staged. `timerStop` is drained before `refillStop` because
`timerLoop`'s final poll can still `requestRefill`; stopping the refiller first would lose that work or race
the trigger. `refillTrigger`, like `wakeTimer`, is **never closed** and is only ever sent to non-blockingly,
so a late coalesced `requestRefill` from the timer's final poll during the drain window is a harmless no-op
rather than a `send on closed channel` panic - the refiller is stopped by closing the *separate* `refillStop`
channel instead. A `cache.refill` into the already-closed cache is a no-op, so a refiller run still in flight
during the drain cannot resurrect a popped-from cache. Using never-closed nudge channels plus dedicated
`timerStop` / `refillStop` termination signals removes the ordering hazard an earlier design carried, where
closing `wakeTimer` before `workers.Wait()` let a worker mid-`processStep` (e.g. the retry-sleep
`shortenNextPoll` after a 408/ack-timeout failStep) race the close and panic.

### Transactions

`Start`, `Resume`, `Restart`, `RestartFrom`, and `Cancel` wrap their step and flow mutations in a transaction with **steps-first-then-flow lock ordering** to prevent deadlocks. `processStep`'s transition evaluation (insert next steps + update flow's `step_id`) also runs in a transaction.

### Lease-Based Crash Recovery

Transactions don't help when a worker crashes during task execution (an HTTP call outside any transaction). The `lease_expires` column on the microbus_steps table serves as a crash-recovery lease. When a worker reserves a step for execution, it sets `lease_expires` to `NOW + TimeBudget + leaseMargin` (using the foreman's current in-memory config, see "Time Budgets" above). If the worker crashes, the lease eventually expires and `pollPendingSteps` resets the step to `pending` for re-execution.

### Background Recovery

1. **`pollPendingSteps`** - runs on a timer. Recovers steps stuck in `running` whose lease has expired by resetting them to `pending`. Enqueues `pending` steps that are due.
2. **Terminal flow check** in `processStep` - after loading flow data, checks if the flow is `cancelled`, `failed`, or `completed`. If so, sets the step to that status and returns. Catches races where `Cancel`, `failStep`, or flow completion set the flow to a terminal status before the step was updated.
3. **Orphan flow detection** in `pollPendingSteps` - logs an `ERROR` for any flow in `running` status that has no non-terminal steps (`pending`, `running`, `created`, `interrupted`) anywhere and whose `updated_at` is older than 5 minutes. This is a bug signal - in steady state the condition should never hold. Auto-recovery is intentionally **not** implemented: it would have to duplicate the fan-in/transition logic in `processStep`, and acting on a false positive (e.g. a flow that was actually mid-transaction) could double-advance the flow. The 5-minute threshold is comfortably past every normal transient state and avoids log noise during the microsecond windows of fan-in.

### Per-Function Analysis

#### Create / CreateTask

**Write order:** Insert flow (`created`) → insert step (`created`) → update flow's `step_id`.

**Partial failure:** If the process crashes after inserting the step but before updating `step_id`, the flow has `step_id=0` with a `created` step. The flow is inert until `Start` is called. If `Start` is called on such a flow, it transitions the flow to `running` and all `created` steps to `pending` within a transaction, then enqueues via `step_id`. If `step_id=0`, no step is enqueued - but `pollPendingSteps` picks up the orphaned `pending` step when its lease expires.

**Verdict:** Self-healing.

#### Start / Resume

**Transaction:** Updates steps to `pending`, then updates the flow to `running`, atomically.

**Partial failure:** If the process crashes after committing the transaction but before enqueuing, the steps are `pending` and the flow is `running`. `pollPendingSteps` picks them up when their leases expire.

**Verdict:** Self-healing.

#### Retry

**Transaction:** Resets the failed step row in place (`status='pending'`, prior error cleared, park slot cleared, `attempt++`) and transitions the flow to `running`, atomically.

**Partial failure:** If the process crashes after committing the transaction but before enqueuing, the step is `pending` and the flow is `running`. `pollPendingSteps` picks it up when its lease expires.

**Verdict:** Self-healing.

#### Cancel

**Transaction:** Uses `surgraphChain` (up) and `allSubgraphFlows` (down) to collect all flows in the hierarchy. Cancels all steps across all flows, computes `final_state` for each, and cancels all flows - atomically in a single transaction.

**Partial failure:** If the process crashes before committing, nothing changes - the transaction rolls back. If it crashes after committing but before notifications, flows are correctly cancelled but `Await` callers may not be woken immediately; they'll discover the status on the next poll.

**Verdict:** Self-healing.

#### failStep

**Transaction:** Uses `surgraphChain` to collect all parent flows. Fails the current step, fails all surgraph steps in the chain, computes `final_state` for each flow, and fails all flows - atomically in a single transaction.

**Partial failure:** If the process crashes before committing, nothing changes - the transaction rolls back and the step's lease expires for re-execution. If it crashes after committing but before notifications, flows are correctly failed but `Await` callers may not be woken immediately.

**Verdict:** Self-healing.

#### processStep - Interrupt

**Transaction:** Uses `surgraphChain` to collect all parent flows. Interrupts all flows and steps in the chain, persists changes on the current step, and propagates the interrupt payload - atomically in a single transaction.

**Partial failure:** If the process crashes before committing, nothing changes - the transaction rolls back and the step's lease expires. Re-execution produces the interrupt again. The task may execute twice, so interrupt-producing tasks should be idempotent.

**Verdict:** Self-healing.

#### Resume

**Transaction:** Walks up (`surgraphChain`) and down (`interruptedSubgraphChain`). Clears interrupt payloads, re-parks surgraph steps, records resume data on the leaf step's `resume_data` column (not merged into `state`), resets the leaf to `pending`, and transitions all flows to `running` - atomically in a single transaction. After commit, re-propagates the next sibling's interrupt payload (if any) and enqueues the leaf step.

**Partial failure:** If the process crashes before committing, nothing changes - the transaction rolls back and the caller can retry `Resume`. If it crashes after committing but before enqueue, `pollPendingSteps` picks up the leaf step.

**Verdict:** Self-healing.

#### processStep - Normal Completion (with next steps)

**Write sequence:**
1. Step to `completed` (persists `changes` and `goto_next`)
2. Fan-in sibling check
3. Transaction: insert next steps + update flow's `step_id`
4. Enqueue next steps

**Partial failures:**
- Crash after (1) but before (3): the step is `completed` but no successor exists. This is a narrow window (~microseconds) and is acceptable for the simplification gained by removing the `completing` intermediate status.
- Crash after (3) but before (4): next steps exist and are `pending`. `pollPendingSteps` picks them up.

**Verdict:** Mostly self-healing. A crash in the narrow window after step completion but before the transaction leaves the flow stuck - an edge case acceptable for the simplification.

#### processStep - Flow Completion (no next steps)

**Write order:** Flow to `completed` (via `completeFlow`) → step to `completed`.

**Partial failure:** If the process crashes after completing the flow but before completing the step, the step remains `running`. Its lease expires, `pollPendingSteps` resets it to `pending`, and `processStep` picks it up. The terminal flow check detects `completed` and marks the step as `completed`.

**Verdict:** Self-healing.

#### pollPendingSteps

**Partial failure:** If the process crashes partway through recovery, the remaining work is picked up on the next poll cycle. Each individual recovery operation is independent.

**Verdict:** Self-healing.

### Database Sharding

The foreman supports distributing flows across multiple database shards to scale write throughput and reduce index contention. The `NumShards` config (default `1`) controls the number of shards.

**Shards are 1-indexed.** Valid shard indices are `1..NumShards`; `0` is reserved as a sentinel meaning "no shard / all shards" (used by `Query.Shard` to opt into all-shards mode). The DSN's `%d` substitution, the leading number in flow keys (`{shard}-{flowID}-{token}`), the `Query.Shard` filter, and the `ShardInfo` endpoint's index all use the same 1-based convention. Internally `svc.dbs` is a 0-based Go slice and `svc.shard(n)` translates: it bounds-checks `n >= 1 && n <= len(svc.dbs)` and returns `svc.dbs[n-1]`. Per-call slices indexed by shard (e.g. `succeeded []bool`, `perShard [][]row`) are sized `numShards+1` so they can be accessed directly with 1-based indices, leaving slot 0 unused; this avoids per-access `-1` translation noise in the hot path.

**Shard routing:** External flow IDs encode the shard number: `{shard}-{flowID}-{token}`. Every operation parses the shard from the flow ID and routes to the correct database connection via `svc.shard(n)`.

**Shard affinity:** Subgraph flows are created on the same shard as their parent. This avoids cross-shard references during active execution (subgraph completion) and history reconstruction. Only top-level flow creation picks a random shard.

**Polling and purging:** All replicas poll all shards for pending steps and purge expired flows across all shards. The atomic CAS update on step acquisition ensures correctness when multiple replicas race.

**Cross-shard fan-out is always parallel, never sequential.** Any operation that touches every shard builds a per-shard job slice and dispatches via `svc.Parallel(jobs...)`. Sequential `for s := 0; s < numDBShards(); s++` loops are forbidden because total latency would grow linearly with `NumShards`: at 8 shards a remote-DB query that costs 10ms per shard becomes 80ms wall-clock, large enough to push the refiller and the Prometheus scrape past their cadence and to lengthen `processStep` enough that worker pool throughput drops. The parallel shape stays at single-shard latency regardless of shard count, which is the load-bearing property of the sharding design.

**The foreman is not shard-fault-tolerant by design.** Every cross-shard fan-out site fails the whole call on any shard's error. A partial-tolerance attempt was considered and rejected: real outages mostly manifest as hangs, not errors, so the fanned-out call blocks until ctx timeout regardless of how the helper folds results; classifying which errors signal "shard down" vs. transient/data errors is driver-specific and brittle; and a helper that *claims* partial tolerance while only delivering it in a narrow subset of failure modes is worse than no tolerance at all because it lies to operators about the system's resilience.

The five cross-shard fan-out sites (`countTasks`, `pollPendingSteps`, `scanPriorityBand`, `OnObserveStepsPending`, `OnObserveStepsOldestPendingAgeSeconds`) share one helper, `svc.eachShard(ctx, op)` in `database.go`. The op is invoked once per shard with the resolved `*sequel.DB` and the 1-based shard index; any non-nil return fails the whole call via `svc.Parallel`'s first-non-nil semantics. The op's concurrency contract is "write only to slot N of any per-shard accumulator you captured" - disjoint-slot writes are safe without locking; shared aggregates (e.g. `pollPendingSteps`'s `nearestDelay`) need the op's own mutex. New cross-shard fan-out sites should use `eachShard`; sites with per-shard error reporting through their own API surface (`ShardInfo`) call `svc.Parallel` directly because they want every shard's result independently.

Each cross-shard caller retries on its next natural cycle: `pollPendingSteps` on the next timer tick, `scanPriorityBand` on the next refill, the metric observers on the next Prometheus scrape, `countTasks` on the next caller-driven invocation. A transient shard hiccup heals itself within one cycle; a persistent shard outage degrades the whole foreman, loudly. The trade is intentional: the foreman trusts the SQL connection pool to surface unhealthy connections as fast errors and the operator to keep shards reachable. Degraded behavior outside that contract is acceptable as long as it surfaces as operation-level errors in logs and metrics rather than silent data loss.

**`List` uses per-shard pagination, not cross-shard global order.** Each shard returns up to
`ceil(limit/numShards)` rows ordered by its own `flow_id DESC`; the aggregate is presented
shard-grouped. Sorting cross-shard by raw `created_at` would compare different SQL servers'
clocks (NTP drift), and sorting by computed age would fluctuate with per-shard network latency
to each query, so neither produces a stable order. Sorting by `flow_id` alone is also broken
because a shard with fewer flows has lower flow_ids than a shard with more - the original code's
"sort by FlowKey string descending" put shard 2's smallest flow above shard 1's largest. The
shard-grouped presentation gives up the global newest-first illusion in exchange for
deterministic order that doesn't wobble between calls; callers needing global newest-first can
sort client-side using a signal they trust. Pagination uses an opaque cursor: `ListOut.NextCursor`
encodes each shard's smallest-returned `flow_id` as `s=fid,s=fid,...`, and the next call passes
it back as `Query.Cursor`. A flow inserted on any shard after a page is returned has a `flow_id`
strictly higher than that shard's cursor and therefore does not appear on subsequent pages of an
in-flight pagination - fresh inserts only land on a fresh first page.

`List` is **strict by design**: any shard error fails the whole call, so a silent shard outage can't
hide as an incomplete result set. The "I need to debug the surviving shards" use case is served by the
composition of `ShardInfo` + `List(Shard=N)`: the operator calls `ShardInfo` to identify healthy shards
(it never fails - per-shard errors are reported in the per-shard entry), then `List(Shard=N)` against
each healthy shard. The per-shard path is the debug escape hatch; keeping the default strict prevents
silent shard-loss in normal use. Making the default partial-tolerant would only buy a silent-degradation
hazard that callers couldn't distinguish from a complete result.


**Dynamic expansion:** `NumShards` can increase at runtime via `OnChangedNumShards`. New shards are opened, migrated, and immediately available. Shrinking is rejected - old shards drain naturally as their flows complete.

**DSN format:** When `NumShards > 1`, the `SQLDataSourceName` must contain `%d` which is replaced with the shard index (0-based). In testing mode (SQLite), each shard gets a separate in-memory database via a unique test ID suffix.



## Flow Rendering (`foremanapi.FlowRenderer`)

`FlowRenderer` produces a Mermaid flowchart from a `History` response. Diagnostic intent: at a
glance answer "where did the time go in this flow?" The defaults render the brand palette in
top-down orientation; `With*Colors` swap palettes; `WithLinks` enables per-node click directives.

### CSS-variable theming model

Color values flow through `classDef` and `style` directives. The renderer does NOT emit a
themeVariables init block. Callers pass either hex literals (static rendering used by
`cmd/genworkflowmmd` and `HistoryMermaid`) or CSS custom-property references like
`var(--bespa-primary-container)` (host pages that want the diagram to track a page-level theme).
With `var()` values, browsers resolve the references through the SVG's CSS cascade, and the
diagram re-colors automatically on light/dark toggle without any renderer reinvocation. The
CSS-mode pattern is: pass `"currentColor"` for the fill and `""` (empty) for the text - the
generated classDef omits the `color:` property, host CSS sets `color` on the class for the shape
and `.nodeLabel` for the inner text, and Mermaid inherits via `currentColor`.

### Color knobs and status groups

Four palette pairs map to seven step statuses plus chrome:

| Pair | Statuses |
|---|---|
| primary | `completed`, `running` (running gets a dashed border) |
| secondary | `pending` + chrome (`_start`, `_end`, fan-out cohort wrappers, subgraph block fills) |
| error | `failed`, `cancelled` |
| attention | `interrupted` (intentionally distinct from error - "needs human eyes," not hard failure) |

### DAG-edge model

The execution DAG is reconstructed from `PredecessorID` / `SuccessorID` columns, NOT from
`step_depth`. Every edge is recorded on at least one endpoint - fan-out via each child's
`PredecessorID`, fan-in via each cohort exit's `SuccessorID`, linear on both - and the rendered
edge set is their deduped union, exact for arbitrary nesting.

### Subgraph caller decomposition

A subgraph caller step renders as **two** visual elements: the caller's task node (call site) and
a visible Mermaid subgraph wrapper block containing the recursively-rendered SubHistory. No
return-marker chrome between them.

The caller node's duration label is the **net** caller cost:

```
net = (caller.UpdatedAt - caller.StartedAt) - subgraph_wall_time
```

where `subgraph_wall_time = max(SubHistory*.UpdatedAt) - min(SubHistory*.CreatedAt)` walked
recursively. The net is the caller's own pre-call + post-return body time. It is independent of,
and additive with, the inner steps' own durations - total call cost reconstructs as
`net + subgraph_wall_time` without either number double-counting the other. The label is omitted
when the caller or any inner step is in-flight (subgraph wall time is not yet defined).

Edges thread:

```
predecessor --> caller         (parent DAG, transition gap label)
caller --> innerHead           (the call, queue-wait label)
innerTail --> Y.entries        (the return, transition gap label)
```

`byID[caller].exits` is set to the subgraph's inner tails, so the existing `addEdge(caller, Y)`
machinery naturally emits one edge per inner-tail × parent-DAG-successor combination - covering
whichever shape the caller's transitions produced (static fan-out, forEach, withWhen, withGoto).
The transition-gap label is computed from the step records (`Y.StartedAt - caller.UpdatedAt`)
regardless of which inner tail the edge visually originates from. A terminal subgraph caller (no
parent-flow successor) surfaces its inner tails as outer tails, and `Render` connects them to
`_end`.

### Node and edge label semantics

**Node label** = `UpdatedAt - StartedAt` (task body time) for any non-caller step that has run
and reached a terminal status. Pending/created/in-flight steps render with just the task name.
Subgraph callers render the *net* cost (above), not body time.

**Edge label** = `to.StartedAt - from.UpdatedAt` (transition gap: DB commit + queue + dispatch).
Computed from the step records, not from the visual src/dst.

**Call edge label** = `entry.StartedAt - entry.CreatedAt` (queue wait on the subgraph's entry
step). Without this label, the time would be inside `subgraph_wall_time` but invisible on any
rendered edge, and the right-path arithmetic wouldn't reconcile with sibling fan-in waits. With
the label, every visible interval in the diagram adds up to the wall clock.

### Fan-out cohort wrappers

Two-or-more steps sharing one `PredecessorID` get wrapped in an invisible Mermaid `subgraph`
block (empty label, no fill, no stroke). Purely a layout container so siblings cluster near
their parent instead of stretching across the full diagram width. Edges always go between actual
task nodes; nothing terminates at the cohort wrapper.

### Truncation in label formatting

`formatDuration` uses integer-millisecond truncation for sub-second values (`%dms` from
`d.Milliseconds()`). A diagram with N labeled edges accumulates up to ~N/2 ms of systematic
underestimation in any path sum. Diagnostically irrelevant (the goal is to spot where time went,
not to reconcile to the millisecond); if exact reconciliation ever matters, switch sub-second
formatting to `%.1fms`.

## Agent Instructions

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Overview

The foreman is the orchestrator for agentic workflows in Microbus. It executes workflow graphs by dispatching tasks to microservices, managing state between steps, and handling fan-out/fan-in, interrupts, retries, and failure recovery.

### Core Concepts

**Workflow graph** - A directed graph that defines the structure of a workflow: which tasks to execute, in what order, and under what conditions. Graphs are defined in code using the `workflow.Graph` API and served as JSON via a dedicated endpoint. Each graph has a name, an entry point, a set of tasks, transitions between them, and optional reducers for fan-in state merging.

**Task** - An addressable endpoint on the bus (port `:428`) that performs a unit of work within a workflow. Tasks receive state via a `workflow.Flow` carrier, read input from state fields, perform work, and write output back to state fields. Tasks are reusable across workflows and can live on any microservice.

**Flow** - A single execution of a workflow graph. Each flow has a unique ID, tracks its current position in the graph, and maintains a state map that evolves as tasks execute. Flows progress through statuses: `created` → `running` → `completed`/`failed`/`cancelled`, with `interrupted` as a parked state for human-in-the-loop scenarios.

**Step** - A single task execution within a flow. Each step captures an immutable state snapshot (input), the changes produced by the task (output), and metadata like status, error, and timing. Steps are numbered sequentially (`step_depth`); parallel fan-out siblings share the same `step_depth`. Once a step reaches a terminal status (`completed`, `failed`, `retried`, `cancelled`), it is immutable. Retry creates a new step rather than modifying the failed one.

**Reducer** - A merge strategy for state fields during fan-in. When parallel branches converge, each branch's changes are merged using the reducer chosen for that field: `replace` (last write wins, default), `append` (concatenate arrays), `add` (sum numbers), `union` (deduplicate arrays), or `merge` (combine objects, new key wins). Reducers are normally selected by the field name's prefix (`sum*`, `list*`, `set*`); `graph.SetReducer` is the escape hatch for fields whose names don't follow the convention.

**Thread** - A chain of flows linked by `Continue`. Each flow has a `thread_id` column that groups it with other flows in the same multi-turn conversation. By default, `thread_id = flow_id` (each flow is its own thread). When `Continue` creates a new flow, it inherits the thread's `thread_id`. Forked flows and subgraph flows always start their own thread (`thread_id = flow_id`) to avoid contaminating the original thread's continuation chain. The flowKey returned by the initial `Create` doubles as the threadKey - callers can pass it (or any flowKey in the chain) to `Continue` or `List` to address the thread.

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
                           │ Retry
                           ▼
                         running ──► ...

                         cancelled (via Cancel)
```

1. **Create** (or `CreateTask`) inserts a flow and its first step in `created` status
2. **Start** transitions the flow to `running` and its steps to `pending`, then enqueues for execution
3. A worker picks up the step, executes the task, and evaluates transitions to create next steps
4. This repeats until no transitions match (flow completes), a task returns an error (flow fails), or the flow is cancelled
5. Tasks can call `flow.Interrupt()` to pause for external input; `Resume` continues execution
6. Failed flows can be retried with `Retry`, which re-executes the last failed step

### Foreman Operations

**Create** - Creates a new flow without starting it. Fetches the workflow graph via a direct GET to the workflow's endpoint URL, creates the flow row, and inserts the entry-point step - both in `created` status. The graph JSON is frozen at creation time. `CreateTask` is a convenience variant that accepts a single task name and wraps it in a trivial graph.

**Start** - Transitions a `created` flow to `running`. Atomically updates all `created` steps to `pending` and the flow to `running` within a transaction, then enqueues the current step for execution. Does not set up notifications.

**StartNotify** - Like `Start`, but also stores a `notify_hostname` on the flow. When the flow stops (completed, failed, cancelled, or interrupted), the foreman fires an `OnFlowStopped(flowKey, status, snapshot)` event targeted at that hostname. The subscriber must use `foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(handler)` to receive these events.

**Snapshot** - Returns the current state, status, task name, and step number of a flow. During fan-out (`step_id=0`), it queries the microbus_steps table directly to find the active steps.

**Resume** - Continues an interrupted flow. Walks up the surgraph chain (`surgraphChain`) and down the interrupted subgraph chain (`interruptedSubgraphChain`) to find the leaf interrupted step. Merges resume data into the leaf step's state (not changes) so the task sees it via `ParseState`. Re-parks intermediate surgraph steps, resets the leaf to `pending`, and transitions all flows in the chain to `running`. Can be called on any flow in the chain (root or subgraph) - propagation goes both directions. If multiple fan-out siblings interrupt, each `Resume` handles one sibling; the flow only transitions to `running` when no more interrupted steps remain.

**Fork** - Creates a new flow from an existing flow's checkpoint at a given `step_depth`. The forked flow inherits the parent's graph, config, and actor claims. The step at the fork point is re-created with the merged state (state + changes) plus any overrides, in `created` status. The caller must call `Start` to begin execution. Lineage is tracked via `forked_flow_id` and `forked_step_depth`.

**Cancel** - Aborts a created, running, or interrupted flow. Uses `surgraphChain` to walk up to the root and `allSubgraphFlows` to walk down to all descendants. Atomically cancels all steps across all flows, computes `final_state` for each flow, and cancels all flows with per-flow `final_state` via CASE - all within a single transaction. Can be called on any flow in the chain (root or subgraph).

**Retry** - Re-executes the last failed step. Marks the failed step as `retried` (preserving its error in history), creates a new copy of the step as `pending`, and transitions the flow to `running`. If the flow had fan-out, all failed siblings are retried. The `retried` status is immutable - it serves as an audit trail of previous attempts.

**History** - Returns the step-by-step execution history as a slice of `FlowStep` records. Each `FlowStep` includes the step's key, depth, task name, state, changes, status, error, and timestamp. Steps that executed a subgraph have `Subgraph=true` and their `SubHistory` field contains the nested execution history of the subgraph flow. For forked flows, reconstructs the full lineage by walking `forked_flow_id` up to the root and querying steps across all ancestor flows with bounded `step_depth` ranges.

**List** - Queries flows by status, workflow name, or thread key. Supports cursor-based pagination via `cursorFlowKey` (newest first). Defaults to 100 results. Returns `ThreadKey` in each `FlowSummary`. When filtering by `ThreadKey`, only flows in that thread are returned (scoped to the thread's shard).

**BreakBefore** - Sets or clears a breakpoint that pauses execution before the named task runs. Breakpoints are stored in a `breakpoints` JSON column on the flow row as a `map[string]string` keyed by task name. During `processStep`, if the current task name matches a breakpoint and the step's `breakpoint_hit` flag is false, the foreman interrupts the flow (using the same interrupt propagation as `flow.Interrupt()`) and sets `breakpoint_hit=1` on the step. The flag prevents the breakpoint from re-triggering when the flow is resumed. Breakpoints are inherited by subgraph flows and forked flows.

**Continue** - Creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers. The `threadKey` parameter accepts any flowKey belonging to the thread - `Continue` resolves the thread via the flow's `thread_id` column, finds the latest flow in that thread (`ORDER BY flow_id DESC`), validates it is completed, and creates the new flow in the same thread. The new flow uses the same workflow graph and is returned in `created` status. This enables multi-turn patterns where each turn is a separate flow: the caller holds a single threadKey (the flowKey returned by the initial `Create`) and calls `Continue` repeatedly without needing to track intermediate flowKeys. Fields that need to persist across turns must be declared in both `DeclareInputs` and `DeclareOutputs`.

**Enqueue** - Internal endpoint (port `:444`) that rings the work **doorbell**: it signals that a step is pending and lets the receiving replica decide, from the step's priority relative to its candidate cache, whether to refill or to flush and refill (see "Execution Model"). It does not enqueue a specific step. Load-balanced across foreman replicas. Fire-and-forget - errors are ignored because `pollPendingSteps` rings the local doorbell each cycle and recovers any missed work.

### Flow Stop Notifications

When a flow is started via `StartNotify(flowKey, notifyHostname)`, the foreman stores the hostname and fires `OnFlowStopped(flowKey, status, snapshot)` events targeted at that hostname when the flow stops - i.e. reaches a terminal status (`completed`, `failed`, `cancelled`) or is `interrupted`. This matches the statuses that `Await` returns on.

For terminal statuses, the snapshot contains the flow's `final_state`. For `interrupted`, the snapshot is nil - the caller should use `Snapshot()` to read the current state including the interrupt payload.

The notification is fire-and-forget - flow execution never blocks on notification delivery. The `notify_hostname` is set only on the root flow (via `StartNotify`); subgraph flows do not receive direct notifications. Interrupt notifications query the root flow's hostname from the surgraph chain.

Subscriber pattern:
```go
foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(svc.OnFlowStopped)
```

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
it is a **doorbell** (`candidateCache.offer`). It resolves the announced step's priority by a PK lookup
(off the selection path), then takes one of three paths. (1) Empty cache: this replica is idle - request a
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
cycle. If a due-pending backlog exists it caps the next poll at `backlogPollInterval` (2s) so an idle replica
that received no doorbell still re-scans promptly - the poll-driven liveness net the old enumerate-and-push
model provided, without per-step queue stuffing.

The refiller/`MaxClaims` slot and the crash-recovery lease are orthogonal: the slot is an in-memory admission
gate on running the selection SQL; the lease is the DB-side crash-recovery timer on the `running` step.

A **timer loop** (`timerLoop`) runs `pollPendingSteps` on the adaptive interval (nearest future `not_before`,
the 2s backlog cap, or `maxPollInterval`).

### Query Parallelism

`processStep` is the hot path - it runs for every task in every workflow. To minimize latency when the database is remote, independent queries within `processStep` are executed in parallel using `svc.Parallel`:

- **Step data + flow data** - after acquiring the lease, the step and flow metadata queries run concurrently (the initial query fetches `flow_id` alongside `time_budget_ms` to enable this)
- **Fan-in sibling counts** - the unfinished and failed sibling COUNT queries run concurrently
- **Subgraph status counts** - the active and completed subgraph COUNT queries run concurrently

Outside the hot path, `completeSurgraphFlow` and `surgraphChain` also parallelize independent queries.

**Transaction constraint:** Functions that receive a `sequel.Executor` parameter (which may be a transaction) cannot use `svc.Parallel` because SQL transactions are not safe for concurrent use. This applies to `computeFinalState` and any code running inside `failStep` or `Cancel` transactions.

### Fan-Out and Fan-In

**Static fan-out** occurs when multiple transitions match from a single task. All target tasks execute in parallel, sharing the same `step_depth`. The flow's `step_id` is set to `0` to indicate fan-out.

**Dynamic fan-out** uses `forEach` on a transition to iterate over a state array and spawn one task instance per element. Each instance receives the element under the `as` key. If the array is empty, no tasks are spawned for that transition. When a `forEach` transition is the only outgoing transition from a task, an empty array causes the flow to complete at that point - downstream tasks (including the fan-in target) are never reached.

**Fan-in** is implicit. When the last sibling at a `step_depth` completes, the foreman merges all siblings' changes using reducers and creates the next step(s) within a transaction. The transaction prevents duplicate next steps when multiple workers finish siblings simultaneously.

**Fan-in does not escalate on cancelled or failed siblings.** If a sibling at the current depth is in `failed` or `cancelled` status when fan-in evaluates, the flow is already being driven by another path: a sibling's `failStep` has cascaded the flow to failed, an external `Cancel` has cancelled it, or an `OnError` sibling-cancel has handed the depth off to an error handler. The fan-in worker bails out with `return nil` instead of calling `failStep` on its own step. Calling `failStep` here races with the OnError handler and incorrectly fails an otherwise-recoverable flow: the OnError handler's next step (e.g. an error-routed alternative path) is in flight at depth `N+1` while the fan-in worker is still finishing depth `N`, and a redundant `failStep` from the fan-in worker would mark the parent flow failed before the OnError path ever runs.

**Fan-in merge order and contribution (lineage `SetFanIn` path).** `insertFanInStep` reads cohort members
(`lineage_id = cohortSpawnID`) `ORDER BY fan_out_ordinal, step_id`. `fan_out_ordinal` is stamped on each branch at
fan-out from its position in the spawn loop, which is the `forEach` array index (or static fan-out declaration order),
so `list`/`append`/`sum`/`set` reducers fold in input-array order rather than non-deterministic completion order;
`step_id` only breaks ties. The firing gate is unchanged - fan-in still fires when `cohort_arrivals >= cohort_size`,
which is a counter on the spawn step independent of the merge query, so the merge's status filter cannot deadlock
fan-in. Only members in `completed` status contribute their `changes`; `failed`/`cancelled`/`retried`/`pending`/
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

**Retry rejoins its cohort.** The API-level `Retry` insert copies the failed step's `lineage_id` and
`fan_out_ordinal` onto the new pending step. Without the `lineage_id` copy the retried step would orphan from its
fan-in cohort (its `lineage_id` would default to 0) and its output would be silently dropped from the merge; the
`fan_out_ordinal` copy keeps the retried branch in its original list position. The immutable `retried` row is excluded
from the merge by the `completed`-only filter, so the retry does not double-count under append/sum/union/merge.

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
- **Retry**: the new step copies the failed step's `predecessor_id` so it slots back into the DAG in place.
- **Entry / subgraph-entry / fork-entry steps**: `predecessor_id` defaults to 0 (no in-flow predecessor).

`renderMermaidSteps` ignores `step_depth` and `lineage_id` entirely: it draws the deduped union of
`{predecessor_id -> step}` and `{step -> successor_id}`, which is exact for arbitrary nesting (validated by
`mermaid_test.go` for the per-element pipeline and `forEach`-of-chain shapes). Heads are nodes with no incoming
edge, tails are nodes with no outgoing edge. Subgraph steps still expand inline via the `SubHistory` recursion;
edges into/out of a subgraph step attach to its enter/exit markers. Cross-flow fork seams are not wired (the
forked entry has `predecessor_id=0`), so a forked flow renders as its own connected sub-DAG from `_start` rather
than stitched to its ancestor - acceptable and arguably clearer than the old depth hack.

`computeFinalState` also reads the DAG, not `step_depth`. The flow's terminal state is the merge of the tail
steps - completed steps with `successor_id = 0` (`mergeTerminalSteps`). The earlier `MAX(step_depth)` heuristic
was wrong for any graph where an intra-thread `flow.Goto` self-loop sits inside a fan-out: each loop iteration
pushes `step_depth + 1`, so the looping branch can outrun the fan-in/terminal step in depth. When `normalC`
(shallow branch) is the last cohort member to arrive, the fan-in `taskD` is created at `normalC.step_depth + 1`,
which is *less* than the dangling final loop iteration's depth, so `MAX(step_depth)` selected the loop step
(no `finalResult`) and `DeclareOutputs` filtering then yielded `{}`. The bug was order-dependent (only when the
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

The worker lease duration is `time_budget + leaseMargin` (30s), i.e. `TimeBudget` ceiling +
`leaseMargin`. This ensures the lease always outlasts execution, preventing premature recovery by
`pollPendingSteps`. Accepted trade-off: because the lease is sized to the ceiling rather than per task, a
*crashed* worker that was running a task with a tight `sub.TimeBudget` is recovered no sooner than
`ceiling + leaseMargin` instead of `tightBudget + leaseMargin`. The common case is unaffected -
a hung (not crashed) task is cancelled at its declared bound by the connector, its outbound call returns,
and the foreman acts immediately; only the rare worker-crash path pays the slower recovery, which is
acceptable for the simplification of a single uniform lease.

`sub.TimeBudget` is endpoint self-protection, not upstream propagation: the foreman never learns a task's
declared budget, and nothing on the wire carries it back. A responsive-but-slow task is cancelled at its
declared bound, returns promptly, and the foreman acts at once - so the dispatch call effectively completes
in `declared budget + one network hop`, far inside the foreman's ceiling. A task that never responds at all
(a crashed or wedged replica) is bounded solely by the foreman's `TimeBudget` ceiling on the `pub.Timeout`
of the dispatch call, which is exactly the hung-downstream behavior that predates `sub.TimeBudget`. The
option therefore adds no new dispatch-latency hazard; it only lets an endpoint bound its own handler.

### State Model

Each step has three JSON columns: `state` (input snapshot), `changes` (output delta produced by task execution), and `interrupt_payload` (data from `flow.Interrupt(payload)`). The `state` column is set at step creation and is normally immutable. The `changes` column is written after task execution.

The next step's `state` is computed as `merge(currentState, changes)` - the merged result overwrites matching fields. This immutability model enables checkpointing, fork, and recovery: the foreman can always reconstruct a step's output by reading its `state` + `changes`.

**State mutation on retry and resume:** The `state` column is updated in two cases: (1) on `flow.Retry()`, the foreman merges `state + changes` back into the `state` column so the task sees its own prior output on the next attempt; (2) on `Resume`, the resume data is merged into the leaf step's `state` column so the task sees the caller-provided data via `ParseState`. In both cases, `changes` is preserved as-is.

**Reducer delta convention:** Tasks writing to reducer-managed fields (append, add, union, merge) must set only the **delta**, not the full accumulated value. For example, if `listMessages` uses the append reducer (auto-selected by the `list*` prefix), the task should set `flow.Set("listMessages", []string{newMessage})` - not the entire message history. Violating this causes duplicates during fan-in merge.

**forEach element injection:** When a `forEach` transition spawns task instances, the current element is injected into `state` only (under the `as` key), not into `changes`. This makes the element available to the task but prevents it from participating in fan-in merge.

### Task-Initiated Control Signals

Tasks can signal the foreman via control methods on the `Flow` carrier. These are distinct from the API-level operations:

- **`flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay) bool`** - Requests the foreman to re-execute this task with exponential backoff. Returns `true` if a retry will be scheduled (attempts remaining), `false` if exhausted. When `true`, the task should return `nil`. When `false`, the task should return the actual error. The step row is reused (not a new row). The foreman tracks the attempt count in the step's `attempt` column and computes the delay as `min(initialDelay * multiplier^attempt, maxDelay)`. Changes from this execution are preserved, and the foreman merges `state + changes` back into the step's `state` column so the task sees its own prior output on the next attempt via `ParseState`/`GetInt`/etc. This is different from the API-level `Retry`, which creates a new step to replace the failed one.
- **`flow.RetryNow() bool`** - Shorthand for `flow.Retry(math.MaxInt32, 0, 0, 0)` - retries immediately with no limit. Always returns `true`. The task itself is responsible for deciding when to stop retrying.

**No jitter on retry backoff:** The foreman does not add jitter to the computed backoff delay. The worker pool (`Workers` config, default 64) already throttles concurrency per replica - even if multiple retries fire simultaneously, they queue up in the worker pool rather than overwhelming downstream resources. Adding jitter would increase latency for no throughput benefit in this architecture.
- **`flow.Sleep(duration)`** - The task tells the foreman to delay the next step's execution. Sets the next step's `not_before` timestamp. Useful for rate-limit delays. The timer loop adapts its poll interval to wake up when the sleep expires. Note: sleep delays the *transition to the next step*, not the current step's re-execution. In fan-out, only the last sibling's sleep affects the fan-in point.
- **`flow.Goto(target)`** - Overrides transition routing. The foreman skips normal condition evaluation and follows the `withGoto` transition to the specified target, if one exists in the graph. Goto transitions are never taken during normal evaluation - only when explicitly requested.
- **`flow.Interrupt(payload)`** - Pauses execution and parks the flow. The payload is stored in the step's `interrupt_payload` column and propagated up the surgraph chain. The task should return normally after calling Interrupt. The foreman sets the flow to `interrupted` and fires `OnFlowStopped` on the root flow's `notify_hostname`.

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

State crosses the subgraph boundary asymmetrically.

**Into the child:** The parent's full state plus the surgraph step's accumulated changes is computed first. For dynamic subgraphs (`flow.Subgraph(url, input)`), the explicit input map is then merged on top using the *child* graph's reducers. The result is filtered through the *child's* `DeclareInputs` before becoming the child flow's initial state.

**Back to the parent:** The child's `final_state` is filtered through the *child's* `DeclareOutputs` and merged into the surgraph step's `changes` using the *parent* graph's reducers.

So both `DeclareInputs` and `DeclareOutputs` shape the subgraph contract, and the child graph's reducers govern only the input merge while the parent graph's reducers govern only the output merge.

### Surgraph Step Identification

Each subgraph flow's `microbus_flows` row stores `surgraph_flow_id`, `surgraph_step_depth`, *and* `surgraph_step_id` - the primary key of the parked surgraph step it belongs to. `completeSurgraphFlow` looks the surgraph step up by primary key, so it can never match a sibling at the same `(flow_id, step_depth)`.

This matters in two scenarios that broke earlier (when the lookup was by `flow_id + step_depth + status='running'` only):

1. **Fan-in race:** while `completeSurgraphFlow` is running, a non-subgraph sibling at the same step_depth might be momentarily in `running` status (e.g. mid-execution). The legacy lookup could match that sibling and corrupt its row.
2. **Parallel subgraphs at the same step_depth:** if a fan-out produces multiple subgraph siblings, every parked surgraph step at the depth has a far-future lease, so a lease-threshold filter can't disambiguate them either.

The `surgraph_step_id` PK lookup eliminates both cases. A lease-threshold fallback (5+ minutes lease remaining = parked) is kept for legacy `microbus_flows` rows created before the column existed (`surgraph_step_id = 0` sentinel). Migration `3.sql` adds the column.

The `IsSubgraph` existence check (`activeSubgraphCount` / `completedSubgraphCount`) in `processStep` is also scoped by `surgraph_step_id`, so parallel subgraph siblings can't see each other's child flows and falsely conclude that "a subgraph already exists" - without this scope, the second sibling would park forever waiting for a subgraph that was never created on its behalf.

### Interrupt/Resume Propagation Across Subgraphs

**Interrupt propagation (up):** When a step inside a subgraph flow is interrupted (breakpoint or `flow.Interrupt()`), the foreman uses `surgraphChain` to walk from the subgraph flow up to the root surgraph, collecting all flow IDs and parked surgraph step IDs. It then interrupts all flows and steps in the chain with bulk `UPDATE ... WHERE flow_id IN (...)` and `UPDATE ... WHERE step_id IN (...)` statements. This ensures the caller awaiting the top-level flow ID sees the `interrupted` status.

**Resume propagation (both directions):** When `Resume` is called on any flow in the chain, the foreman uses `surgraphChain` to walk up and `interruptedSubgraphChain` to walk down. It re-parks intermediate surgraph steps (restore to `running` with far-future lease), merges resume data into the leaf step's state (so the task sees it via `ParseState`), resets the leaf to `pending`, transitions all flows in the chain to `running`, and enqueues the leaf step. All writes are in a single transaction.

**Fan-out interaction:** During fan-out, one sibling may interrupt while others continue running. The flow is marked `interrupted` by the first sibling, but other siblings run to completion. `Resume` handles one interrupted sibling at a time; the flow only transitions to `running` when no more interrupted steps remain at any level in the chain.

### Actor Identity Propagation

The actor's JWT claims are captured at `Create` time and stored in the flow's `actor_claims` column. Before each task execution, the foreman mints a fresh access token from these claims and attaches it to the request via `pub.Token`. This ensures tasks run with the original caller's identity even when executed long after the flow was created.

### Await

`Await` blocks until a flow stops (i.e. is no longer `created`, `pending`, or `running`). It returns on `completed`, `failed`, `cancelled`, or `interrupted`. It registers a buffered channel in the `waiters` map, then loops: check `State`, return if stopped, otherwise `select` on the channel or context cancellation. Non-terminal notifications (e.g. `running` from `Start`) cause the loop to re-check `State` rather than returning early. This is important because `NotifyStatusChange` fires for all status transitions, not just terminal ones - without the loop, a `running` notification could consume the channel's buffer and cause `Await` to return prematurely with an empty status.

### SQLite Testing Support

When `SQLDataSourceName` is empty (default in `TESTING` deployment), the foreman uses an in-memory SQLite database via `sequel.OpenTesting`. Key differences from server-based databases:

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

### Flow Scheduling (priority / fairness)

`8.sql` adds `priority`, `fairness_key`, `fairness_weight` to **both** `microbus_flows` (authoritative) and
`microbus_steps` (denormalized). The flow row is the source of truth; the step copy exists so the two-level
selection query never has to join `microbus_flows` on the hot path - the same split already used for
`time_budget_ms`/`actor_claims`.

`resolveFlowOptions` resolves a caller's `*workflow.FlowOptions` against the foreman defaults: priority falls
back to the `DefaultPriority` config, the fairness key falls back to the `tid`/`tenant` actor claim then the
`""` bucket, and the weight falls back to `1`. The three values are immutable for the life of a flow (no
`Reprioritize` in v1; switching a flow's fairness key mid-run would be a self-promotion abuse vector).
`Create`/`CreateTask` resolve from options/claim; `Continue`, `Fork`, and `createSubgraphFlow` **inherit** the
parent/thread flow's three values (read from its row) rather than re-resolving, so a high-priority parent never
silently spawns default-priority descendants.

Propagation onto step rows takes one of two shapes, both correct because the flow's values are immutable and
authoritative:

- Where the resolved values are already in hand (entry step in `createWithGraph`, the `Fork` step, the API
  `Retry` insert which copies them off the failed step row like `lineage_id`), they are written as literal
  bind parameters.
- In the deep `processStep` paths (next-step fan-out and the two fan-in inserts), the values are **not**
  threaded through the call chain; the insert copies them straight from the owning flow with a scalar
  subquery `(SELECT priority FROM microbus_flows WHERE flow_id=?)` (and likewise for the other two). The flow
  row always exists before any of its steps, so this is single-source-of-truth with zero new function
  parameters. The subquery is a PK lookup, off the hot read path (it runs at step creation, not selection).

The scheduler *reads* `priority` and `fairness_key`/`fairness_weight` via the two-level selection in the
refiller (see "Execution Model"), backed by `idx_microbus_steps_selection`. `fairness_weight` is read only
from each key's oldest candidate step. `idx_microbus_steps_saturation` is created but not yet read: the
per-task `MaxTaskConcurrency` saturated-set exclusion is not wired into selection (a separate future
backpressure deliverable).

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
  pressured downstream is a separate, deferred per-task-concurrency deliverable, not a function of pool size.
- **Completion writes are deliberately *not* gated by the refiller/`MaxClaims` slot.** That slot bounds
  selection only; finishing in-flight work must outrank starting new work, so the post-execution advance
  (persist changes, transitions, fan-in) is never serialized behind selection. The real contention axis is
  same-flow fan-in, already serialized by the fan-in transaction plus the lock-contention rewind path; the
  `CompletionContention` counter exists to decide if an explicit completion bound is ever warranted.

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
| `forked_flow_id` | Ancestor flow id if this flow was produced by `Fork`; `0` otherwise |
| `forked_step_depth` | The ancestor `step_depth` the fork was taken from |
| `surgraph_flow_id` | Parent (surgraph) flow id if this is a subgraph flow; `0` otherwise |
| `surgraph_step_depth` | The parent's `step_depth` that spawned this subgraph |
| `surgraph_step_id` | (`3.sql`) PK of the parent's parked surgraph step, so a subgraph flow identifies its surgraph step unambiguously when parallel parked surgraph steps coexist at one depth. `0` = legacy row, falls back to the lease-threshold search |
| `thread_id` | Groups flows in a `Continue` thread; defaults to `flow_id` (each flow its own thread) |
| `thread_token` | Token component of the thread's flowKey |
| `trace_parent` | W3C `traceparent` for distributed-trace continuity across the flow |
| `notify_hostname` | Set by `StartNotify`; `OnFlowStopped` fires here when the flow stops. `''` = no notification |
| `final_state` | JSON state computed at termination, filtered through the graph's `DeclareOutputs` |
| `breakpoints` | JSON `map[taskName]string` of `BreakBefore` breakpoints |
| `created_at` | UTC creation time. Append-only and PK-correlated; the `PurgeExpiredFlows` cursor heuristic and `idx_*_created_at` rely on this |
| `updated_at` | UTC time of the last status transition; bumped on every state change |
| `priority` | (`8.sql`) Scheduling priority, integer >= 1, lower runs first. Resolved at `Create` from `FlowOptions` (an explicit priority is >= 1; a 0/unset `FlowOptions.Priority` falls back) else the `DefaultPriority` config; inherited unchanged by `Continue`/`Fork`/subgraph. Immutable after creation |
| `fairness_key` | (`8.sql`) Fairness bucket, typically a tenant. From `FlowOptions`, else the `tid`/`tenant` actor claim, else `''`. Immutable after creation |
| `fairness_weight` | (`8.sql`) Relative dispatch share of the `fairness_key`. From `FlowOptions`, else `1` |

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
| `interrupt_payload` | JSON payload from `flow.Interrupt()` |
| `status` | Step lifecycle: `created`/`pending`/`running`/`interrupted`/`completed`/`failed`/`retried`/`cancelled` |
| `goto_next` | Task-requested `flow.Goto` target; `''` = none |
| `error` | Error text when `failed`; `''` otherwise |
| `time_budget_ms` | Execution budget; the HTTP timeout on the task call |
| `breakpoint_hit` | `1` once a breakpoint on this step has fired, so it does not re-trigger on resume |
| `attempt` | Task-initiated retry (`flow.Retry`) attempt counter, drives the backoff |
| `not_before` | Earliest UTC time the step may execute (`flow.Sleep` / retry backoff) |
| `lease_expires` | Crash-recovery lease; `pollPendingSteps` reclaims `running` steps past this |
| `created_at` | UTC creation time |
| `updated_at` | UTC time of the last status transition |
| `lineage_id` | (`5.sql`) Cohort frame: the spawn step's `step_id` on a push, else inherited. Drives explicit `SetFanIn` arrival counting and the merge. A cohort-counting device, **not** a DAG. `0` = no `SetFanIn` |
| `cohort_size` | (`5.sql`) On a fan-out spawn step: number of branches spawned |
| `cohort_arrivals` | (`5.sql`) On a fan-out spawn step: branches that reached the fan-in; fan-in fires when `arrivals >= size` |
| `fan_out_ordinal` | (`6.sql`) This branch's index in its fan-out (forEach array index / static declaration order); fan-in merges in this order so list/sum reducers are deterministic. Retry copies it. `0` = not part of a fan-out |
| `predecessor_id` | (`7.sql`) Step that ran immediately before this one in the execution DAG. `0` = none (entry/subgraph-entry/fork-entry) |
| `successor_id` | (`7.sql`) Step that runs immediately after this one. `0` = none (exit) |
| `priority` | (`8.sql`) Denormalized copy of the owning flow's `priority` for the hot selection path. Copied at every step-insert path so selection never joins `microbus_flows`. Read by the two-level selection |
| `fairness_key` | (`8.sql`) Denormalized copy of the flow's `fairness_key` |
| `fairness_weight` | (`8.sql`) Denormalized copy of the flow's `fairness_weight` |

## Database Indexing Strategy

The foreman's `microbus_flows` and `microbus_steps` tables grow indefinitely as workflows execute. The indexing strategy is designed to keep the hot-path queries fast without introducing fragmentation or excessive write amplification.

### Design Principles

1. **Append-only terminal sections.** Indexes that include `status` as the leading column naturally partition the B-tree by status value. Terminal statuses (`completed`, `failed`, `cancelled`) are append-only because entries arrive with a monotonically increasing `updated_at` timestamp (always `NOW_UTC()` at the time of the status transition). This means the terminal sections of the B-tree stay well-ordered - no mid-tree page splits, no fragmentation.

2. **Small transient sections.** The `pending` and `running` sections of status-based indexes churn as steps are created and processed, but they remain small (proportional to active work, not historical volume). Page reuse within these sections is efficient because the working set fits in a few leaf pages.

3. **Partial indexes for PostgreSQL.** Where only non-terminal statuses are queried through an index (e.g., `pollPendingSteps`), PostgreSQL uses a partial index filtered to `status IN ('pending', 'running')`. This keeps the index tiny regardless of table size. MySQL and SQL Server use the full composite index since they lack partial index support.

4. **PK-ordered scans for bulk operations.** The `PurgeExpiredFlows` ticker walks the `microbus_flows` table by primary key in ascending order, using `created_at` as a cursor-stop heuristic (since PK order correlates with creation time). This avoids full table scans and unindexed column filters.

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
| `idx_microbus_steps_selection` | `(status, priority, fairness_key)` (migration `8.sql`) - partial `WHERE status IN ('pending', 'running')` on pgx/mssql/sqlite, full on mysql | Two-level priority+fairness candidate selection. Read by the refiller (band MIN(priority), then candidate rows by key) |
| `idx_microbus_steps_saturation` | `(status, task_name)` (migration `8.sql`) - partial as above | Per-task max-concurrency saturated-set aggregate. Created but not yet read (reserved for a future backpressure deliverable) |

### Data Retention

The `PurgeExpiredFlows` ticker runs every 24 hours and deletes terminated flows older than `RetentionDays` (default 0 = disabled). It uses a hybrid approach:
- Scans `microbus_flows` by PK in batches of 1000 (PK-ordered, no full scan)
- Stops when `created_at` exceeds the retention cutoff (PK-order heuristic). This assumes auto-increment IDs correlate with creation time, which holds per shard since each shard has an independent auto-increment sequence. Flows are never migrated between shards.
- Protects recently-active flows by also checking `updated_at` (a flow resumed yesterday won't be purged even if created months ago)
- Deletes steps first (by `flow_id` index), then flows

## Concurrency and Crash Recovery

The foreman uses SQL transactions for multi-statement operations and `lease_expires` for crash recovery.

### Worker context: `rootCtx`, not `OnStartup`'s ctx and not `Lifetime()`

All worker, timer, and refiller database operations run on `svc.rootCtx`, a foreman-owned context created
(`context.WithCancel(context.Background())`) at the top of `OnStartup` and cancelled in `OnShutdown`
*after* `svc.workers.Wait()` has drained the pool. It is deliberately **not**:

- `OnStartup`'s `ctx` argument - that ctx is request/startup-scoped; if it were cancelled while a
  flow was mid-flight, a fan-in/step/cohort write would abort and strand the flow in `running`
  (no fan-in fires, `Await` waits forever). This was the suspected cause of an intermittent
  "deadlock" report; the real culprit turned out to be a too-short synchronous `Run`/`Await`
  request timeout, but the ctx coupling was a latent hazard regardless and is now removed.
- `svc.Lifetime()` - per `connector/CLAUDE.md`, `lifetimeCtx` is created *between* `OnStartup`
  returning and control-sub activation, so during `OnStartup` `svc.Lifetime()` is still the
  construction-time `context.Background()`. Capturing it there would not give the real
  cancellable lifetime ctx.

The task call is the only place a *cancellable* context is used: `executeTask` receives a
`taskCtx` derived from `rootCtx` bounded by the step's time budget. So: worker DB ops on the
foreman lifetime context, a timing-out context only for the outbound task HTTP call. Worker
termination is driven by `cache.close()` + `svc.workers.Wait()`, not ctx cancellation, so
cancelling `rootCtx` only after every pool has drained never interrupts an in-flight write.

### Shutdown ordering: drain workers, then timer, then refiller, then cancel

`shortenNextPoll` signals the timer via `select { case svc.wakeTimer <- struct{}{}: default: }`. The
`default` only guards against a *full* channel - a send on a *closed* channel still panics, even inside a
`select`. The only callers of `shortenNextPoll` are worker goroutines (`processStep` and its retry/sleep/
poison paths); `timerLoop` only ever receives from `wakeTimer`. So `OnShutdown` drains the worker pool
before closing `wakeTimer`, then stops the refiller, then cancels `rootCtx`:

```
cache.close()        // unblocks blocked candidate pops independently of any channel
workers.Wait()       // no shortenNextPoll / requestRefill worker caller remains
close(wakeTimer)     // timerLoop's only termination signal
timerWorker.Wait()   // timerLoop fully exited (last requestRefill caller gone)
close(refillStop)    // refiller's termination signal
refiller.Wait()      // refiller fully exited; its DB ops complete
rootCancel()         // all worker/timer/refiller DB writes already complete
```

The timer and refiller each live on their own WaitGroup (`timerWorker`, `refiller`), separate from the
worker pool, so the close-then-wait order can be staged: `timerLoop` exits only when `wakeTimer` closes, so
draining it before `close(wakeTimer)` would deadlock. `refillTrigger` is **never closed** and is only ever
sent to non-blockingly, so a late coalesced `requestRefill` from the timer's final poll during the drain
window is a harmless no-op rather than a `send on closed channel` panic - the refiller is stopped by closing
the *separate* `refillStop` channel instead. A `cache.refill` into the already-closed cache is a no-op, so a
refiller run still in flight during the drain cannot resurrect a popped-from cache. The earlier (pre-Phase-2)
code closed `wakeTimer` before `workers.Wait()`, so a worker mid-`processStep` (e.g. the retry-sleep
`shortenNextPoll` after a 408/ack-timeout failStep) raced the close and panicked (caught by `workerLoop`'s
`errors.CatchPanic`, but spurious and a real ordering hazard).

### Transactions

`Start`, `Resume`, `Retry`, and `Cancel` wrap their step and flow mutations in a transaction with **steps-first-then-flow lock ordering** to prevent deadlocks. `processStep`'s transition evaluation (insert next steps + update flow's `step_id`) also runs in a transaction.

### Lease-Based Crash Recovery

Transactions don't help when a worker crashes during task execution (an HTTP call outside any transaction). The `lease_expires` column on the microbus_steps table serves as a crash-recovery lease. When a worker reserves a step for execution, it sets `lease_expires` to `NOW + time_budget + leaseMargin`. If the worker crashes, the lease eventually expires and `pollPendingSteps` resets the step to `pending` for re-execution.

### Background Recovery

1. **`pollPendingSteps`** - runs on a timer. Recovers steps stuck in `running` whose lease has expired by resetting them to `pending`. Enqueues `pending` steps that are due.
2. **Terminal flow check** in `processStep` - after loading flow data, checks if the flow is `cancelled`, `failed`, or `completed`. If so, sets the step to that status and returns. Catches races where `Cancel`, `failStep`, or flow completion set the flow to a terminal status before the step was updated.
3. **Orphan flow detection** in `pollPendingSteps` - logs an `ERROR` for any flow in `running` status that has no non-terminal steps (`pending`, `running`, `created`, `interrupted`) anywhere and whose `updated_at` is older than 5 minutes. This is a bug signal - in steady state the condition should never hold. Auto-recovery is intentionally **not** implemented: it would have to duplicate the fan-in/transition logic in `processStep`, and acting on a false positive (e.g. a flow that was actually mid-transaction) could double-advance the flow. The 5-minute threshold is comfortably past every normal transient state and avoids log noise during the microsecond windows of fan-in.

### Per-Function Analysis

#### Create / CreateTask / Fork

**Write order:** Insert flow (`created`) → insert step (`created`) → update flow's `step_id`.

**Partial failure:** If the process crashes after inserting the step but before updating `step_id`, the flow has `step_id=0` with a `created` step. The flow is inert until `Start` is called. If `Start` is called on such a flow, it transitions the flow to `running` and all `created` steps to `pending` within a transaction, then enqueues via `step_id`. If `step_id=0`, no step is enqueued - but `pollPendingSteps` picks up the orphaned `pending` step when its lease expires.

**Verdict:** Self-healing.

#### Start / Resume

**Transaction:** Updates steps to `pending`, then updates the flow to `running`, atomically.

**Partial failure:** If the process crashes after committing the transaction but before enqueuing, the steps are `pending` and the flow is `running`. `pollPendingSteps` picks them up when their leases expire.

**Verdict:** Self-healing.

#### Retry

**Transaction:** Marks failed steps as `retried`, inserts new copies as `pending`, updates the flow to `running` with the new `step_id`, atomically.

**Partial failure:** If the process crashes after committing the transaction but before enqueuing, the new steps are `pending` and the flow is `running`. `pollPendingSteps` picks them up when their leases expire. The original failed steps remain as `retried` with their error preserved.

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

**Transaction:** Walks up (`surgraphChain`) and down (`interruptedSubgraphChain`). Clears interrupt payloads, re-parks surgraph steps, merges resume data into the leaf step's state, resets the leaf to `pending`, and transitions all flows to `running` - atomically in a single transaction. After commit, re-propagates the next sibling's interrupt payload (if any) and enqueues the leaf step.

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

**Shard routing:** External flow IDs encode the shard number: `{shard}-{flowID}-{token}`. Every operation parses the shard from the flow ID and routes to the correct database connection via `svc.shard(n)`.

**Shard affinity:** Subgraph flows and forked flows are created on the same shard as their parent. This avoids cross-shard references during active execution (subgraph completion) and history reconstruction (fork lineage). Only top-level flow creation picks a random shard.

**Polling and purging:** All replicas poll all shards for pending steps and purge expired flows across all shards. The atomic CAS update on step acquisition ensures correctness when multiple replicas race.

**Dynamic expansion:** `NumShards` can increase at runtime via `OnChangedNumShards`. New shards are opened, migrated, and immediately available. Shrinking is rejected - old shards drain naturally as their flows complete.

**DSN format:** When `NumShards > 1`, the `SQLDataSourceName` must contain `%d` which is replaced with the shard index (0-based). In testing mode (SQLite), each shard gets a separate in-memory database via a unique test ID suffix.

#### PurgeExpiredFlows

**Write order:** Delete steps (by `flow_id`) → delete flow.

**Partial failure:** If the process crashes after deleting steps but before deleting the flow, the flow remains as an orphan with no steps. The next purge cycle deletes it.

**Verdict:** Self-healing.

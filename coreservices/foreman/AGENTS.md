**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Overview

The foreman is the orchestrator for agentic workflows in Microbus. It executes workflow graphs by dispatching tasks to microservices, managing state between steps, and handling fan-out/fan-in, interrupts, retries, and failure recovery.

### Core Concepts

**Workflow graph** - A directed graph that defines the structure of a workflow: which tasks to execute, in what order, and under what conditions. Graphs are defined in code using the `workflow.Graph` API and served as JSON via a dedicated endpoint. Each graph has a name, an entry point, a set of tasks, transitions between them, and optional reducers for fan-in state merging.

**Task** - An addressable endpoint on the bus (port `:428`) that performs a unit of work within a workflow. Tasks receive state via a `workflow.Flow` carrier, read input from state fields, perform work, and write output back to state fields. Tasks are reusable across workflows and can live on any microservice.

**Flow** - A single execution of a workflow graph. Each flow has a unique ID, tracks its current position in the graph, and maintains a state map that evolves as tasks execute. Flows progress through statuses: `created` → `running` → `completed`/`failed`/`cancelled`, with `interrupted` as a parked state for human-in-the-loop scenarios.

**Step** - A single task execution within a flow. Each step captures an immutable state snapshot (input), the changes produced by the task (output), and metadata like status, error, and timing. Steps are numbered sequentially (`step_depth`); parallel fan-out siblings share the same `step_depth`. Once a step reaches a terminal status (`completed`, `failed`, `retried`, `cancelled`), it is immutable. Retry creates a new step rather than modifying the failed one.

**Reducer** - A merge strategy for state fields during fan-in. When parallel branches converge, each branch's changes are merged using the reducer declared for that field: `replace` (last write wins, default), `append` (concatenate arrays), `add` (sum numbers), or `union` (deduplicate arrays).

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

**StartNotify** - Like `Start`, but also stores a `notify_hostname` on the flow. When the flow stops (completed, failed, cancelled, or interrupted), the foreman fires an `OnFlowStopped(flowID, status, snapshot)` event targeted at that hostname. The subscriber must use `foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(handler)` to receive these events.

**Snapshot** - Returns the current state, status, task name, and step number of a flow. During fan-out (`step_id=0`), it queries the microbus_steps table directly to find the active steps.

**Resume** - Continues an interrupted flow. Walks up the surgraph chain (`surgraphChain`) and down the interrupted subgraph chain (`interruptedSubgraphChain`) to find the leaf interrupted step. Merges resume data into the leaf step's state (not changes) so the task sees it via `ParseState`. Re-parks intermediate surgraph steps, resets the leaf to `pending`, and transitions all flows in the chain to `running`. Can be called on any flow in the chain (root or subgraph) - propagation goes both directions. If multiple fan-out siblings interrupt, each `Resume` handles one sibling; the flow only transitions to `running` when no more interrupted steps remain.

**Fork** - Creates a new flow from an existing flow's checkpoint at a given `step_depth`. The forked flow inherits the parent's graph, config, and actor claims. The step at the fork point is re-created with the merged state (state + changes) plus any overrides, in `created` status. The caller must call `Start` to begin execution. Lineage is tracked via `forked_flow_id` and `forked_step_depth`.

**Cancel** - Aborts a created, running, or interrupted flow. Uses `surgraphChain` to walk up to the root and `allSubgraphFlows` to walk down to all descendants. Atomically cancels all steps across all flows, computes `final_state` for each flow, and cancels all flows with per-flow `final_state` via CASE - all within a single transaction. Can be called on any flow in the chain (root or subgraph).

**Retry** - Re-executes the last failed step. Marks the failed step as `retried` (preserving its error in history), creates a new copy of the step as `pending`, and transitions the flow to `running`. If the flow had fan-out, all failed siblings are retried. The `retried` status is immutable - it serves as an audit trail of previous attempts.

**History** - Returns the step-by-step execution history as a slice of `FlowStep` records. Each `FlowStep` includes the step's key, depth, task name, state, changes, status, error, and timestamp. Steps that executed a subgraph have `Subgraph=true` and their `SubHistory` field contains the nested execution history of the subgraph flow. For forked flows, reconstructs the full lineage by walking `forked_flow_id` up to the root and querying steps across all ancestor flows with bounded `step_depth` ranges.

**List** - Queries flows by status, workflow name, or thread key. Supports cursor-based pagination via `cursorFlowID` (newest first). Defaults to 100 results. Returns `ThreadKey` in each `FlowSummary`. When filtering by `ThreadKey`, only flows in that thread are returned (scoped to the thread's shard).

**BreakBefore** - Sets or clears a breakpoint that pauses execution before the named task runs. Breakpoints are stored in a `breakpoints` JSON column on the flow row as a `map[string]string` keyed by task name. During `processStep`, if the current task name matches a breakpoint and the step's `breakpoint_hit` flag is false, the foreman interrupts the flow (using the same interrupt propagation as `flow.Interrupt()`) and sets `breakpoint_hit=1` on the step. The flag prevents the breakpoint from re-triggering when the flow is resumed. Breakpoints are inherited by subgraph flows and forked flows.

**Continue** - Creates a new flow from the latest completed flow in a thread, merged with additional state using the graph's reducers. The `threadKey` parameter accepts any flowKey belonging to the thread - `Continue` resolves the thread via the flow's `thread_id` column, finds the latest flow in that thread (`ORDER BY flow_id DESC`), validates it is completed, and creates the new flow in the same thread. The new flow uses the same workflow graph and is returned in `created` status. This enables multi-turn patterns where each turn is a separate flow: the caller holds a single threadKey (the flowKey returned by the initial `Create`) and calls `Continue` repeatedly without needing to track intermediate flowKeys. Fields that need to persist across turns must be declared in both `DeclareInputs` and `DeclareOutputs`.

**Enqueue** - Internal endpoint (port `:444`) that adds a step ID to the local worker queue. Load-balanced across foreman replicas to distribute work evenly during fan-out. Enqueue is fire-and-forget - errors are ignored because `pollPendingSteps` recovers any steps that fail to enqueue.

### Flow Stop Notifications

When a flow is started via `StartNotify(flowID, notifyHostname)`, the foreman stores the hostname and fires `OnFlowStopped(flowID, status, snapshot)` events targeted at that hostname when the flow stops - i.e. reaches a terminal status (`completed`, `failed`, `cancelled`) or is `interrupted`. This matches the statuses that `Await` returns on.

For terminal statuses, the snapshot contains the flow's `final_state`. For `interrupted`, the snapshot is nil - the caller should use `Snapshot()` to read the current state including the interrupt payload.

The notification is fire-and-forget - flow execution never blocks on notification delivery. The `notify_hostname` is set only on the root flow (via `StartNotify`); subgraph flows do not receive direct notifications. Interrupt notifications query the root flow's hostname from the surgraph chain.

Subscriber pattern:
```go
foremanapi.NewHook(svc).ForHost(svc.Hostname()).OnFlowStopped(svc.OnFlowStopped)
```

### Execution Model

The foreman uses a **queue-based execution model** with a configurable pool of worker goroutines (default 4). Each worker pops step IDs from a FIFO queue and calls `processStep`:

1. Reserve the step (atomic `UPDATE ... WHERE step_id=? AND lease_expires <= NOW`)
2. Check for terminal flow status (abort if cancelled/failed/completed)
3. Load the flow's graph, config, and actor claims
4. Execute the task via HTTP call to the task endpoint with a time budget
5. Persist changes, evaluate transitions, create next steps (in a transaction), enqueue them

Steps enter the queue via `Enqueue` (called by Start, Resume, Retry, processStep, and pollPendingSteps). All enqueue calls go through the distributed `foremanapi.NewClient(svc).Enqueue()` which load-balances across replicas. Enqueue is fire-and-forget - errors are ignored since `pollPendingSteps` recovers any steps that fail to enqueue.

A **timer loop** (`timerLoop`) runs `pollPendingSteps` periodically to recover stuck steps and enqueue steps whose `not_before` time has arrived. The poll interval adapts based on the nearest upcoming step.

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

### Time Budgets

Each step has a `time_budget_ms` that controls the `pub.Timeout` on the task execution HTTP call. The budget is resolved at step creation: the graph's per-task budget (`graph.SetTimeBudget`) takes precedence; otherwise the foreman's `DefaultTimeBudget` config is used (default 2m).

The worker lease duration is `time_budget + reservationMargin` (30s). This ensures the lease always outlasts execution, preventing premature recovery by `pollPendingSteps`.

### State Model

Each step has three JSON columns: `state` (input snapshot), `changes` (output delta produced by task execution), and `interrupt_payload` (data from `flow.Interrupt(payload)`). The `state` column is set at step creation and is normally immutable. The `changes` column is written after task execution.

The next step's `state` is computed as `merge(currentState, changes)` - the merged result overwrites matching fields. This immutability model enables checkpointing, fork, and recovery: the foreman can always reconstruct a step's output by reading its `state` + `changes`.

**State mutation on retry and resume:** The `state` column is updated in two cases: (1) on `flow.Retry()`, the foreman merges `state + changes` back into the `state` column so the task sees its own prior output on the next attempt; (2) on `Resume`, the resume data is merged into the leaf step's `state` column so the task sees the caller-provided data via `ParseState`. In both cases, `changes` is preserved as-is.

**Reducer delta convention:** Tasks writing to reducer-managed fields (append, add, union) must set only the **delta**, not the full accumulated value. For example, if `messages` uses the append reducer, the task should set `flow.Set("messages", []string{newMessage})` - not the entire message history. Violating this causes duplicates during fan-in merge.

**forEach element injection:** When a `forEach` transition spawns task instances, the current element is injected into `state` only (under the `as` key), not into `changes`. This makes the element available to the task but prevents it from participating in fan-in merge.

### Task-Initiated Control Signals

Tasks can signal the foreman via control methods on the `Flow` carrier. These are distinct from the API-level operations:

- **`flow.Retry(maxAttempts, initialDelay, multiplier, maxDelay) bool`** - Requests the foreman to re-execute this task with exponential backoff. Returns `true` if a retry will be scheduled (attempts remaining), `false` if exhausted. When `true`, the task should return `nil`. When `false`, the task should return the actual error. The step row is reused (not a new row). The foreman tracks the attempt count in the step's `attempt` column and computes the delay as `min(initialDelay * multiplier^attempt, maxDelay)`. Changes from this execution are preserved, and the foreman merges `state + changes` back into the step's `state` column so the task sees its own prior output on the next attempt via `ParseState`/`GetInt`/etc. This is different from the API-level `Retry`, which creates a new step to replace the failed one.
- **`flow.RetryNow() bool`** - Shorthand for `flow.Retry(math.MaxInt32, 0, 0, 0)` - retries immediately with no limit. Always returns `true`. The task itself is responsible for deciding when to stop retrying.

**No jitter on retry backoff:** The foreman does not add jitter to the computed backoff delay. The worker pool (`Workers` config, default 4) already throttles concurrency per replica - even if multiple retries fire simultaneously, they queue up in the worker pool rather than overwhelming downstream resources. Adding jitter would increase latency for no throughput benefit in this architecture.
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

### Flat State Across Subgraphs

State is flat and public across the subgraph boundary. When a subgraph flow is created, the full surgraph state is passed through. When a subgraph flow completes, its full final state is merged back into the surgraph step's changes. There is no input or output filtering.

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
- **Migration scripts** - all `.sql` files in `resources/sql/` include `-- DRIVER: sqlite` variants.

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

#### `microbus_steps` table

| Index | Columns | Purpose |
|---|---|---|
| PK | `(step_id)` | Row lookups, lease acquisition in `processStep` |
| `idx_microbus_steps_flow_id` | `(flow_id, step_id)` on MySQL; `(flow_id)` on pgx/mssql | Per-flow step queries (history, fan-in siblings, cancel) |
| `idx_microbus_steps_status` | `(status, updated_at)` - partial on pgx: `WHERE status IN ('pending', 'running')` | `pollPendingSteps` recovery and pending step discovery. Only non-terminal statuses are queried through this index |

### Data Retention

The `PurgeExpiredFlows` ticker runs every 24 hours and deletes terminated flows older than `RetentionDays` (default 0 = disabled). It uses a hybrid approach:
- Scans `microbus_flows` by PK in batches of 1000 (PK-ordered, no full scan)
- Stops when `created_at` exceeds the retention cutoff (PK-order heuristic). This assumes auto-increment IDs correlate with creation time, which holds per shard since each shard has an independent auto-increment sequence. Flows are never migrated between shards.
- Protects recently-active flows by also checking `updated_at` (a flow resumed yesterday won't be purged even if created months ago)
- Deletes steps first (by `flow_id` index), then flows

## Concurrency and Crash Recovery

The foreman uses SQL transactions for multi-statement operations and `lease_expires` for crash recovery.

### Transactions

`Start`, `Resume`, `Retry`, and `Cancel` wrap their step and flow mutations in a transaction with **steps-first-then-flow lock ordering** to prevent deadlocks. `processStep`'s transition evaluation (insert next steps + update flow's `step_id`) also runs in a transaction.

### Lease-Based Crash Recovery

Transactions don't help when a worker crashes during task execution (an HTTP call outside any transaction). The `lease_expires` column on the microbus_steps table serves as a crash-recovery lease. When a worker reserves a step for execution, it sets `lease_expires` to `NOW + time_budget + reservationMargin`. If the worker crashes, the lease eventually expires and `pollPendingSteps` resets the step to `pending` for re-execution.

### Background Recovery

1. **`pollPendingSteps`** - runs on a timer. Recovers steps stuck in `running` whose lease has expired by resetting them to `pending`. Enqueues `pending` steps that are due.
2. **Terminal flow check** in `processStep` - after loading flow data, checks if the flow is `cancelled`, `failed`, or `completed`. If so, sets the step to that status and returns. Catches races where `Cancel`, `failStep`, or flow completion set the flow to a terminal status before the step was updated.

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

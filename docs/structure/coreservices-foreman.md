# Package `coreservices/foreman`

The foreman is the orchestration engine for [agentic workflows](../blocks/agentic-workflows.md) in Microbus. It manages the lifecycle of flows - instances of a workflow that progress through a series of steps, where each step executes a task defined by a microservice. The foreman persists all state in a [SQL database](../blocks/sql.md), making workflows durable across restarts and recoverable after failures.

Workflows are defined as directed graphs by individual microservices. When a flow is created via `Create`, the foreman fetches the workflow's graph and stores it. `Start` transitions the flow to `running` and enqueues the first step. A pool of worker goroutines picks up pending steps and dispatches them to the corresponding task endpoints. After a task completes, the foreman evaluates the graph's transitions to determine the next step(s) and enqueues them. This continues until the flow reaches a terminal status: `completed`, `failed`, or `cancelled`.

`CreateTask` is a convenience that creates a single-step flow for executing one task without defining a full workflow graph.

### Flow Lifecycle

A flow progresses through these statuses:

- **`created`** - initialized but not yet started
- **`running`** - actively executing steps
- **`interrupted`** - paused, waiting for external input via `Resume`
- **`completed`** - all steps finished successfully
- **`failed`** - a step returned an error
- **`cancelled`** - explicitly cancelled via `Cancel`

`Snapshot` returns the current status and state of a flow. `History` returns the step-by-step execution history. `HistoryMermaid` renders the history as an HTML page with a Mermaid diagram.

### Notifications and Awaiting

When a flow is started via `StartNotify`, the foreman fires `OnFlowStopped` when the flow reaches a terminal status or is interrupted, delivering the flow ID, status, and state snapshot to the caller's hostname. `Await` is a synchronous alternative that blocks until the flow stops and returns the final status and state.

### Lineage

`Fork` creates a new flow from a checkpoint in an existing flow, with optional state overrides. This allows what-if exploration of alternative paths from a prior state.

### Failure Recovery

The foreman uses a lease-based model for step execution. When a worker picks up a step, it reserves it for a time window. If the worker crashes or times out, the lease expires and a periodic poll recovers the step so another worker can pick it up. `Retry` allows manually re-executing the last failed step of a flow.

### Debugging

`BreakBefore` sets a breakpoint that pauses execution before a named task runs. When a step hits a breakpoint, the flow is interrupted with the breakpoint information in the interrupt payload. This is useful for inspecting state mid-flow during development.

### Querying Flows

`List` supports filtering by status or workflow name and returns results in reverse chronological order. It uses cursor-based pagination with a default page size of 100.

### Data Retention

The `RetentionDays` config controls how long terminated flows and their steps are retained. When set to a positive value, the `PurgeExpiredFlows` ticker runs daily and deletes old flows in batches. Set to `0` (the default) to retain flows indefinitely.

### Database Sharding

The `NumShards` config distributes flows across multiple database instances. Each shard is opened and migrated independently. Shards can be added dynamically but never removed. Subgraph and forked flows are always created on the same shard as their parent.

### Configuration

```yaml
foreman.core:
  SQLDataSourceName: root:root@tcp(127.0.0.1:3306)/
  Workers: 4
  DefaultTimeBudget: 20s
  NumShards: 1
  RetentionDays: 0
```

`SQLDataSourceName` is the connection string for the backing database. The foreman supports MySQL, PostgreSQL, SQL Server and SQLite.

`Workers` controls the number of concurrent goroutines that process steps. Increase this for high-throughput workloads; keep it low to limit database connection pressure.

`DefaultTimeBudget` is the execution timeout for task steps when the workflow graph does not specify a per-task time budget.

`NumShards` is the number of database shards to distribute flows across.

`RetentionDays` sets the number of days to keep terminated flows. Set to `0` to disable automatic purging.

### Further Reading

- [Agentic workflows](../blocks/agentic-workflows.md) - conceptual overview of tasks, workflows and flows
- [Building agentic workflows](../howto/agentic-workflows.md) - step-by-step guide to implementing workflows
- [Package `workflow`](../structure/workflow.md) - API reference for `Flow`, `Graph`, `Reducer` and related types

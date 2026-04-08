# Agentic Workflows

Agentic workflows allow microservices to collaborate on multi-step processes that may branch, fan out, pause for external input, and recover from failures - all without the participants having to know about each other or the overall flow. Microbus models these workflows as directed graphs whose execution is managed by the [Foreman](../structure/coreservices-foreman.md) core service.

## Workflows

A workflow is a directed graph that describes how tasks are connected. Each node is either a task or a reference to another workflow (a subgraph). Edges between nodes are transitions that determine which path is taken after a task completes.

<img src="agentic-workflows-1.drawio.svg">
<p></p>

Workflow graphs are defined in code by a microservice and exposed as an endpoint. The graph is fetched once when a flow is created and stored alongside it, so the definition is immutable for the lifetime of that flow.

## Tasks

A task is a functional endpoint that performs a single step of a workflow. It receives a `*workflow.Flow` carrier that provides access to the workflow's shared state and control signals. Tasks read input from the state, do their work, write output back to the state, and return. They have no knowledge of what comes before or after them.

Tasks are registered on port `:428` by default and are built using a dedicated [coding agent skill](../blocks/coding-agents.md), just like any other microservice feature.

## State

A workflow's state is a `map[string]any` key-value bag that is shared across all tasks in a flow. Tasks read their inputs from the state and write their outputs back to it. The state is passed from step to step and persisted in a [SQL database](../blocks/sql.md) by the Foreman. Each step records both its input state and the changes it produced, creating a full history of the flow's execution.

## Transitions

Transitions are the edges of the workflow graph. They connect tasks and control the order of execution. There are six types of transitions, each with different routing semantics.

### Unconditional

An unconditional transition is always taken after the source task completes. It is the simplest form of transition and is used for straightforward sequential flows.

### Conditional

A conditional transition is taken only when a boolean expression over the flow's state evaluates to true. This allows branching based on values that tasks have written to the state - for example, routing to an approval task when a dollar amount exceeds a threshold.

### Goto

A goto transition is taken only when the source task explicitly requests to jump to a specific target. Unlike conditional transitions that are evaluated by the Foreman based on state, goto transitions are controlled imperatively by the task itself. This is useful for loops or retry patterns where the task decides at runtime whether to repeat or move on.

### Fan-Out

When multiple transitions match from a single task, all of their targets execute in parallel. This is fan-out, and it happens naturally whether the transitions are unconditional, conditional, for each, or a mix.

### Dynamic Fan-Out

A `forEach` transition iterates over an array field in the state and spawns one parallel branch per element. This is useful when the number of parallel branches is not known at design time - for example, verifying each employer in a list of variable length.

### Fan-In

When parallel branches converge on a shared successor, the Foreman waits for all branches to complete and merges their state changes before continuing. This is fan-in. Configurable reducers control how conflicting changes to the same state field are combined - last-write-wins, append, sum, or set union.

## Control Signals

Tasks can issue control signals to influence the flow's execution.

### Goto

A task can `goto` another task to override normal transition routing and jump to a specific target. This is useful for loops or retry patterns where the task decides at runtime whether to repeat or move on.

### Interrupt

A task can `interrupt` itself to pause the flow and signal that external input is needed - a human approval, a callback from a third-party system, or any event that cannot be awaited synchronously. The flow enters the `interrupted` status and remains parked until an external actor calls `Resume` with the required data.

### Sleep

A task can `sleep` to delay the execution of the next step by a specified duration. The flow is parked and automatically resumed by the Foreman after the duration elapses.

### Retry

A task can `retry` to re-execute itself on the next pass, preserving the changes it has made so far. This is useful for polling or incremental progress.

### Dynamic Subgraph

A task can launch a child workflow dynamically with `flow.Subgraph(workflowURL, input)`. The step is parked until the child completes, then the task is re-executed with the child's output merged into state. This follows the same re-entry pattern as `Interrupt` - the task detects re-entry and processes the child's result. Unlike static subgraphs (registered in the graph definition), dynamic subgraphs are triggered at runtime based on the task's logic.

## The Foreman

The [Foreman](../structure/coreservices-foreman.md) core service is the orchestration engine that ties everything together. It fetches workflow graphs, creates and persists flows, dispatches tasks to the appropriate microservices, evaluates transitions, merges state at fan-in points, and handles interrupts and recovery. The Foreman persists all state in a SQL database via the [Sequel](../blocks/sql.md) library, making workflows durable across restarts.

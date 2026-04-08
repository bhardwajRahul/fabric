# Package `workflow`

The `workflow` package provides the core types for defining and executing agentic workflows in Microbus. It is used by both task authors (who interact with `Flow`) and workflow authors (who build `Graph` definitions).

### Flow

`Flow` is the carrier object passed to every task endpoint. It provides access to the workflow's identity, state, and control signals.

#### Identity fields (read-only)

- `FlowID` - unique identifier for this flow execution
- `WorkflowName` - the workflow graph's name
- `TaskName` - the current task being executed
- `StepNum` - the current step number in the flow

#### State access

Tasks read input from state and write output back:

```go
// Field-based access (one-off reads/writes)
name := flow.GetString("name")
score := flow.GetFloat("score")
flow.Set("approved", true)

// Check for existence
if flow.Has("delay") {
    delay := flow.GetFloat("delay")
    // ...
}

// Struct-based access (parse/diff pattern)
snap := flow.Snapshot() // returns map[string]any
var state MyState
flow.ParseState(&state)
// ... modify state ...
flow.SetChanges(state, snap) // only records fields that differ from snap
```

#### Control signals

Tasks can influence execution flow:

```go
flow.Goto(myserviceapi.OtherTask.URL()) // override next transition
flow.Retry()                            // re-execute this task
flow.Sleep(5 * time.Second)             // delay before next execution
flow.Interrupt(payload)                  // pause for external input
```

### Graph

`Graph` defines the structure of a workflow: tasks, transitions between them, and reducers for fan-in state merging.

```go
graph := workflow.NewGraph(myserviceapi.CreateOrder.URL())
graph.AddTransition(myserviceapi.Validate.URL(), myserviceapi.Charge.URL())
graph.AddTransitionWhen(myserviceapi.Validate.URL(), myserviceapi.Reject.URL(), "valid != true")
graph.AddTransition(myserviceapi.Charge.URL(), myserviceapi.Fulfill.URL())
graph.AddTransition(myserviceapi.Fulfill.URL(), workflow.END)
```

#### Transition types

| Method | Description |
|---|---|
| `AddTransition(from, to)` | Unconditional transition, always taken |
| `AddTransitionWhen(from, to, when)` | Taken when the `when` expression matches state |
| `AddTransitionGoto(from, to)` | Only taken when the task calls `flow.Goto(to)` |
| `AddTransitionForEach(from, to, forEach, as)` | Dynamic fan-out: iterates over a state array field |

#### Reducers

Declare merge strategies for state fields during fan-in:

```go
graph.SetReducer("messages", workflow.ReducerAppend)
graph.SetReducer("score", workflow.ReducerAdd)
```

| Reducer | Behavior |
|---|---|
| `ReducerReplace` (default) | Last write wins |
| `ReducerAppend` | Concatenate arrays |
| `ReducerAdd` | Sum numbers |
| `ReducerUnion` | Deduplicate and merge arrays |

#### Time budgets

Set per-task execution timeouts:

```go
graph.SetTimeBudget(myserviceapi.SlowTask.URL(), 2*time.Minute)
```

#### Validation and visualization

```go
err := graph.Validate()    // check for structural errors
mermaid := graph.Mermaid() // generate a Mermaid flowchart
```

### Transition

`Transition` describes a single edge in the graph:

```go
type Transition struct {
    From     string // source task URL
    To       string // target task URL (or workflow.END)
    When     string // boolean expression evaluated against state
    WithGoto bool   // only taken on explicit flow.Goto()
    ForEach  string // state field to iterate over (dynamic fan-out)
    As       string // alias for current element during forEach
}
```

### Node

`Node` describes a task or subgraph node registered in a graph:

```go
type Node struct {
    Name       string        // task or subgraph URL
    TimeBudget time.Duration // execution timeout (0 = use foreman default)
    Subgraph   bool          // true if this node is a child workflow
}
```

### Reducer

`Reducer` is a string constant defining a merge strategy:

```go
const (
    ReducerReplace Reducer = "replace"
    ReducerAppend  Reducer = "append"
    ReducerAdd     Reducer = "add"
    ReducerUnion   Reducer = "union"
)
```

### RawFlow

`RawFlow` wraps `Flow` with additional methods used by the foreman orchestrator. Task authors do not interact with `RawFlow` directly. It exposes raw state/config/changes access, control signal readers, and mutation methods needed for orchestration bookkeeping.

### Further Reading

- [Agentic workflows](../blocks/agentic-workflows.md) - conceptual overview of tasks, workflows and flows
- [Building agentic workflows](../howto/agentic-workflows.md) - step-by-step guide to implementing workflows
- [Package `coreservices/foreman`](../structure/coreservices-foreman.md) - Foreman configuration and endpoints

# Building Agentic Workflows

This guide walks through the steps of building an [agentic workflow](../blocks/agentic-workflows.md) in Microbus - from defining tasks and wiring them into a graph, to running and controlling flows via the [Foreman](../structure/coreservices-foreman.md).

## Step 1: Define the Tasks

Each step of a workflow is a task endpoint. Use the coding agent to add tasks to your microservice:

> HEY CLAUDE...
>
> Add a task "VerifyCredit" that receives a creditScore int and returns creditVerified bool. It should approve scores of 550 or higher.

A task endpoint receives `ctx context.Context` and `flow *workflow.Flow` as the first two parameters, followed by input arguments sourced from the workflow's shared state. Output arguments are written back to the state automatically.

```go
func (svc *Service) VerifyCredit(ctx context.Context, flow *workflow.Flow, creditScore int) (creditVerified bool, err error) {
	creditVerified = creditScore >= 550
	return creditVerified, nil
}
```

Tasks are standalone and have no knowledge of the workflow graph. They can be tested individually using the generated `Executor` in the API package:

```go
t.Run("good_score", func(t *testing.T) {
	assert := testarossa.For(t)
	creditVerified, err := exec.VerifyCredit(ctx, 750)
	assert.Expect(creditVerified, true, err, nil)
})
```

## Step 2: Define the Workflow Graph

A workflow graph endpoint returns a `*workflow.Graph` that wires tasks together with transitions. Use the coding agent to create it:

> HEY CLAUDE...
>
> Add a workflow "CreditApproval" that defines the graph for the credit approval process. Inputs: applicant Applicant. Outputs: approved bool.

The graph is built programmatically using task URL constants from the generated API package:

```go
func (svc *Service) CreditApproval(ctx context.Context) (graph *workflow.Graph, err error) {
	submit := myserviceapi.SubmitApplication.URL()
	verify := myserviceapi.VerifyCredit.URL()
	decide := myserviceapi.Decision.URL()

	graph = workflow.NewGraph(myserviceapi.CreditApproval.URL())
	graph.AddTransition(submit, verify)
	graph.AddTransition(verify, decide)
	graph.AddTransition(decide, workflow.END)
	return graph, nil
}
```

### Conditional Transitions

Route to different tasks based on state values:

```go
graph.AddTransitionWhen(verify, manualReview, "!creditVerified")
graph.AddTransitionWhen(verify, decide, "creditVerified")
```

### Fan-Out and Fan-In

Multiple transitions from a single task create parallel branches. When parallel branches target the same successor, the Foreman waits for all branches to complete before continuing.

```go
// Fan-out: submit fans out to three parallel verifications
graph.AddTransition(submit, verifyCredit)
graph.AddTransition(submit, verifyIdentity)
graph.AddTransition(submit, verifyEmployment)
// Fan-in: all three converge on decide
graph.AddTransition(verifyCredit, decide)
graph.AddTransition(verifyIdentity, decide)
graph.AddTransition(verifyEmployment, decide)
```

When parallel branches write to the same state field, use a reducer to control how their values are merged:

```go
graph.SetReducer("failures", workflow.ReducerAdd)    // Sum numeric values
graph.SetReducer("messages", workflow.ReducerAppend) // Concatenate arrays
graph.SetReducer("tags", workflow.ReducerUnion)      // Merge and deduplicate
```

### Dynamic Fan-Out

When the number of parallel branches depends on runtime data, use `forEach` to iterate over a state array:

```go
// Spawn one VerifyEmployment task per employer in the "employers" array.
// Each instance receives the current element as "employerName" in state.
graph.AddTransitionForEach(submit, verifyEmployment, "employers", "employerName")
```

If the array is empty, no tasks are spawned for that transition. When a `forEach` transition is the only outgoing transition from a task, an empty array causes the flow to complete at that point - downstream tasks (including the fan-in target) are never reached.

### Goto Transitions

A goto transition is only taken when the task explicitly calls `flow.Goto()`. This enables loops and dynamic routing:

```go
graph.AddTransitionGoto(review, requestMoreInfo)
graph.AddTransition(requestMoreInfo, review) // Loop back
graph.AddTransition(review, decide)          // Normal exit
```

```go
func (svc *Service) ReviewCredit(ctx context.Context, flow *workflow.Flow, creditScore int, attempts int) (err error) {
	if creditScore < 580 && attempts < 3 {
		flow.Goto(myserviceapi.RequestMoreInfo.URL())
	}
	return nil
}
```

### Subgraphs

Reference another workflow as a node in your graph. The subgraph runs as a child flow with its own steps, and its final state is merged back into the parent when it completes.

```go
graph.AddSubgraph(otherserviceapi.IdentityVerification.URL())
graph.AddTransition(submit, otherserviceapi.IdentityVerification.URL())
```

### Time Budgets

Set per-task execution timeouts. If a task exceeds its budget, the step fails.

```go
graph.SetTimeBudget(verifyPhone, 1*time.Second)
```

## Step 3: Add the Foreman to Your App

The Foreman core service must be included in any application that runs workflows.

```go
app.Add(
	foreman.NewService(),
)
```

Configure the database connection in `config.yaml`:

```yaml
foreman.core:
  SQLDataSourceName: root:root@tcp(127.0.0.1:3306)/my_database
```

In tests, the Foreman automatically uses an in-memory SQLite database - no configuration needed.

## Step 4: Run a Workflow

Use the Foreman's client to create and start a flow:

```go
client := foremanapi.NewClient(svc)

// Create a flow with initial state
flowID, err := client.Create(ctx, myserviceapi.CreditApproval.URL(), map[string]any{
	"applicant": applicant,
})

// Start execution
err = client.Start(ctx, flowID)

// Wait for completion
status, state, err := client.Await(ctx, flowID)
```

For workflows that have typed inputs and outputs, the generated API package provides convenience methods that handle create, start, await, and state parsing in a single call:

```go
approved, status, err := exec.CreditApproval(ctx, applicant)
```

### Asynchronous Notification

Add an inbound event sink to receive a notification when the flow stops:

> HEY CLAUDE...
>
> Add an inbound event sink for `OnFlowStopped` from the foreman core service. When a flow completes, log its status.

Then use `StartNotify` to tell the Foreman to notify your microservice when the flow stops:

```go
err = client.StartNotify(ctx, flowID, svc.Hostname())
```

## Control Signals

Tasks can influence the flow's execution using control signals on the `*workflow.Flow` carrier.

### Retry

Re-execute the current task on the next pass, preserving changes made so far. Useful for polling or incremental progress.

```go
func (svc *Service) PollStatus(ctx context.Context, flow *workflow.Flow, retryCount int) (retryCountOut int, err error) {
	if retryCount < 3 {
		flow.Retry()
		return retryCount + 1, nil
	}
	return retryCount, nil
}
```

### Interrupt

Pause the flow for external input. The caller receives the interrupt payload via `Snapshot` or `OnFlowStopped` and resumes the flow with `Resume`.

```go
func (svc *Service) VerifySSN(ctx context.Context, flow *workflow.Flow, ssn string) (ssnVerified bool, err error) {
	if ssn == "" {
		flow.Interrupt(map[string]any{"request": "ssn"})
		return false, nil
	}
	ssnVerified = isValidSSN(ssn)
	return ssnVerified, nil
}
```

The caller resumes the flow by providing the missing data:

```go
err = client.Resume(ctx, flowID, map[string]any{"ssn": "123-45-6789"})
```

### Sleep

Delay the next step by a duration. Useful for rate limiting or waiting for external state to settle.

```go
flow.Sleep(5 * time.Minute)
```

### Dynamic Subgraph

`flow.Subgraph` dynamically launches a child workflow at runtime. Unlike static subgraphs (registered in the graph with `AddSubgraph`), dynamic subgraphs are triggered by a task during execution. The step is parked until the child completes, then the task is re-executed with the child's output merged into state - the same re-entry pattern as `flow.Interrupt`.

```go
func (svc *Service) ExecuteTool(ctx context.Context, flow *workflow.Flow, toolExecuted bool) (toolExecutedOut bool, err error) {
    if toolExecuted {
        // Re-entry: child workflow completed, its output is in state
        // Process the result...
        return true, nil
    }
    // First run: launch child workflow dynamically
    flow.Subgraph(otherserviceapi.SomeWorkflow.URL(), map[string]any{
        "inputField": "value",
    })
    return true, nil
}
```

The `input` map is merged into the parent's current state using the child graph's reducers, then filtered through the child's `DeclareInputs` - the same semantics as `Continue`'s additionalState.

If the child workflow interrupts, the interrupt propagates up through the parent chain to the root flow. A task must set at most one control signal - setting both `Subgraph` and another signal (Goto, Retry, Interrupt) will fail the step.

## Debugging

### Breakpoints

Set a breakpoint to pause execution before a specific task runs:

```go
err = client.BreakBefore(ctx, flowID, myserviceapi.ReviewCredit.URL(), true)
```

When the breakpoint is hit, the flow enters `interrupted` status. Inspect the state, then resume:

```go
status, state, err := client.Await(ctx, flowID)
// Inspect state...
err = client.Resume(ctx, flowID, nil)
```

### History

Retrieve the step-by-step execution history of a flow:

```go
steps, err := client.History(ctx, flowID)
```

### Fork

Create a new flow from a checkpoint in an existing flow's history to explore alternative paths. Use the `StepKey` from `History` to identify the checkpoint:

```go
newFlowKey, err := client.Fork(ctx, step.StepKey, map[string]any{
	"creditScore": 800, // Override a state field
})
err = client.Start(ctx, newFlowKey)
```

## Further Reading

- [Agentic workflows](../blocks/agentic-workflows.md) - conceptual overview of tasks, workflows and flows
- [Package `workflow`](../structure/workflow.md) - API reference for `Flow`, `Graph`, `Reducer` and related types
- [Package `coreservices/foreman`](../structure/coreservices-foreman.md) - Foreman configuration and endpoints

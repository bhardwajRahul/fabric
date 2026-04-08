---
name: Adding a Workflow Graph
description: Creates or modifies a workflow graph endpoint that defines the structure of a workflow. Use when explicitly asked by the user to create or modify a workflow or graph of a microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**IMPORTANT**: Read `.claude/rules/workflows.txt` for workflow and task conventions before proceeding.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a workflow graph:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Determine the route
- [ ] Step 4: Determine a description
- [ ] Step 5: Extend the API
- [ ] Step 6: Extend the ToDo interface
- [ ] Step 7: Implement the logic
- [ ] Step 8: Define the marshaler function
- [ ] Step 9: Bind the marshaler function to the microservice
- [ ] Step 10: Expose the workflow via OpenAPI
- [ ] Step 11: Extend the mock
- [ ] Step 12: Test the workflow
- [ ] Step 13: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the input and output fields of the workflow. Inputs are the state fields the workflow expects from its caller. Outputs are the state fields the workflow produces as its result. The signature is documentation only - it describes the workflow's expected state contract but does not generate typed code. The actual state is `map[string]any`.

```go
MyWorkflow(inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int)
```

Constraints:
- The name `status` is reserved
- All fields must be serializable into JSON
- Field names must start with a lowercase letter
- The workflow name must start with an uppercase letter

#### Step 3: Determine the Route

The route of the workflow graph endpoint is resolved relative to the hostname of the microservice. Workflows use the dedicated port `:428` to prevent external access. Use the name of the workflow in kebab-case as its route, e.g. `:428/my-workflow`. The method is `GET`.

#### Step 4: Determine a Description

Describe the workflow starting with its name, in Go doc style: `MyWorkflow does X`. Embed this description in followup steps wherever you see `MyWorkflow does X`.

#### Step 5: Extend the API

Define the endpoint in the `var` block at the top of `myserviceapi/client.go`, after the `// HINT: Insert endpoint definitions here` comment. Workflows use `GET` method.

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyWorkflow = Def{Method: "GET", Route: ":428/my-workflow"} // MARKER: MyWorkflow
)
```

Append the workflow's payload structs at the end of `myserviceapi/client.go`.
Use PascalCase for the field names and camelCase for the `json` tag names.

`MyWorkflowIn` holds the input arguments of the workflow.

```go
// MyWorkflowIn are the input arguments of MyWorkflow.
type MyWorkflowIn struct { // MARKER: MyWorkflow
	InputField1 string  `json:"inputField1,omitzero"`
	InputField2 float64 `json:"inputField2,omitzero"`
}
```

`MyWorkflowOut` holds the output arguments of the workflow.

```go
// MyWorkflowOut are the output arguments of MyWorkflow.
type MyWorkflowOut struct { // MARKER: MyWorkflow
	OutputField1 bool `json:"outputField1,omitzero"`
	OutputField2 int  `json:"outputField2,omitzero"`
}
```

Append the following Executor method after the last existing Executor method. This method delegates to `marshalWorkflow` which calls the `WorkflowRunner` to create, start, and await the workflow.

```go
/*
MyWorkflow creates and runs the MyWorkflow workflow, blocking until termination.
*/
func (_c Executor) MyWorkflow(ctx context.Context, inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int, status string, err error) { // MARKER: MyWorkflow
	if _c.runner == nil {
		return outputField1, outputField2, "", errors.New("workflow runner not set, use WithWorkflowRunner")
	}
	var out MyWorkflowOut
	status, err = marshalWorkflow(ctx, _c.runner, MyWorkflow.URL(), MyWorkflowIn{
		InputField1: inputField1,
		InputField2: inputField2,
	}, &out)
	return out.OutputField1, out.OutputField2, status, err
}
```

#### Step 6: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`. All workflow graph functions have the fixed signature `MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error)`.

```go
type ToDo interface {
	// ...
	MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error) // MARKER: MyWorkflow
}
```

#### Step 7: Implement the Logic

Implement the workflow graph builder in `service.go`. Use the `workflow.NewGraph` builder API to construct the graph. Reference task endpoints from this or other microservices using their `Def.URL()` method.

Define short variable names for task URLs at the top of the function to keep the graph definition legible.

```go
/*
MyWorkflow does X.
*/
func (svc *Service) MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: MyWorkflow
	taskA := myserviceapi.TaskA.URL()
	taskB := myserviceapi.TaskB.URL()
	taskC := myserviceapi.TaskC.URL()
	// childWorkflow := otherapi.ChildWorkflow.URL()

	graph = workflow.NewGraph(myserviceapi.MyWorkflow.URL())
	// Build the graph here...
	// graph.AddSubgraph(childWorkflow)  // register a child workflow as a subgraph node
	// graph.AddTransition(taskA, taskB)
	// graph.AddTransitionWhen(taskB, workflow.END, "done == true")
	// graph.AddTransitionGoto(taskB, taskC)
	return graph, nil
}
```

#### Step 8: Define the Marshaler Function

Append a web handler at the end of `intermediate.go` to perform the marshaling.

```go
// doMyWorkflow handles marshaling for MyWorkflow.
func (svc *Intermediate) doMyWorkflow(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyWorkflow
	graph, err := svc.MyWorkflow(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

#### Step 9: Bind the Marshaler Function to the Microservice

Bind the `doMyWorkflow` marshaler function to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the `// HINT: Add graph endpoints here` comment. If other subscriptions already exist under this HINT, add the new one after the last existing subscription.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...

	// HINT: Add graph endpoints here
	svc.Subscribe(myserviceapi.MyWorkflow.Method, myserviceapi.MyWorkflow.Route, svc.doMyWorkflow) // MARKER: MyWorkflow

	// ...
}
```

#### Step 10: Expose the Workflow via OpenAPI

Register the workflow in `doOpenAPI` in `intermediate.go`, after the corresponding `HINT` comment.

- For a workflow, the `Type` field should be set to `workflow`
- Use `POST` as the method in the OpenAPI registration since workflows are invoked with input state
- Set the simplified signature of the workflow in the `Summary` field, excluding `ctx context.Context`, `err error`, and `status string`
- Set the `RequiredClaims` boolean expression, if relevant. Otherwise, omit the field or leave it empty

```go
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	// ...
	endpoints := []*openapi.Endpoint{
		// ...
		{ // MARKER: MyWorkflow
			Type:           "workflow",
			Name:           "MyWorkflow",
			Method:         "POST",
			Route:          myserviceapi.MyWorkflow.Route,
			Summary:        "MyWorkflow(inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int)",
			Description:    `MyWorkflow does X.`,
			RequiredClaims: ``,
			InputArgs:      myserviceapi.MyWorkflowIn{},
			OutputArgs:     myserviceapi.MyWorkflowOut{},
		},
	}
	// ...
}
```

Add the route of the workflow to the `routes` slice in `TestMyService_OpenAPI` in `service_test.go`.

```go
routes := []string{
	// HINT: Insert routes of functional and web endpoints here
	// ...
	myserviceapi.MyWorkflow.Route, // MARKER: MyWorkflow
}
```

#### Step 11: Extend the Mock

Add fields to the `Mock` structure definition in `mock.go` to hold the graph override and the unsub callback.

```go
type Mock struct {
	// ...
	mockMyWorkflowGraph func(ctx context.Context) (graph *workflow.Graph, err error)              // MARKER: MyWorkflow
	unsubMockMyWorkflow func() error                                                              // MARKER: MyWorkflow
}
```

Add the stubs to the `Mock`. `MockMyWorkflow` mocks the workflow's behavior by replacing the graph with a trivial single-task graph and subscribing a synthetic task endpoint. The handler has the same typed signature as the workflow - typed inputs, typed outputs, plus the `*workflow.Flow` carrier for control signals.

```go
// MockMyWorkflow sets up a mock handler for the MyWorkflow workflow.
func (svc *Mock) MockMyWorkflow(handler func(ctx context.Context, flow *workflow.Flow, inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int, err error)) *Mock { // MARKER: MyWorkflow
	if svc.unsubMockMyWorkflow != nil {
		svc.unsubMockMyWorkflow()
		svc.unsubMockMyWorkflow = nil
	}
	if handler == nil {
		svc.mockMyWorkflowGraph = nil
		return svc
	}
	mockRoute := ":428/mock-wf-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockMyWorkflowGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(myserviceapi.MyWorkflow.URL())
		g.AddTransition(mockTaskURL, workflow.END)
		return g, nil
	}
	unsub, _ := svc.Subscribe("POST", mockRoute, func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		err := json.NewDecoder(r.Body).Decode(&f)
		if err != nil {
			return errors.Trace(err)
		}
		snap := f.Snapshot()
		var in myserviceapi.MyWorkflowIn
		f.ParseState(&in)
		var out myserviceapi.MyWorkflowOut
		out.OutputField1, out.OutputField2, err = handler(r.Context(), &f, in.InputField1, in.InputField2)
		if err != nil {
			return err // No trace
		}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	})
	svc.unsubMockMyWorkflow = unsub
	return svc
}

// MyWorkflow returns the workflow graph.
func (svc *Mock) MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: MyWorkflow
	if svc.mockMyWorkflowGraph == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	graph, err = svc.mockMyWorkflowGraph(ctx)
	return graph, errors.Trace(err)
}
```

Add a test case at the end of `TestMyService_Mock` in `service_test.go`, after the last existing test case.

```go
t.Run("my_workflow", func(t *testing.T) { // MARKER: MyWorkflow
	assert := testarossa.For(t)

	// Before mocking, graph endpoint returns "not implemented"
	_, err := mock.MyWorkflow(ctx)
	assert.Contains(err.Error(), "not implemented")

	// Mock the workflow behavior
	mock.MockMyWorkflow(func(ctx context.Context, flow *workflow.Flow, inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int, err error) {
		return true, 42, nil
	})
	// Graph endpoint should now return a valid graph
	graph, err := mock.MyWorkflow(ctx)
	if assert.NoError(err) {
		assert.NotNil(graph)
	}

	// Clear the mock
	mock.MockMyWorkflow(nil)
	_, err = mock.MyWorkflow(ctx)
	assert.Contains(err.Error(), "not implemented")
})
```

#### Step 12: Test the Workflow

Append the integration test to `service_test.go`. The test includes the foreman service in the app to enable end-to-end workflow execution.

Ensure that `"github.com/microbus-io/fabric/coreservices/foreman"` and `"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"` are imported in `service_test.go`. Add them if not already present.

```go
func TestMyService_MyWorkflow(t *testing.T) { // MARKER: MyWorkflow
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := myserviceapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputState to also inspect the full state map if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			outputField1, outputField2, status, err := exec.MyWorkflow(ctx, inputField1, inputField2)
			assert.Expect(
				err, nil,
				status, foremanapi.StatusCompleted,
				outputField1, expectedValue1,
				outputField2, expectedValue2,
			)
		})
	*/
}
```

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

- Run the workflow via `exec.MyWorkflow` with various initial states
- Assert expected output state values
- Cover the main paths through the workflow (happy path, error paths, edge cases)
- Do not remove the `HINT` comments

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	outputField1, outputField2, status, err := exec.MyWorkflow(ctx, inputField1, inputField2)
	assert.Expect(
		err, nil,
		status, foremanapi.StatusCompleted,
		outputField1, expectedValue1,
		outputField2, expectedValue2,
	)
})
```

#### Step 13: Housekeeping

Follow the `microbus/housekeeping` skill.

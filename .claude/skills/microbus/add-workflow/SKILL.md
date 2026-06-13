---
name: add-workflow
description: TRIGGER when user asks to define a workflow graph, create an agent, create an agentic workflow, orchestrate tasks, or build a multi-step process. "Agent", "agentic workflow", and "workflow" are interchangeable in this codebase - an LLM is not required for a workflow to be an agent (rule-based, deterministic, and LLM-driven workflows are all agents). Defines task transitions and conditions. Affects intermediate.go, *api/client.go, mock.go, manifest.yaml.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**IMPORTANT**: Read `.claude/rules/workflows.txt` for workflow and task conventions before proceeding.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a workflow graph:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Determine the route
- [ ] Step 4: Determine a description
- [ ] Step 5: Define the endpoint and payload structs
- [ ] Step 6: Extend the executor
- [ ] Step 7: Extend the ToDo interface
- [ ] Step 8: Implement the logic
- [ ] Step 9: Define the marshaler function
- [ ] Step 10: Bind the marshaler function to the microservice
- [ ] Step 11: Regenerate the mock
- [ ] Step 12: Test the workflow
- [ ] Step 13: Housekeeping
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

Ensure the local `CLAUDE.md` advertises that this microservice implements agentic workflows. The Agent Instructions block holds one short paragraph per instruction so multiple instructions (workflows, SQL, auth) can coexist as separate paragraphs. The paragraph to add is:

```markdown
This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.
```

How to apply:

- If the file does not exist, create it with the hostname as an H1 heading, then add an `## Agent Instructions` section containing the paragraph above.
- If the file exists and already has an `## Agent Instructions` section, append the paragraph (separated by a blank line) after the existing instructions; skip if a workflows-related paragraph is already present.
- If the file exists but has no `## Agent Instructions` section, insert one as the first section after the H1 hostname heading and add the paragraph.

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

Describe **what the workflow does and the outcome it produces**, not who is expected to run it. The same workflow may be started by a user request, a scheduled job, a subgraph, a test, or an LLM tool call - the description should read the same in every case. `"Underwrites a credit application: gathers identity, employment and credit history, then approves or declines"` is good; `"called by the customer portal"` or `"used by the LLM as a tool"` is not. Naming the caller leaks deployment context into the workflow contract and goes stale as soon as another caller starts it.

#### Step 5: Define the Endpoint and Payload Structs

Append the workflow's payload structs at the end of `myserviceapi/endpoints.go`. Use PascalCase for the field names and camelCase for the `json` tag names.

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

Append the endpoint definition to the `var` block in `myserviceapi/endpoints.go`, after the `// HINT: Insert endpoint definitions here` comment. Workflows always use the `GET` method on the dedicated `:428` port. The `Def` struct carries only the `Method` and `Route`; the endpoint name, description, input/output schemas, and required claims are wired up via `svc.Subscribe` in `intermediate.go` (Step 10).

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyWorkflow = Def{Method: "GET", Route: ":428/my-workflow"} // MARKER: MyWorkflow
)
```

#### Step 6: Extend the Executor

Append the following Executor method at the end of `myserviceapi/client.go`, after the last existing Executor method. This method delegates to `marshalWorkflow` which calls the `WorkflowRunner` to create, start, and await the workflow.

```go
/*
MyWorkflow creates and runs the MyWorkflow workflow, blocking until termination.
*/
func (_c Executor) MyWorkflow(ctx context.Context, inputField1 string, inputField2 float64) (outputField1 bool, outputField2 int, status string, err error) { // MARKER: MyWorkflow
	if _c.runner == nil {
		return outputField1, outputField2, "", errors.New("workflow runner not set, use WithWorkflowRunner")
	}
	var out MyWorkflowOut
	status, err = marshalWorkflow(ctx, _c.runner, _c.flowOptions, MyWorkflow.URL(), MyWorkflowIn{
		InputField1: inputField1,
		InputField2: inputField2,
	}, &out)
	return out.OutputField1, out.OutputField2, status, err
}
```

#### Step 7: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`. All workflow graph functions have the fixed signature `MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error)`.

```go
type ToDo interface {
	// ...
	MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error) // MARKER: MyWorkflow
}
```

#### Step 8: Implement the Logic

Implement the workflow graph builder in `service.go`. Use the `workflow.NewGraph` builder API to construct the graph. Reference task endpoints from this or other microservices using their `URL()` method.

Each node in the graph carries both a short **name** (used in transitions) and a **URL** (used to dispatch the task at runtime). Register nodes with `graph.AddTask("name", taskURL)`, then write transitions in terms of names. Use camelCase names that match the task endpoint (e.g. `"taskA"` for `TaskA`). Child workflows are not graph nodes; they are launched dynamically from a task body via `flow.Subgraph(childWorkflow.URL(), input)` - see the "Subgraphs and Interrupts" section in `.claude/rules/workflows.txt`.

```go
/*
MyWorkflow does X.
*/
func (svc *Service) MyWorkflow(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: MyWorkflow
	graph = workflow.NewGraph(myserviceapi.MyWorkflow.URL())
	graph.AddTask("taskA", myserviceapi.TaskA.URL())
	graph.AddTask("taskB", myserviceapi.TaskB.URL())
	graph.AddTask("taskC", myserviceapi.TaskC.URL())
	// graph.AddTransition("taskA", "taskB")
	// graph.AddTransitionWhen("taskB", workflow.END, "done == true")
	// graph.AddTransitionGoto("taskB", "taskC")
	return graph, nil
}
```

State flows through the entire workflow unfiltered - the `MyWorkflowIn`/`MyWorkflowOut` structs from Step 5 are documentation (OpenAPI, the runner UI, the Executor signature), not runtime contracts. If a subgraph call needs its input or output adapted across a contract boundary, do it in any ordinary task immediately upstream or downstream of the subgraph using `flow.Transform`/`Delete`/`Clear` - see the "State Transformation Around a Subgraph" section in `.claude/rules/workflows.txt`. If the workflow's terminal state needs scrubbing before it lands in `final_state`, the last task calls `flow.Delete`/`Transform`.

Naming the same task URL twice with different names is the supported way to reuse a task at multiple positions in the graph (each position keeps its own node identity for fan-in tracking).

**Every fan-out requires a matching fan-in.** Any node with two or more normal outgoing transitions, or an `AddTransitionForEach` transition, is a fan-out: its branches run in parallel. Every fan-out must converge on a single node that you mark with `graph.SetFanIn("name")`. Add the `SetFanIn` call in the same edit as the fan-out transitions, and route every parallel branch into the marked node before the graph reaches `workflow.END`. A graph that fans out without a `SetFanIn` node still compiles and passes `go vet`, but `graph.Validate()` (Step 9) rejects it at run time, so the test in Step 12 fails. This is the single most common workflow-graph mistake. The fan-out/fan-in section in `.claude/rules/workflows.txt` has the full rule and a worked example.

**Reducers for fan-in fields.** When parallel branches converge, each field that needs anything other than last-write-wins must be explicitly wired with `graph.SetReducer(field, reducer)` at graph-build time. Fields without a registered reducer use `Replace` (last write wins). No name-driven inference - a field named `messages` and a field named `total` are treated identically until `SetReducer` says otherwise.

```go
graph.SetReducer("messages", workflow.ReducerAppend) // accumulate per-branch deltas
graph.SetReducer("total",    workflow.ReducerAdd)    // sum numeric contributions
graph.SetReducer("seen",     workflow.ReducerUnion)  // dedupe across branches
graph.SetReducer("attrs",    workflow.ReducerMerge)  // merge per-branch objects
graph.SetReducer("approved", workflow.ReducerAnd)    // all branches must approve
graph.SetReducer("flagged",  workflow.ReducerOr)     // any branch flags
graph.SetReducer("notes",    workflow.ReducerConcat) // join string deltas
```

Pick the reducer by what the merge means, not by what the field is named. The default `Replace` is correct when only one branch ever writes the field.

**Prefer `AddTransitionWhen` for routing the graph knows about; reserve `AddTransitionGoto` for runtime loops the task body decides.** A `When` transition is part of the static graph - the validator sees it, the Mermaid diagram renders it as a labeled branch, and lineage scoping handles it like any other fan-out. A `Goto` is an out-of-band edge that's only taken when the task calls `flow.Goto`. The canonical use for `Goto` is a fan-in node looping back ("ask for more info, retry the review"); for any branch a `When` expression can evaluate, prefer `When`.

**Optionally annotate nodes with their business meaning.** `graph.Annotate("taskName", "short note")` adds a teal note beneath the node in the rendered Mermaid diagram, explaining what the step *does in business terms* (not how it's implemented). See the "Annotating Tasks" section of `.claude/rules/workflows.txt` for what makes a good annotation and what to skip.

#### Step 9: Define the Marshaler Function

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

#### Step 10: Bind the Marshaler Function to the Microservice

Bind the `doMyWorkflow` marshaler function to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the `// HINT: Add graph endpoints here` comment. If other subscriptions already exist under this HINT, add the new one after the last existing subscription.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: MyWorkflow
"MyWorkflow", svc.doMyWorkflow,
		sub.At(myserviceapi.MyWorkflow.Method, myserviceapi.MyWorkflow.Route),
sub.Description(`MyWorkflow does X.`),
		sub.Workflow(myserviceapi.MyWorkflowIn{}, myserviceapi.MyWorkflowOut{}),
	)

	// ...
}
```

The first argument to `svc.Subscribe` is the workflow name (must be a Go identifier starting with an uppercase letter). The `sub.Description` carries the godoc text from Step 4. `sub.Workflow(In{}, Out{})` declares the feature type and the input/output struct types - these flow through to the connector's built-in OpenAPI document so the workflow can be exposed as an LLM tool.

Add `sub.RequiredClaims(requiredClaims)` to `svc.Subscribe` to define the authorization requirements of the workflow endpoint. Omit to allow all requests.

#### Step 11: Regenerate the Mock

Run `go run github.com/microbus-io/fabric/cmd/genmock --path .` from the microservice's directory. This regenerates both `mock.go` and `mock_test.go`.

#### Step 12: Test the Workflow

Append the integration test to `service_test.go`. The test includes the foreman service in the app to enable end-to-end workflow execution.

Ensure that `"github.com/microbus-io/fabric/coreservices/foreman"`, `"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"`, and `"github.com/microbus-io/fabric/workflow"` are imported in `service_test.go`. Add them if not already present. (foreman is needed to add the foreman microservice to the test bundle, foremanapi for `NewClient`, and workflow for the status constants like `workflow.StatusCompleted`.)

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
				status, workflow.StatusCompleted,
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
		status, workflow.StatusCompleted,
		outputField1, expectedValue1,
		outputField2, expectedValue2,
	)
})
```

#### Step 13: Housekeeping

Follow the `housekeeping` skill.

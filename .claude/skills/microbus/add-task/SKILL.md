---
name: add-task
description: TRIGGER when user asks to add a workflow step, task endpoint, or workflow phase. Tasks are handlers that read/write shared state via workflow.Flow. Affects intermediate.go, *api/client.go, mock.go, manifest.yaml.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**IMPORTANT**: Read `.claude/rules/workflows.txt` for workflow and task conventions before proceeding.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a task endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Extend the ToDo interface
- [ ] Step 4: Determine the route
- [ ] Step 5: Determine a description
- [ ] Step 6: Determine the required claims
- [ ] Step 7: Define complex types
- [ ] Step 8: Define the endpoint and payload structs
- [ ] Step 9: Extend the executor
- [ ] Step 10: Implement the logic
- [ ] Step 11: Define the marshaler function
- [ ] Step 12: Bind the marshaler function to the microservice
- [ ] Step 13: Extend the mock
- [ ] Step 14: Test the task
- [ ] Step 15: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the task endpoint. A task always receives `ctx context.Context` and `flow *workflow.Flow` as its first two arguments, followed by state fields it reads as input. It returns state fields it writes as output, plus `err error`.

```go
func MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error)
```

Constraints:
- The first argument must be `ctx context.Context`
- The second argument must be `flow *workflow.Flow`
- The function must return an `err error`
- All input arguments (after `flow`) represent state fields read from the workflow state
- All output arguments (except `err`) represent state fields written to the workflow state
- To read and modify the same state field, use the `Out` suffix on the return value - the intermediate strips `Out` to map back to the same state key (e.g. input `counter int` and output `counterOut int` both map to state key `"counter"`)
- Complex types (structs) are allowed by value or by reference
- All arguments must be serializable into JSON
- Arguments must not be named `t` or `svc`
- Argument names must start with a lowercase letter
- The function name must start with an uppercase letter

#### Step 3: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error) // MARKER: MyTask
}
```

#### Step 4: Determine the Route

The route of the task endpoint is resolved relative to the hostname of the microservice. Tasks use the dedicated port `:428` to prevent external access. Use the name of the task in kebab-case as its route, e.g. `:428/my-task`.

#### Step 5: Determine a Description

Describe the task starting with its name, in Go doc style: `MyTask does X`. Embed this description in followup steps wherever you see `MyTask does X`.

#### Step 6: Determine the Required Claims

Determine if the task endpoint should be restricted to authorized actors only. Compose a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied. For example: `roles.manager && level>2`. Leave empty if the task should be accessible by all.

#### Step 7: Define Complex Types

Identify the struct types in the signature. These complex types must be defined in the `myserviceapi` directory because they are part of the public API of the microservice. Skip this step if there are no complex types.

Place each definition in a separate file named after the type, e.g. `myserviceapi/mystruct.go`.

If the complex type is owned by this microservice, define its struct explicitly. Be sure to include `json` tags with camelCase names and the `omitzero` option. Add short `jsonschema` description tags to each field to improve OpenAPI documentation and LLM tool-calling accuracy.

```go
package myserviceapi

// MyStruct is X.
type MyStruct struct {
	FooField string `json:"fooField,omitzero" jsonschema:"description=FooField is X"`
	BarField int    `json:"barField,omitzero" jsonschema:"description=BarField is X"`
}
```

If the complex type is owned by another microservice, define an alias to it instead.

```go
package myserviceapi

import (
	"github.com/path/to/thirdparty"
)

// ThirdPartyStruct is X.
type ThirdPartyStruct = thirdparty.ThirdPartyStruct
```

#### Step 8: Define the Endpoint and Payload Structs

Append the task's payload structs at the end of `myserviceapi/endpoints.go`. Use PascalCase for the field names and camelCase for the `json` tag names.

`MyTaskIn` holds the input arguments of the task, excluding `ctx context.Context` and `flow *workflow.Flow`.

```go
// MyTaskIn are the input arguments of MyTask.
type MyTaskIn struct { // MARKER: MyTask
	Input1 string  `json:"input1,omitzero"`
	Input2 float64 `json:"input2,omitzero"`
}
```

`MyTaskOut` holds the output arguments of the task, excluding `err error`. For fields with the `Out` suffix, strip the suffix from the JSON tag name so it maps to the same state key as the input.

```go
// MyTaskOut are the output arguments of MyTask.
type MyTaskOut struct { // MARKER: MyTask
	Output1 bool `json:"output1,omitzero"`
}
```

Append the endpoint definition to the `var` block in `myserviceapi/endpoints.go`, after the corresponding `HINT` comment. Tasks always use the `POST` method. The `Def` struct carries only the `Method` and `Route` from Step 4; the endpoint name, description, input/output schemas, and required claims are wired up via `svc.Subscribe` in `intermediate.go` (Step 12).

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyTask = Def{Method: "POST", Route: ":428/my-task"} // MARKER: MyTask
)
```

#### Step 9: Extend the Executor

Append the following Executor method at the end of `myserviceapi/client.go`. This method calls the task endpoint directly. The signature mirrors the task's own input/output arguments (without `flow`), plus `err error`.

```go
/*
MyTask creates and runs the MyTask task.
*/
func (_c Executor) MyTask(ctx context.Context, input1 string, input2 float64) (output1 bool, err error) { // MARKER: MyTask
	var out MyTaskOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, MyTask.Method, MyTask.Route, MyTaskIn{
		Input1: input1,
		Input2: input2,
	}, &out, _c.inFlow, _c.outFlow)
	return out.Output1, err // No trace
}
```

#### Step 10: Implement the Logic

Implement the task in `service.go`. Complex types should always refer to their definition in `myserviceapi`, even if owned by a third-party.

The task receives state fields as input arguments and returns state fields as output. It also has access to `flow` for control operations (`flow.Goto()`, `flow.Interrupt()`, `flow.Retry()`, `flow.Sleep()`) and for field-based state access (`flow.GetString()`, `flow.Set()`) when needed.

```go
/*
MyTask does X.
*/
func (svc *Service) MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error) { // MARKER: MyTask
	// Implement logic here...
	return
}
```

#### Step 11: Define the Marshaler Function

Append a web handler at the end of `intermediate.go` to perform the marshaling.

```go
// doMyTask handles marshaling for MyTask.
func (svc *Intermediate) doMyTask(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyTask
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in myserviceapi.MyTaskIn
	flow.ParseState(&in)
	var out myserviceapi.MyTaskOut
	out.Output1, err = svc.MyTask(r.Context(), &flow, in.Input1, in.Input2)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

#### Step 12: Bind the Marshaler Function to the Microservice

Bind the `doMyTask` marshaler function to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment. If other subscriptions already exist under this HINT, add the new one after the last existing subscription.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: MyTask
"MyTask", svc.doMyTask,
		sub.At(myserviceapi.MyTask.Method, myserviceapi.MyTask.Route),
sub.Description(`MyTask does X.`),
		sub.Task(myserviceapi.MyTaskIn{}, myserviceapi.MyTaskOut{}),
	)

	// ...
}
```

The first argument to `svc.Subscribe` is the task name (must be a Go identifier starting with an uppercase letter). The `sub.Description` carries the godoc text from Step 5. `sub.Task(In{}, Out{})` declares the feature type and the input/output struct types - the input struct's fields are read from the workflow state on entry, and the output struct's fields are written back on exit.

Add `sub.RequiredClaims(requiredClaims)` to `svc.Subscribe` to define the authorization requirements of the task endpoint. Omit to allow all requests.

Tasks are NOT exposed via OpenAPI - the connector's built-in `:888/openapi.json` handler filters them out automatically.

#### Step 13: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyTask func(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error) // MARKER: MyTask
}
```

Add the stubs to the `Mock`.

```go
// MockMyTask sets up a mock handler for MyTask.
func (svc *Mock) MockMyTask(handler func(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error)) *Mock { // MARKER: MyTask
	svc.mockMyTask = handler
	return svc
}

// MyTask executes the mock handler.
func (svc *Mock) MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error) { // MARKER: MyTask
	if svc.mockMyTask == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	output1, err = svc.mockMyTask(ctx, flow, input1, input2)
	return output1, errors.Trace(err)
}
```

Add a test case at the end of `TestMyService_Mock` in `service_test.go`, after the last existing test case.

- Set values for the example input arguments
- Set values for the expected output arguments

```go
t.Run("my_task", func(t *testing.T) { // MARKER: MyTask
	assert := testarossa.For(t)

	exampleParam1 := ""
	exampleParam2 := 0.0
	expectedResult1 := false

	_, err := mock.MyTask(ctx, nil, exampleParam1, exampleParam2)
	assert.Contains(err.Error(), "not implemented")
	mock.MockMyTask(func(ctx context.Context, flow *workflow.Flow, input1 string, input2 float64) (output1 bool, err error) {
		return expectedResult1, nil
	})
	output1, err := mock.MyTask(ctx, nil, exampleParam1, exampleParam2)
	assert.Expect(
		output1, expectedResult1,
		err, nil,
	)
})
```

#### Step 14: Test the Task

Append the integration test to `service_test.go`. The test calls the task endpoint directly via the Executor without needing the foreman.

```go
func TestMyService_MyTask(t *testing.T) { // MARKER: MyTask
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := myserviceapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			output1, err := exec.WithOutputFlow(&outFlow).MyTask(ctx, input1, input2)
			if assert.NoError(err) {
				assert.Expect(output1, expectedResult1)
				_, interrupted := outFlow.InterruptRequested()
				assert.Expect(interrupted, true)
			}
		})
	*/
}
```

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

- Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	output1, err := exec.MyTask(ctx, input1, input2)
	if assert.NoError(err) {
		assert.Expect(output1, expectedResult1)
	}
})
```

#### Step 15: Housekeeping

Follow the `housekeeping` skill.

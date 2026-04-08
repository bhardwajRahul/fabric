---
name: Adding a Task Endpoint
description: Creates or modifies a task endpoint for use in agentic workflows. Use when explicitly asked by the user to create or modify a task, a workflow step or workflow phase of a microservice.
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
- [ ] Step 8: Extend the API
- [ ] Step 9: Implement the logic
- [ ] Step 10: Define the marshaler function
- [ ] Step 11: Bind the marshaler function to the microservice
- [ ] Step 12: Extend the mock
- [ ] Step 13: Test the task
- [ ] Step 14: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the task endpoint. A task always receives `ctx context.Context` and `flow *workflow.Flow` as its first two arguments, followed by state fields it reads as input. It returns state fields it writes as output, plus `err error`.

```go
func MyTask(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error)
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
	MyTask(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error) // MARKER: MyTask
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

#### Step 8: Extend the API

Define the endpoint in the `var` block at the top of `myserviceapi/client.go`, after the corresponding `HINT` comment. Tasks use `POST` method.

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyTask = Def{Method: "POST", Route: ":428/my-task"} // MARKER: MyTask
)
```

Append the task's payload structs at the end of `myserviceapi/client.go`.
Use PascalCase for the field names and camelCase for the `json` tag names.

`MyTaskIn` holds the input arguments of the task, excluding `ctx context.Context` and `flow *workflow.Flow`.

```go
// MyTaskIn are the input arguments of MyTask.
type MyTaskIn struct { // MARKER: MyTask
	Param1 string  `json:"param1,omitzero"`
	Param2 float64 `json:"param2,omitzero"`
}
```

`MyTaskOut` holds the output arguments of the task, excluding `err error`. For fields with the `Out` suffix, strip the suffix from the JSON tag name so it maps to the same state key as the input.

```go
// MyTaskOut are the output arguments of MyTask.
type MyTaskOut struct { // MARKER: MyTask
	Result1 bool `json:"result1,omitzero"`
}
```

Append the following Executor method at the end of `myserviceapi/client.go`. This method calls the task endpoint directly. The signature mirrors the task's own input/output arguments (without `flow`), plus `err error`.

```go
/*
MyTask creates and runs the MyTask task.
*/
func (_c Executor) MyTask(ctx context.Context, param1 string, param2 float64) (result1 bool, err error) { // MARKER: MyTask
	var out MyTaskOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, MyTask.Method, MyTask.Route, MyTaskIn{
		Param1: param1,
		Param2: param2,
	}, &out, _c.inFlow, _c.outFlow)
	return out.Result1, err // No trace
}
```

#### Step 9: Implement the Logic

Implement the task in `service.go`. Complex types should always refer to their definition in `myserviceapi`, even if owned by a third-party.

The task receives state fields as input arguments and returns state fields as output. It also has access to `flow` for control operations (`flow.Goto()`, `flow.Interrupt()`, `flow.Retry()`, `flow.Sleep()`) and for field-based state access (`flow.GetString()`, `flow.Set()`) when needed.

```go
/*
MyTask does X.
*/
func (svc *Service) MyTask(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error) { // MARKER: MyTask
	// Implement logic here...
	return
}
```

#### Step 10: Define the Marshaler Function

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
	out.Result1, err = svc.MyTask(r.Context(), &flow, in.Param1, in.Param2)
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

#### Step 11: Bind the Marshaler Function to the Microservice

Bind the `doMyTask` marshaler function to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment. If other subscriptions already exist under this HINT, add the new one after the last existing subscription.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...

	// HINT: Add task endpoints here
	svc.Subscribe(myserviceapi.MyTask.Method, myserviceapi.MyTask.Route, svc.doMyTask) // MARKER: MyTask

	// ...
}
```

Add the following options to `svc.Subscribe` as needed:

- `sub.RequiredClaims(requiredClaims)` to define the authorization requirements of the task endpoint. Omit to allow all requests

**Note**: Tasks are NOT exposed via OpenAPI. Do not register tasks in `doOpenAPI`.

#### Step 12: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyTask func(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error) // MARKER: MyTask
}
```

Add the stubs to the `Mock`.

```go
// MockMyTask sets up a mock handler for MyTask.
func (svc *Mock) MockMyTask(handler func(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error)) *Mock { // MARKER: MyTask
	svc.mockMyTask = handler
	return svc
}

// MyTask executes the mock handler.
func (svc *Mock) MyTask(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error) { // MARKER: MyTask
	if svc.mockMyTask == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	result1, err = svc.mockMyTask(ctx, flow, param1, param2)
	return result1, errors.Trace(err)
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
	mock.MockMyTask(func(ctx context.Context, flow *workflow.Flow, param1 string, param2 float64) (result1 bool, err error) {
		return expectedResult1, nil
	})
	result1, err := mock.MyTask(ctx, nil, exampleParam1, exampleParam2)
	assert.Expect(
		result1, expectedResult1,
		err, nil,
	)
})
```

#### Step 13: Test the Task

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
			result1, err := exec.WithOutputFlow(&outFlow).MyTask(ctx, param1, param2)
			if assert.NoError(err) {
				assert.Expect(result1, expectedResult1)
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

	result1, err := exec.MyTask(ctx, param1, param2)
	if assert.NoError(err) {
		assert.Expect(result1, expectedResult1)
	}
})
```

#### Step 14: Housekeeping

Follow the `microbus/housekeeping` skill.

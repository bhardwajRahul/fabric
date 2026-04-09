---
name: add-function
description: TRIGGER when user asks to add, create, or modify an API endpoint, function, or RPC. Also when adding a route that accepts typed arguments and returns typed results. Affects intermediate.go, *api/client.go, mock.go, manifest.yaml.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a functional endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Extend the ToDo interface
- [ ] Step 4: Determine the method and route
- [ ] Step 5: Determine a description
- [ ] Step 6: Determine the required claims
- [ ] Step 7: Define complex types
- [ ] Step 8: Define the endpoint and payload structs
- [ ] Step 9: Extend the clients
- [ ] Step 10: Implement the logic
- [ ] Step 11: Define the marshaler function
- [ ] Step 12: Bind the marshaler function to the microservice
- [ ] Step 13: Extend the mock
- [ ] Step 14: Test the function
- [ ] Step 15: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the functional endpoint.

```go
func MyFunction(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]MyStruct, err error)
```

Constraints:
- The first input argument must be `ctx context.Context`
- The function must return an `err error`
- Maps must be keyed by string, e.g. `map[string]any`
- Complex types (structs) are allowed by value or by reference, e.g. `MyStruct` or `*MyStruct`
- All input or output arguments must be serializable into JSON, including complex types
- Arguments must not be named `t` or `svc`
- Argument names must start with a lowercase letter
- The function name must start with an uppercase letter
- A return argument named `httpStatusCode` must be of type `int`
- If the return argument `httpResponseBody` is present, no other return argument other than `httpStatusCode` and `error` can be present
- The magic HTTP argument names `httpRequestBody`, `httpResponseBody` and `httpStatusCode` are documented in the rules file under "Magic HTTP Arguments"

#### Step 3: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyFunction(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) // MARKER: MyFunction
}
```

#### Step 4: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. Unless there's a reason to use a specific method, like for a REST API, use `ANY` to accept requests with any method.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-function`.

To set a port other than the default 443, prefix the route with the port, e.g. `:1234/my-function`.

Encase path arguments with `{}` , e.g. `/section/{section}/page/{page...}`.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:1234/on-something`

#### Step 5: Determine a Description

Describe the endpoint starting with its name, in Go doc style: `MyFunction does X`. Embed this description in followup steps wherever you see `MyFunction does X`.

When the function has non-trivial parameters or results, include `Input:` and/or `Output:` sections in the description. These sections are extracted by the OpenAPI generator to populate per-field descriptions. Use the godoc indented list format:

```
MyFunction does X.

Input:
  - input1: input1 is X
  - input2: input2 is X

Output:
  - output1: output1 is X
```

#### Step 6: Determine the Required Claims

Determine if the endpoint should be restricted to authorized actors only. Compose a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied. For example: `roles.manager && level>2`. Leave empty if the endpoint should be accessible by all.

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

Append the function's payload structs at the end of `myserviceapi/endpoints.go`. Use PascalCase for the field names and camelCase for the `json` tag names.

`MyFunctionIn` holds the input arguments of the function, excluding `ctx context.Context`. If an argument is named `httpRequestBody`, set its `json` tag value to `-`.

```go
// MyFunctionIn are the input arguments of MyFunction.
type MyFunctionIn struct { // MARKER: MyFunction
	Input1 string           `json:"input1,omitzero"`
	Input2 ThirdPartyStruct `json:"input2,omitzero"`
}
```

`MyFunctionOut` holds the output arguments of the function, excluding `err error`. If an argument is named `httpStatusCode`, set its `json` tag value to `-`.

```go
// MyFunctionOut are the output arguments of MyFunction.
type MyFunctionOut struct { // MARKER: MyFunction
	Output1 map[string]MyStruct `json:"output1,omitzero"`
}
```

Append the endpoint definition to the `var` block in `myserviceapi/endpoints.go`, after the corresponding `HINT` comment. The `Def` struct carries only the `Method` and `Route` from Step 4; the endpoint name, description, input/output schemas, and required claims are wired up via `svc.Subscribe` in `intermediate.go` (Step 12).

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	MyFunction = Def{Method: "ANY", Route: "/my-function"} // MARKER: MyFunction
)
```

#### Step 9: Extend the Clients

Append the response wrapper at the end of `myserviceapi/client.go`. `MyFunctionResponse` holds the response of the request and provides a single method `Get` that returns the function's return arguments.

```go
// MyFunctionResponse packs the response of MyFunction.
type MyFunctionResponse multicastResponse // MARKER: MyFunction

// Get unpacks the return arguments of MyFunction.
func (_res *MyFunctionResponse) Get() (output1 map[string]MyStruct, err error) { // MARKER: MyFunction
	_d := _res.data.(*MyFunctionOut)
	return _d.Output1, _res.err
}
```

Append the following client methods at the end of `myserviceapi/client.go`.

```go
/*
MyFunction does X.

Input:
  - input1: input1 is X
  - input2: input2 is X

Output:
  - output1: output1 is X
*/
func (_c MulticastClient) MyFunction(ctx context.Context, input1 string, input2 ThirdPartyStruct) iter.Seq[*MyFunctionResponse] { // MARKER: MyFunction
	_in := MyFunctionIn{Input1: input1, Input2: input2}
	_out := MyFunctionOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MyFunction.Method, MyFunction.Route, &_in, &_out)
	return func(yield func(*MyFunctionResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MyFunctionResponse)(_r)) {
				return
			}
		}
	}
}

/*
MyFunction does X.

Input:
  - input1: input1 is X
  - input2: input2 is X

Output:
  - output1: output1 is X
*/
func (_c Client) MyFunction(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]MyStruct, err error) { // MARKER: MyFunction
	_in := MyFunctionIn{Input1: input1, Input2: input2}
	_out := MyFunctionOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MyFunction.Method, MyFunction.Route, &_in, &_out)
	return _out.Output1, err // No trace
}
```

#### Step 10: Implement the Logic

Implement the function in `service.go`. Complex types should always refer to their definition in `myserviceapi`, even if owned by a third-party. Include the full description with `Input:` and `Output:` sections if determined in Step 5.

```go
/*
MyFunction does X.

Input:
  - input1: input1 is X
  - input2: input2 is X

Output:
  - output1: output1 is X
*/
func (svc *Service) MyFunction(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) { // MARKER: MyFunction
	// Implement logic here...
	return
}
```

#### Step 11: Define the Marshaler Function

Append a web handler at the end of `intermediate.go` to perform the marshaling of the input and output arguments.

```go
// doMyFunction handles marshaling for MyFunction.
func (svc *Intermediate) doMyFunction(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyFunction
	var in myserviceapi.MyFunctionIn
	var out myserviceapi.MyFunctionOut
	err = marshalFunction(w, r, myserviceapi.MyFunction.Route, &in, &out, func(_ any, _ any) error {
		out.Output1, err = svc.MyFunction(r.Context(), in.Input1, in.Input2)
		return err // No trace
	})
	return err // No trace
}
```

#### Step 12: Bind the Marshaler Function to the Microservice

Bind the `doMyFunction` marshaler function to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment. If other subscriptions already exist under this HINT, add the new one after the last existing subscription.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...
	svc.Subscribe( // MARKER: MyFunction
"MyFunction", svc.doMyFunction,
		sub.At(myserviceapi.MyFunction.Method, myserviceapi.MyFunction.Route),
sub.Description(`MyFunction does X.

Input:
  - input1: input1 is X
  - input2: input2 is X

Output:
  - output1: output1 is X`),
		sub.Function(myserviceapi.MyFunctionIn{}, myserviceapi.MyFunctionOut{}),
	)
	// ...
}
```

The first argument to `svc.Subscribe` is the endpoint name (must be a Go identifier starting with an uppercase letter). The `sub.Description` carries the full godoc text from Step 5, including any `Input:` / `Output:` sections - the connector's built-in OpenAPI handler reads it from the subscription. `sub.Function(In{}, Out{})` declares the input/output struct types so the OpenAPI handler can reflect their schema.

Add the following options to `svc.Subscribe` as needed:

- A queue option to control how requests are distributed among replicas of the microservice
  - `sub.DefaultQueue()`: requests are load balanced among peers and processed by only one. This is the default and may be omitted
  - `sub.NoQueue()`: requests are processed by all subscribers
  - `sub.Queue(queueName)`: requests are load balanced among peers associated with this queue name. Subscribers associated with other queue names receive the requests separately based on their own queue option
- `sub.RequiredClaims(requiredClaims)` to define the authorization requirements of the endpoint. Omit to allow all requests

#### Step 13: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyFunction func(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) // MARKER: MyFunction
}
```

Add the stubs to the `Mock`.

```go
// MockMyFunction sets up a mock handler for MyFunction.
func (svc *Mock) MockMyFunction(handler func(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error)) *Mock { // MARKER: MyFunction
	svc.mockMyFunction = handler
	return svc
}

// MyFunction executes the mock handler.
func (svc *Mock) MyFunction(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) { // MARKER: MyFunction
	if svc.mockMyFunction == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	output1, err = svc.mockMyFunction(ctx, input1, input2)
	return output1, errors.Trace(err)
}
```

Add a test case at the end of `TestMyService_Mock` in `service_test.go`, after the last existing test case.

- Set values for the example input arguments
- Set values for the expected output arguments

```go
t.Run("my_function", func(t *testing.T) { // MARKER: MyFunction
	assert := testarossa.For(t)

	exampleParam1 := ""
	exampleParam2 := myserviceapi.ThirdPartyStruct{}
	expectedResult1 := map[string]myserviceapi.MyStruct{}

	_, err := mock.MyFunction(ctx, exampleParam1, exampleParam2)
	assert.Contains(err.Error(), "not implemented")
	mock.MockMyFunction(func(ctx context.Context, input1 string, input2 myserviceapi.ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) {
		return expectedResult1, nil
	})
	output1, err := mock.MyFunction(ctx, exampleParam1, exampleParam2)
	assert.Expect(
		output1, expectedResult1,
		err, nil,
	)
})
```

#### Step 14: Test the Function

Append the integration test to `service_test.go`.

```go
func TestMyService_MyFunction(t *testing.T) { // MARKER: MyFunction
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			actor := jwt.MapClaims{}
			output1, err := client.WithOptions(pub.Actor(actor)).MyFunction(ctx, input1, input2)
			assert.Expect(
				output1, expectedResult1,
				err, nil,
			)
		})
	*/
}
```

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

- You may omit the `pub.Actor` option if the functional endpoint does not require claims.
- Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	actor := jwt.MapClaims{}
	output1, err := client.WithOptions(pub.Actor(actor)).MyFunction(ctx, input1, input2)
	assert.Expect(
		output1, expectedResult1,
		err, nil,
	)
})
```

#### Step 15: Housekeeping

Follow the `housekeeping` skill.

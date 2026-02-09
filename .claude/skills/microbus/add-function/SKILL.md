---
name: Adding a Functional Endpoint
description: Creates or modify a functional endpoint of a microservice. Use when explicitly asked by the user to create or modify a functional or RPC endpoint of a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

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
- [ ] Step 8: Define the payload structs
- [ ] Step 9: Extend the clients
- [ ] Step 10: Implement the logic
- [ ] Step 11: Define the marshaller function
- [ ] Step 12: Bind the marshaller function to the microservice
- [ ] Step 13: Expose the endpoint via OpenAPI
- [ ] Step 14: Extend the mock
- [ ] Step 15: Test the function
- [ ] Step 16: Update manifest
- [ ] Step 17: Document the microservice
- [ ] Step 18: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the functional endpoint.

```go
func MyFunction(ctx context.Context, inArg1 string, inArg2 ThirdPartyStruct) (outArg1 map[string]MyStruct, err error)
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

There are three magic variable names that are useful when constructing a REST API.

- An input argument `httpRequestBody` has the request's body read directly into it. Other input arguments, if present, are read from the query arguments of the request. Use this pattern when an endpoint accepts a single object that is defined elsewhere. Handling a `PUT` request is an example
- Similarly, an output argument `httpResponseBody` has the request's body read directly into it. No other output arguments other than `httpStatusCode` can be returned if `httpResponseBody` is present. Use this pattern when an endpoint returns a single object that is defined elsewhere. Handling a `GET` request is an example
- An output argument named `httpStatusCode` of type `int` will allow the function to set the request's status code. Returning `http.StatusCreated` after a `POST` request is an example

#### Step 3: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyFunction(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) { // MARKER: MyFunction
}
```

#### Step 4: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. Use `ANY` to accept requests with any method. The most common approach is to use `POST`.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-function`.

To set a port other than the default 443, prefix the route with the port, e.g. `:1234/my-function`.

Encase path arguments with `{}` , e.g. `/section/{section}/page/{page...}`.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:1234/on-something`

#### Step 5: Determine a Description

Describe the endpoint starting with its name, in Go doc style: `MyFunction does X`. Embed this description in followup steps where appropriate.

#### Step 6: Determine the Required Claims

Determine if the endpoint should be restricted to authorized actors only. Compose a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied. For example: `roles=~"manager" && level>2`. Leave empty if the endpoint should be accessible by all.

#### Step 7: Define Complex Types

Identify the struct types in the signature. These complex types must be defined in the `myserviceapi` directory because they are part of the public API of the microservice. Skip this step if there are no complex types.

Place each definition in a separate file named after the type, e.g. `myserviceapi/mystruct.go`.

If the complex type is owned by this microservice, define its struct explicitly. Be sure to include `json` tags with camelCase names and the `omitzero` option.

```go
package myserviceapi

// MyStruct is X.
type MyStruct struct {
	FooField string `json:"fooField,omitzero"`
	BarField int    `json:"barField,omitzero"`
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

#### Step 8: Define the Payload Structs

In `myserviceapi/client.go`, define a struct `MyFunctionIn` to hold the input arguments of the function, excluding `ctx context.Context`. Use PascalCase for the field names and camelCase for the `json` tag names. If an argument is named `httpRequestBody`, set its `json` tag value to `-`.

```go
// MyFunctionIn are the input arguments of MyFunction.
type MyFunctionIn struct { // MARKER: MyFunction
	InArg1 string           `json:"inArg1,omitzero"`
	InArg2 ThirdPartyStruct `json:"inArg2,omitzero"`
}
```

Also in `myserviceapi/client.go`, define a struct `MyFunctionOut` to hold the output arguments of the function, excluding `err error`. Use PascalCase for the field names and camelCase for the `json` tag names. If an argument is named `httpStatusCode`, set its `json` tag value to `-`. Append the definition at the end of the file.

```go
// MyFunctionOut are the output arguments of MyFunction.
type MyFunctionOut struct { // MARKER: MyFunction
	OutArg1 map[string]MyStruct `json:"outArg1,omitzero"`
}
```

Also in `myserviceapi/client.go`, define a struct `MyFunctionResponse` to hold the response of the request. The struct provides a single method `Get` that returns the functions return arguments. Append the definition at the end of the file.

```go
// MyFunctionResponse is the response to MyFunction.
type MyFunctionResponse struct { // MARKER: MyFunction
	data MyFunctionOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_res *MyFunctionResponse) Get() (argOut1 map[string]MyStruct, err error) { // MARKER: MyFunction
	return _res.data.ArgOut1, _res.err
}
```

#### Step 9: Extend the Clients

Extend the clients in `myserviceapi/client.go`.

Define the route and URL of the endpoint at the top of the file in the respective `const` and `var` blocks. 

```go
// Endpoint routes.
const (
	// ...
	RouteOfMyFunction = `/my-function` // MARKER: MyFunction
)

// Endpoint URLs.
var (
	// ...
	URLOfMyFunction = httpx.JoinHostAndPath(Hostname, RouteOfMyFunction) // MARKER: MyFunction
)
```

Append the methods at the bottom of the file.

- In the comments, replace `MyFunction does X` with the description of the endpoint
- If the method of the endpoint is `ANY` set the value of `_method` to `POST`. Otherwise, set the value of `_method` to the method of the endpoint

```go
/*
MyFunction does X.
*/
func (_c MulticastClient) MyFunction(ctx context.Context, argIn1 string, argIn2 ThirdPartyStruct) <-chan *MyFunctionResponse { // MARKER: MyFunction
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfMyFunction)
	_in := MyFunctionIn{
		ArgIn1: argIn1,
		ArgIn2: argIn2,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *MyFunctionResponse, 1)
		_res <- &MyFunctionResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	_res := make(chan *MyFunctionResponse, cap(_ch))
	for _i := range _ch {
		var _r MyFunctionResponse
		_httpRes, _err := _i.Get()
		_r.HTTPResponse = _httpRes
		if _err != nil {
			_r.err = _err // No trace
		} else {
			_err = httpx.ReadOutputPayload(_httpRes, &_r.data)
			if _err != nil {
				_r.err = errors.Trace(_err)
			}
		}
		_res <- &_r
	}
	close(_res)
	return _res
}

/*
MyFunction does X.
*/
func (_c Client) MyFunction(ctx context.Context, argIn1 string, argIn2 ThirdPartyStruct) (argOut1 map[string]MyStruct, err error) { // MARKER: MyFunction
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfMyFunction)
	_in := MyFunctionIn{
		ArgIn1: argIn1,
		ArgIn2: argIn2,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		err = _err // No trace
		return
	}
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _out MyFunctionOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.ArgOut1, nil
}
```

#### Step 10: Implement the Logic

Implement the function in `service.go`. Complex types should always refer to their definition in `myserviceapi`, even if owned by a third-party.

```go
/*
MyFunction does X.
*/
func (svc *Service) MyFunction(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) { // MARKER: MyFunction
	// Implement logic here...
	return nil, nil
}
```

#### Step 11: Define the Marshaller Function

In `intermediate.go`, add a web handler to perform the marshaling of the input and output arguments. For the route, set the same path you used in the previous step to bind the function.

```go
// doMyFunction handles marshaling for MyFunction.
func (svc *Intermediate) doMyFunction(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyFunction
	var i myserviceapi.MyFunctionIn
	var o myserviceapi.MyFunctionOut
	err = httpx.ReadInputPayload(r, myserviceapi.RouteOfMyFunction, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.ArgOut1, err = svc.MyFunction(r.Context(), i.ArgIn1, i.ArgIn2)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

#### Step 12: Bind the Marshaller Function to the Microservice

Bind the `doMyFunction` marshaller function to the microservice in the `NewIntermediate` constructor in `intermediate.go`.

- The first two arguments to `svc.Subscribe` are the method and route of the endpoint
- The queue option indicate how requests are distributed among replicas of the microservice
  - `sub.DefaultQueue()`: requests are load balanced among peers and processed by only one. This is the default option and may be omitted
  - `sub.NoQueue()`: requests are processed by all subscribers
  - `sub.Queue(queueName)`: requests are load balanced among peers associated with this queue name. Subscribers associated with other queue names receive the requests separately based on their own queue option
- The `sub.RequiredClaims(requiredClaims)` option defines the authorization requirements of the endpoint. This option can be omitted to allow all requests

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.Subscribe("POST", "/my-function", svc.doMyFunction, sub.LoadBalanced(), sub.RequiredClaims("true")) // MARKER: MyFunction
	// ...
}
```

#### Step 13: Expose the Endpoint via OpenAPI

Register the functional endpoint in `doOpenAPI` in `intermediate.go`.

- For a functional endpoint, the `Type` field should be set to `function`
- Set the simplified signature of the endpoint in the `Summary` field. Exclude the arguments `ctx context.Context`, `err error` and `httpStatusCode int`. Remove the argument names `httpRequestBody` and `httpResponseBody` but keep their types
- In the `Description` field, replace `MyFunction does X` with the description of the endpoint
- Set the `RequiredClaims` boolean expression, if relevant to this endpoint. Otherwise, omit the field or leave it empty

```go
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	// ...
	endpoints := []*openapi.Endpoint{
		// ...
		{ // MARKER: MyFunction
			Type:          "function",
			Name:          "MyFunction",
			Method:        "ANY",
			Route:         myserviceapi.RouteOfMyFunction,
			Summary:       "MyFunction(inArg1 string, inArg2 ThirdPartyStruct) (outArg1 map[string]MyStruct)",
			Description:   `MyFunction does X.`,
			RequiredClaims: ``,
			InputArgs:      myserviceapi.MyFunctionIn{},
			OutputArgs:     myserviceapi.MyFunctionOut{},
		},
	}
	// ...
}
```

#### Step 14: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyFunction func(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) // MARKER: MyFunction
}
```

Add the stubs to the `Mock`.

```go
// MockMyFunction sets up a mock handler for MyFunction.
func (svc *Mock) MockMyFunction(handler func(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error)) *Mock { // MARKER: MyFunction
	svc.mockMyFunction = handler
	return svc
}

// MyFunction executes the mock handler.
func (svc *Mock) MyFunction(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) { // MARKER: MyFunction
	if svc.mockMyFunction == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	argOut1, err = svc.mockMyFunction(ctx, argIn1, argIn2)
	return argOut1, errors.Trace(err)
}
```

Add a test case in `TestMyService_Mock`.

- Set values for the example input arguments
- Set values for the expected output arguments

```go
t.Run("my_function", func(t *testing.T) { // MARKER: MyFunction
	assert := testarossa.For(t)

	exampleArgIn1 := ""
	exampleArgIn2 := myserviceapi.ThirdPartyStruct{}
	expectedArgOut1 := map[string]myserviceapi.MyStruct{}

	_, err := mock.MyFunction(ctx, exampleArgIn1, exampleArgIn2)
	assert.Contains(err.Error(), "not implemented")
	mock.MockMyFunction(func(ctx context.Context, argIn1 string, argIn2 myserviceapi.ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) {
		return expectedArgOut1, nil
	})
	argOut1, err := mock.MyFunction(ctx, exampleArgIn1, exampleArgIn2)
	assert.Expect(
		argOut1, expectedArgOut1,
		err, nil,
	)
})
```

#### Step 15: Test the Function

Skip this step if instructed to be "quick" or to skip tests.

Append the following code block to the end of `service_test.go`.

- Do not remove comments with `HINT`s. They are there to guide you in the future.
- Insert test cases at the bottom of the test function using the recommended pattern.
- There is no need to set the `pub.Actor` option if the functional endpoint does not require claims.

```go
func TestMyService_MyFunction(t *testing.T) { // MARKER: MyFunction
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)

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
			argOut1, err := client.WithOptions(pub.Actor(actor)).MyFunction(ctx, argIn1, argIn2)
			assert.Expect(
				argOut1, expectedArgOut1,
				err, nil,
			)
		})
	*/
}
```

In `TestMyService_OpenAPI` in `service_test.go`, add the endpoint's port, if not already tested.

#### Step 16: Update Manifest

Update the `functions` and `downstream` sections of `manifest.yaml`.

#### Step 17: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 18: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.

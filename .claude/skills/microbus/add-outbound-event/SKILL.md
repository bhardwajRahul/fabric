---
name: Adding an Outbound Event Endpoint
description: Creates or modify an outbound event endpoint of a microservice. Use when explicitly asked by the user to create or modify an outbound event endpoint of a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**CRITICAL**: Do NOT register outbound events in `doOpenAPI`. Only functional endpoints and web handlers are exposed via OpenAPI.                        

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying an event endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Determine the method and route
- [ ] Step 4: Determine a description
- [ ] Step 5: Define complex types
- [ ] Step 6: Define the payload structs
- [ ] Step 7: Extend the trigger and hook
- [ ] Step 8: Trigger the event
- [ ] Step 9: Test the outgoing event
- [ ] Step 10: Update manifest
- [ ] Step 11: Document the microservice
- [ ] Step 12: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the outgoing event endpoint.

```go
func OnMyEvent(ctx context.Context, inArg1 string, inArg2 ThirdPartyStruct) (outArg1 map[string]MyStruct, err error)
```

Constraints:
- The first input argument must be `ctx context.Context`
- The function must return an `err error`
- Maps must be keyed by string, e.g. `map[string]any`
- Complex types (structs) are allowed by value or by reference, e.g. `MyStruct` or `*MyStruct`
- All input or output arguments must be serializable into JSON, including complex types
- Arguments must not be named `t` or `svc`
- Argument names must start with a lowercase letter
- The function name must start with `On` followed by an uppercase letter
- A return argument named `httpStatusCode` must be of type `int`
- If the return argument `httpResponseBody` is present, no other return argument other than `httpStatusCode` and `error` can be present

There are three magic variable names that are useful when constructing a REST API.

- An input argument `httpRequestBody` has the request's body read directly into it. Other input arguments, if present, are read from the query arguments of the request. Use this pattern when an endpoint accepts a single object that is defined elsewhere. Handling a `PUT` request is an example
- Similarly, an output argument `httpResponseBody` has the request's body read directly into it. No other output arguments other than `httpStatusCode` can be returned if `httpResponseBody` is present. Use this pattern when an endpoint returns a single object that is defined elsewhere. Handling a `GET` request is an example
- An output argument named `httpStatusCode` of type `int` will allow the function to set the request's status code. Returning `http.StatusCreated` after a `POST` request is an example

#### Step 3: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. The most common approach is to use `POST`.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-event`.

Events should be set on a dedicated port to allow blocking external requests from reaching them. The recommended port to use for events is 417. Prefix the route with the port, e.g. `:417/my-event`.

Do not use path arguments in events.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:417/on-something`

#### Step 4: Determine a Description

Describe the endpoint starting with its name, in Go doc style: `OnMyEvent is triggered when X`. Embed this description in followup steps where appropriate.

#### Step 5: Define Complex Types

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

#### Step 6: Define the Payload Structs

In `myserviceapi/client.go`, define a struct `OnMyEventIn` to hold the input arguments of the function, excluding `ctx context.Context`. Use PascalCase for the field names and camelCase for the `json` tag names. If an argument is named `httpRequestBody`, set its `json` tag value to `-`.

```go
// OnMyEventIn are the input arguments of OnMyEvent.
type OnMyEventIn struct { // MARKER: OnMyEvent
	InArg1 string           `json:"inArg1,omitzero"`
	InArg2 ThirdPartyStruct `json:"inArg2,omitzero"`
}
```

Also in `myserviceapi/client.go`, define a struct `OnMyEventOut` to hold the output arguments of the function, excluding `err error`. Use PascalCase for the field names and camelCase for the `json` tag names. If an argument is named `httpStatusCode`, set its `json` tag value to `-`. Append the definition at the end of the file.

```go
// OnMyEventOut are the output arguments of OnMyEvent.
type OnMyEventOut struct { // MARKER: OnMyEvent
	OutArg1 map[string]MyStruct `json:"outArg1,omitzero"`
}
```

Also in `myserviceapi/client.go`, define a struct `OnMyEventResponse` to hold the response of the request. The struct provides a single method `Get` that returns the functions return arguments. Append the definition at the end of the file.

```go
// OnMyEventResponse is the response to OnMyEvent.
type OnMyEventResponse struct { // MARKER: OnMyEvent
	data OnMyEventOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_res *OnMyEventResponse) Get() (argOut1 map[string]MyStruct, err error) { // MARKER: OnMyEvent
	return _res.data.ArgOut1, _res.err
}
```

#### Step 7: Extend the Trigger and Hook

Extend the trigger and hook in `myserviceapi/client.go`.

Define the route and URL of the endpoint at the top of the file in the respective `const` and `var` blocks. 

```go
// Endpoint routes.
const (
	// ...
	RouteOfOnMyEvent = `/on-my-event` // MARKER: OnMyEvent
)

// Endpoint URLs.
var (
	// ...
	URLOfOnMyEvent = httpx.JoinHostAndPath(Hostname, RouteOfOnMyEvent) // MARKER: OnMyEvent
)
```

Append the methods at the bottom of the file.

- In the comments, replace `OnMyEvent is triggered when X` with the description of the endpoint
- Set the value of `method` and `_method` to the method of the endpoint

```go
/*
OnMyEvent is triggered when X.
*/
func (_c MulticastTrigger) OnMyEvent(ctx context.Context, argIn1 string, argIn2 ThirdPartyStruct) <-chan *OnMyEventResponse { // MARKER: OnMyEvent
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfOnMyEvent)
	_in := OnMyEventIn{
		ArgIn1: argIn1,
		ArgIn2: argIn2,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *OnMyEventResponse, 1)
		_res <- &OnMyEventResponse{err: _err} // No trace
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
	_res := make(chan *OnMyEventResponse, cap(_ch))
	for _i := range _ch {
		var _r OnMyEventResponse
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
OnMyEvent is triggered when X.
*/
func (c Hook) OnMyEvent(handler func(ctx context.Context, argIn1 string, argIn2 ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error)) (func unsub() (err error), err error) {
	doOnMyEvent := func(w http.ResponseWriter, r *http.Request) error {
		var i OnMyEventIn
		var o OnMyEventOut
        err = httpx.ReadInputPayload(r, RouteOfOnMyEvent, &i)
        if err != nil {
            return errors.Trace(err)
        }
        o.ArgOut1, err = handler(r.Context(), i.ArgIn1, i.ArgIn2)
        if err != nil {
            return err // No trace
        }
        err = httpx.WriteOutputPayload(w, o)
        if err != nil {
            return errors.Trace(err)
        }
		return nil
	}
    method := "POST"
	path := httpx.JoinHostAndPath(c.host, RouteOfOnMyEvent)
	unsub, err = _c.svc.Subscribe(method, path, doOnMyEvent)
	return unsub, errors.Trace(err)
}
```

#### Step 8: Trigger the Event

The event itself does not have an implementation. Rather, trigger the event from within other endpoints using its trigger.

To fire an event and wait for zero or more responses, loop over the response channel.

```go
for r := range myserviceapi.NewMulticastTrigger(svc).OnMyEvent(ctx, argIn1, argIn2) {
    argOut1, err := r.Get()
    if err != nil {
        return errors.Trace(err)
    }
    // ...
}
```

To fire and forget, call the trigger inside a Go routine.

```go
svc.Go(ctx, func(ctx context.Context) (err error) {
    myserviceapi.NewMulticastTrigger(svc).OnMyEvent(ctx, argIn1, argIn2)
    return nil
})
```

#### Step 9: Test the Outgoing Event

Skip this step if instructed to be "quick" or to skip tests.

Append the following code block to the end of `service_test.go`.

- Do not remove comments with `HINT`s. They are there to guide you in the future.
- Insert test cases at the bottom of the test function using the recommended pattern.
- Enter distinct queue names in `sub.Queue` when hooking multiple times to the event to simulate multiple clients

```go
func TestMyService_OnMyEvent(t *testing.T) { // MARKER: OnMyEvent
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)
	trigger := myserviceapi.NewMulticastTrigger(tester)
	hook := myserviceapi.NewHook(tester)

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

			unsub, err := hook.WithOptions(sub.Queue("queue1")).OnMyEvent(
				func(ctx context.Context, argIn1 string, argIn2 ThirdPartyStruct) (argOut1 map[string]myserviceapi.MyStruct, err error) {
					assert.Expect(
						argIn1, expectedArgIn1,
						argIn2, expectedArgIn2,
					)
					// Implement event sink here...
					return argOut1, err
				},
			)
			if assert.NoError(err) {
				defer unsub()
			}

			for e := range trigger.OnMyEvent(ctx, argIn1, argIn2) {
				if frame.Of(e.HTTPResponse).FromHost() == tester.Hostname() {
					argOut1, err := e.Get()
					assert.Expect(
                        argOut1, expectedArgOut1,
						err, nil,
					)
				}
			}
		})
	*/
}
```

#### Step 10: Update Manifest

Update the `outboundEvents` and `downstream` sections of `manifest.yaml`.

#### Step 11: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 12: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.

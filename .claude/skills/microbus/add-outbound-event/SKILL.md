---
name: add-outbound-event
description: TRIGGER when user asks to fire, emit, or publish an event that other services can subscribe to. Creates event trigger and hook in *api/client.go. Affects intermediate.go, manifest.yaml.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**IMPORTANT**: Outbound events are not exposed via OpenAPI. The connector's built-in `:888/openapi.json` handler filters them out automatically.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying an event endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Determine the method and route
- [ ] Step 4: Determine a description
- [ ] Step 5: Define complex types
- [ ] Step 6: Define the endpoint and payload structs
- [ ] Step 7: Extend the trigger and hook
- [ ] Step 8: Trigger the event
- [ ] Step 9: Test the outgoing event
- [ ] Step 10: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Signature

Determine the Go signature of the outgoing event endpoint.

```go
func OnMyEvent(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]MyStruct, err error)
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

#### Step 3: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. The most common approach is to use `POST`.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-event`.

Events should be set on a dedicated port to allow blocking external requests from reaching them. The recommended port to use for events is 417. Prefix the route with the port, e.g. `:417/my-event`.

Do not use path arguments in events.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:417/on-something`

#### Step 4: Determine a Description

Describe the endpoint starting with its name, in Go doc style: `OnMyEvent is triggered when X`. Embed this description in followup steps wherever you see `OnMyEvent is triggered when X`.

#### Step 5: Define Complex Types

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

#### Step 6: Define the Endpoint and Payload Structs

Append the event's payload structs at the end of `myserviceapi/endpoints.go`. Use PascalCase for the field names and camelCase for the `json` tag names.

`OnMyEventIn` holds the input arguments of the function, excluding `ctx context.Context`.

```go
// OnMyEventIn are the input arguments of OnMyEvent.
type OnMyEventIn struct { // MARKER: OnMyEvent
	Input1 string           `json:"input1,omitzero"`
	Input2 ThirdPartyStruct `json:"input2,omitzero"`
}
```

`OnMyEventOut` holds the output arguments of the function, excluding `err error`.

```go
// OnMyEventOut are the output arguments of OnMyEvent.
type OnMyEventOut struct { // MARKER: OnMyEvent
	Output1 map[string]MyStruct `json:"output1,omitzero"`
}
```

Append the endpoint definition to the `var` block in `myserviceapi/endpoints.go`, after the corresponding `HINT` comment. The `Def` struct carries only the `Method` and `Route` from Step 3; the description and any required claims are wired up by consumers via the `Hook` they register through (Step 7) or - when this service triggers itself - at the call site (Step 8).

```go
var (
	// HINT: Insert endpoint definitions here
	// ...
	OnMyEvent = Def{Method: "POST", Route: ":417/on-my-event"} // MARKER: OnMyEvent
)
```

#### Step 7: Extend the Trigger and Hook

Append the response wrapper at the end of `myserviceapi/client.go`. `OnMyEventResponse` holds the response of the request and provides a single method `Get` that returns the event handler's return arguments.

If there are output arguments besides `err error`:

```go
// OnMyEventResponse packs the response of OnMyEvent.
type OnMyEventResponse multicastResponse // MARKER: OnMyEvent

// Get retrieves the return values.
func (_res *OnMyEventResponse) Get() (output1 map[string]MyStruct, err error) { // MARKER: OnMyEvent
	_d := _res.data.(*OnMyEventOut)
	return _d.Output1, _res.err
}
```

If `err error` is the only return argument:

```go
// OnMyEventResponse packs the response of OnMyEvent.
type OnMyEventResponse multicastResponse // MARKER: OnMyEvent

// Get retrieves the return values.
func (_res *OnMyEventResponse) Get() (err error) { // MARKER: OnMyEvent
	return _res.err
}
```

Append the following client methods at the end of `myserviceapi/client.go`.

```go
/*
OnMyEvent is triggered when X.
*/
func (_c MulticastTrigger) OnMyEvent(ctx context.Context, input1 string, input2 ThirdPartyStruct) iter.Seq[*OnMyEventResponse] { // MARKER: OnMyEvent
	_in := OnMyEventIn{Input1: input1, Input2: input2}
	_out := OnMyEventOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, OnMyEvent.Method, OnMyEvent.Route, &_in, &_out)
	return func(yield func(*OnMyEventResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*OnMyEventResponse)(_r)) {
				return
			}
		}
	}
}

/*
OnMyEvent is triggered when X.
*/
func (c Hook) OnMyEvent(handler func(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]MyStruct, err error)) (unsub func() error, err error) { // MARKER: OnMyEvent
	doOnMyEvent := func(w http.ResponseWriter, r *http.Request) error {
		var in OnMyEventIn
		var out OnMyEventOut
		err = marshalFunction(w, r, OnMyEvent.Route, &in, &out, func(_ any, _ any) error {
			out.Output1, err = handler(r.Context(), in.Input1, in.Input2)
			return err
		})
		return err // No trace
	}
	const name = "OnMyEvent"
	path := httpx.JoinHostAndPath(c.host, OnMyEvent.Route)
	subOpts := append([]sub.Option{
		sub.At(OnMyEvent.Method, path),
sub.InboundEvent(OnMyEventIn{}, OnMyEventOut{}),
	}, c.opts...)
	if err := c.svc.Subscribe(name, doOnMyEvent, subOpts...); err != nil {
		return nil, errors.Trace(err)
	}
	return func() error { return c.svc.Unsubscribe(name) }, nil
}
```

The Hook registers the inbound event with the consuming service via `c.svc.Subscribe` (the consuming service is whatever was passed to `NewHook`). The fixed `name = "OnMyEvent"` keys the registration in the consumer's `c.subs` map. The `sub.InboundEvent(In{}, Out{})` option tags the subscription as a feature type the connector's built-in OpenAPI handler will skip. Returning `c.svc.Unsubscribe(name)` lets the consumer remove the registration if needed (most callers discard the closure).

#### Step 8: Trigger the Event

The event itself does not have an implementation. Rather, trigger the event from within other endpoints using its trigger.

To fire an event and wait for zero or more responses, loop over the response channel.

```go
for r := range myserviceapi.NewMulticastTrigger(svc).OnMyEvent(ctx, input1, input2) {
	output1, err := r.Get()
	if err != nil {
		return errors.Trace(err)
	}
	// ...
}
```

To fire and forget, call the trigger without iterating over its response.

```go
myserviceapi.NewMulticastTrigger(svc).OnMyEvent(ctx, input1, input2)
```

#### Step 9: Test the Outgoing Event

Append the integration test to `service_test.go`.

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

			unsub, err := hook.WithOptions(sub.Queue("UniqueQueueName")).OnMyEvent(
				func(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) {
					assert.Expect(
						input1, expectedParam1,
						input2, expectedParam2,
					)
					// Implement event sink here...
					return output1, err
				},
			)
			if assert.NoError(err) {
				defer unsub()
			}

			for e := range trigger.OnMyEvent(ctx, input1, input2) {
				if frame.Of(e.HTTPResponse).FromHost() == tester.Hostname() {
					output1, err := e.Get()
					assert.Expect(
						output1, expectedResult1,
						err, nil,
					)
				}
			}
		})
	*/
}
```

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

- Enter distinct queue names in `sub.Queue` when hooking multiple times to the event to simulate multiple clients. Use only alphanumeric characters for queue names.
- Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	unsub, err := hook.WithOptions(sub.Queue("UniqueQueueName")).OnMyEvent(
		func(ctx context.Context, input1 string, input2 ThirdPartyStruct) (output1 map[string]myserviceapi.MyStruct, err error) {
			assert.Expect(
				input1, expectedParam1,
				input2, expectedParam2,
			)
			// Implement event sink here...
			return output1, err
		},
	)
	if assert.NoError(err) {
		defer unsub()
	}

	for e := range trigger.OnMyEvent(ctx, input1, input2) {
		if frame.Of(e.HTTPResponse).FromHost() == tester.Hostname() {
			output1, err := e.Get()
			assert.Expect(
				output1, expectedResult1,
				err, nil,
			)
		}
	}
})
```

#### Step 10: Housekeeping

Follow the `housekeeping` skill.

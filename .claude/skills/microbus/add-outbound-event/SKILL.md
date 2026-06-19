---
name: add-outbound-event
description: TRIGGER when user asks to fire, emit, or publish an event that other microservices can subscribe to.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: An outbound event is declared as a `define.OutboundEvent` var in `<name>api/definition.go`. Add the declaration and run `cmd/genservice`. An outbound event has no handler in `service.go` - this microservice fires it; other microservices sink it.

**CRITICAL**: Keep the `// MARKER: Name` comment on the `define.OutboundEvent` var and on its In/Out structs.

**IMPORTANT**: Outbound events are not exposed via OpenAPI. The connector's built-in `:888/openapi.json` handler filters them out automatically.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying an outbound event:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Determine the signature
- [ ] Step 3: Determine the method and route
- [ ] Step 4: Determine a description
- [ ] Step 5: Define complex types
- [ ] Step 6: Declare the event in definition.go
- [ ] Step 7: Generate the boilerplate
- [ ] Step 8: Trigger the event
- [ ] Step 9: Test the outgoing event
- [ ] Step 10: Housekeeping
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine the Signature

Determine the Go signature of the outgoing event.

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
- The event name must start with `On` followed by an uppercase letter

#### Step 3: Determine the Method and Route

The method of the endpoint determines the HTTP method with which it will be addressable. The most common approach is to use `POST`.

The route of the endpoint is resolved relative to the hostname of the microservice to determine how it is addressed. The common approach is to use the name of the endpoint in kebab-case as its route, e.g. `/my-event`.

Events should be set on a dedicated port to allow blocking external requests from reaching them. The recommended port to use for events is 417. Prefix the route with the port, e.g. `:417/on-my-event`.

Do not use path arguments in events.

Prefix the route with `//` to set a hostname other than that of this microservice, e.g. `//another.host.name:417/on-something`

#### Step 4: Determine a Description

Describe the event starting with its name, in Go doc style: `OnMyEvent is triggered when X`. This becomes the godoc comment on the `define.OutboundEvent` var.

#### Step 5: Define Complex Types

Identify the struct types in the signature. Define these complex types in the `myserviceapi` directory. Skip this step if there are no complex types.

Place each definition in a separate file named after the type, e.g. `myserviceapi/mystruct.go`.

If the complex type is owned by this microservice, define its struct explicitly. Include `json` tags with camelCase names and the `omitzero` option, and a short `jsonschema` description tag on each field.

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

#### Step 6: Declare the Event in `definition.go`

Append the `define.OutboundEvent` var and its In/Out structs to `myserviceapi/definition.go`.

```go
// OnMyEvent is triggered when X.
var OnMyEvent = define.OutboundEvent{ // MARKER: OnMyEvent
	Host: Hostname, Method: "POST", Route: ":417/on-my-event",
	In: OnMyEventIn{}, Out: OnMyEventOut{},
}

// OnMyEventIn are the input arguments of OnMyEvent.
type OnMyEventIn struct { // MARKER: OnMyEvent
	Input1 string           `json:"input1,omitzero"`
	Input2 ThirdPartyStruct `json:"input2,omitzero"`
}

// OnMyEventOut are the output arguments of OnMyEvent.
type OnMyEventOut struct { // MARKER: OnMyEvent
	Output1 map[string]MyStruct `json:"output1,omitzero"`
}
```

- `Host` is always `Hostname`. `Method` and `Route` come from Step 3
- The In struct holds the input arguments excluding `ctx`; the Out struct holds the output arguments excluding `err`
- If an In/Out field's type comes from another package (e.g. a `time.Time` field needs `"time"`), add that import to `definition.go`

#### Step 7: Generate the Boilerplate

From the microservice's directory, run the generator. It regenerates `myserviceapi/client.go` (the `MulticastTrigger`, the `Hook`, and the response wrapper for `OnMyEvent`) and `manifest.yaml` from the updated `definition.go`.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then verify the microservice compiles with `go vet ./...` from the project root.

#### Step 8: Trigger the Event

The event has no implementation of its own. Trigger it from within other endpoints using the generated trigger.

To fire an event and wait for zero or more responses, loop over the response sequence.

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

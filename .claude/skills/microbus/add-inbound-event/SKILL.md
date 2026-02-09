---
name: Adding an Inbound Event Sink Endpoint
description: Creates or modify an inbound event sink endpoint of a microservice. Use when explicitly asked by the user to create or modify an inbound event sink endpoint of a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

**CRITICAL**: Do NOT register inbound event sinks in `doOpenAPI`. Only functional endpoints and web handlers are exposed via OpenAPI.                        

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a sink endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Locate the source of the outbound event and determine signature
- [ ] Step 3: Determine a description
- [ ] Step 4: Determine the required claims
- [ ] Step 5: Extend the ToDo interface
- [ ] Step 6: Implement the inbound event sink logic
- [ ] Step 7: Bind the inbound event sink to the microservice
- [ ] Step 8: Extend the mock
- [ ] Step 9: Test the inbound event sink
- [ ] Step 10: Update manifest
- [ ] Step 11: Document the microservice
- [ ] Step 12: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Locate the Source of the Outbound Event and Determine Signature

Locate the `Hook` in the API directory of the microservice that is the source the event. Determine the signature of the outbound event.

```go
func OnMyEvent(ctx context.Context, inArg1 string, inArg2 ThirdPartyStruct) (outArg1 map[string]MyStruct, err error)
```

#### Step 3: Determine a Description

Pull the event description from the `Hook`: `OnMyEvent is triggered when X`. Embed this description in followup steps where appropriate.

#### Step 4: Determine the Required Claims

Determine if the endpoint should be restricted to authorized actors only. Compose a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied. For example: `roles=~"manager" && level>2`. Leave empty if the endpoint should be accessible by all.

#### Step 5: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	OnMyEvent(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error) { // MARKER: OnMyEvent
}
```

#### Step 6: Implement the Inbound Event Sink Logic

Implement the inbound event sink in `service.go`. Complex types should always refer to their definition in `eventsourceapi`, even if owned by a third-party.

```go
/*
OnMyEvent is triggered when X.
*/
func (svc *Service) OnMyEvent(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error) { // MARKER: OnMyEvent
	// Implement logic here...
	return nil, nil
}
```

#### Step 7: Bind the Inbound Event Sink to the Microservice

Bind the inbound event sink to the microservice in the `NewIntermediate` constructor in `intermediate.go`.

- The queue option indicate how requests are distributed among replicas of the microservice
  - `sub.DefaultQueue()`: requests are load balanced among peers and processed by only one. This is the default option and may be omitted
  - `sub.NoQueue()`: requests are processed by all subscribers
  - `sub.Queue(queueName)`: requests are load balanced among peers associated with this queue name. Subscribers associated with other queue names receive the requests separately based on their own queue option
- The `sub.RequiredClaims(requiredClaims)` option defines the authorization requirements of the endpoint. This option can be omitted to allow all requests
- The return values of `eventsourceapi.NewHook` are discarded by intent

```go
func NewIntermediate() *Intermediate {
	// ...
	eventsourceapi.NewHook(svc).OnMyEvent(svc.OnMyEvent, sub.LoadBalanced(), sub.RequiredClaims(requiredClaims)) // MARKER: OnMyEvent
	// ...
}
```

Add the appropriate import to `github.com/company/project/eventsource/eventsourceapi`.

#### Step 8: Extend the Mock

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockOnMyEvent func(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error) // MARKER: OnMyEvent
}
```

Add the stubs to the `Mock`.

```go
// MockOnMyEvent sets up a mock handler for OnMyEvent.
func (svc *Mock) MockOnMyEvent(handler func(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error)) *Mock { // MARKER: OnMyEvent
	svc.mockOnMyEvent = handler
	return svc
}

// OnMyEvent executes the mock handler.
func (svc *Mock) OnMyEvent(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error) { // MARKER: OnMyEvent
	if svc.mockOnMyEvent == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	argOut1, err = svc.mockOnMyEvent(ctx, argIn1, argIn2)
	return argOut1, errors.Trace(err)
}
```

Add a test case in `TestMyService_Mock`.

- Set values for the example input arguments
- Set values for the expected output arguments

```go
t.Run("on_my_event", func(t *testing.T) { // MARKER: OnMyEvent
	assert := testarossa.For(t)

	exampleArgIn1 := ""
	exampleArgIn2 := eventsourceapi.ThirdPartyStruct{}
	expectedArgOut1 := map[string]eventsourceapi.MyStruct{}

	_, err := mock.OnMyEvent(ctx, exampleArgIn1, exampleArgIn2)
	assert.Contains(err.Error(), "not implemented")
	mock.MockOnMyEvent(func(ctx context.Context, argIn1 string, argIn2 eventsourceapi.ThirdPartyStruct) (argOut1 map[string]eventsourceapi.MyStruct, err error) {
		return expectedArgOut1, nil
	})
	argOut1, err := mock.OnMyEvent(ctx, exampleArgIn1, exampleArgIn2)
	assert.Expect(
		argOut1, expectedArgOut1,
		err, nil,
	)
})
```

#### Step 9: Test the Inbound Event Sink

Skip this step if instructed to be "quick" or to skip tests.

Append the following code block to the end of `service_test.go`.

- Do not remove comments with `HINT`s. They are there to guide you in the future.
- Insert test cases at the bottom of the test function using the recommended pattern.
- There is no need to set the `pub.Actor` option if the inbound event sink does not require claims.

```go
func TestMyService_OnMyEvent(t *testing.T) { // MARKER: OnMyEvent
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	eventsourceTrigger := eventsourceapi.NewMulticastTrigger(tester)

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
			for e := range eventsourceTrigger.WithOptions(pub.Actor(actor)).OnMyEvent(ctx, argIn1, argIn2) {
				argOut1, err := e.Get()
				if frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {
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

Update the `inboundEvents` and `downstream` sections of `manifest.yaml`.

#### Step 11: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 12: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.

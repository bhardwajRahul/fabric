---
name: Adding a Ticker
description: Creates or modify a ticker of a microservice. Use when explicitly asked by the user to create or modify a ticker or a recurring operation for a microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a ticker:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Define and implement handler
- [ ] Step 3: Extend the ToDo interface
- [ ] Step 4: Bind handler to the microservice
- [ ] Step 5: Extend the mock
- [ ] Step 6: Test the handler
- [ ] Step 7: Update manifest
- [ ] Step 8: Document the microservice
- [ ] Step 9: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Define and Implement Handler

Implement the ticker handler function in `service.go`. Append it at the end of the file.

```go
/*
MyTicker does X.
*/
func (svc *Service) MyTicker(ctx context.Context) (err error) { // MARKER: MyTicker
	// Implement logic here...
	return nil
}
```

#### Step 3: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyTicker(ctx context.Context) (err error) // MARKER: MyTicker
}
```

#### Step 4: Bind the Handler to the Microservice

Bind the ticker handler to the microservice in the `NewIntermediate` constructor in `intermediate/intermediate.go`.

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.StartTicker("MyTicker", time.Minute, svc.MyTicker) // MARKER: MyTicker
	// ...
}
```

Customize the duration to indicate how often to invoke the ticker.

#### Step 5: Extend the Mock

The `Mock` must satisfy the `ToDo` interface.

Add a field to the `Mock` structure definition in `intermediate/mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockMyTicker func(ctx context.Context) (err error) // MARKER: MyTicker
}
```

Add the stubs to the `Mock`:

```go
// MockMyTicker sets up a mock handler for MyTicker.
func (svc *Mock) MockMyTicker(handler func(ctx context.Context) (err error)) *Mock { // MARKER: MyTicker
	svc.mockMyTicker = handler
	return svc
}

// MyTicker executes the mock handler.
func (svc *Mock) MyTicker(ctx context.Context) (err error) { // MARKER: MyTicker
	if svc.mockMyTicker == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockMyTicker(ctx)
	return errors.Trace(err)
}
```

Add a test case in `TestMyService_Mock`.

```go
t.Run("my_ticker", func(t *testing.T) { // MARKER: MyTicker
	assert := testarossa.For(t)

	err := mock.MyTicker(ctx)
	assert.Contains(err.Error(), "not implemented")
	mock.MockMyTicker(func(ctx context.Context) (err error) {
		return nil
	})
	err = mock.MyTicker(ctx)
	assert.NoError(err)
})
```

#### Step 6: Test the Handler

Skip this step if instructed to be "quick" or to skip tests.

Add a test to `service_test.go`.

```go
func TestMyService_MyTicker(t *testing.T) { // MARKER: MyTicker
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			err := svc.MyTicker(ctx)
			assert.NoError(err)
		})
	*/
}
```

The `MyService` part of the test name should match the microservice name.

The `MyTicker` part of the test name should match the ticker name.

#### Step 7: Update Manifest

Update the `tickers` and `downstream` sections of `manifest.yaml`.

#### Step 8: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 9: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate/intermediate.go`.

---
name: Adding a Ticker
description: Creates or modify a ticker of a microservice. Use when explicitly asked by the user to create or modify a ticker or a recurring operation for a microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

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
- [ ] Step 7: Housekeeping
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

Bind the ticker handler to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...
	svc.StartTicker("MyTicker", time.Minute, svc.MyTicker) // MARKER: MyTicker
	// ...
}
```

Customize the duration to indicate how often to invoke the ticker.

#### Step 5: Extend the Mock

The `Mock` must satisfy the `ToDo` interface.

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

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

Append the integration test to `service_test.go`.

```go
func TestMyService_MyTicker(t *testing.T) { // MARKER: MyTicker
	t.Parallel()
	ctx := t.Context()
	_ = ctx

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

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	err := svc.MyTicker(ctx)
	assert.NoError(err)
})
```

Do not remove the `HINT` comments.

#### Step 7: Housekeeping

Follow the `microbus/housekeeping` skill.

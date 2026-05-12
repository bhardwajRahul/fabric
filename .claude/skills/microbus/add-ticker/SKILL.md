---
name: add-ticker
description: TRIGGER when user asks to add a recurring job, periodic task, scheduled operation, or ticker. Affects intermediate.go, mock.go, manifest.yaml. Do NOT manually wire up tickers.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a ticker:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Define and implement handler
- [ ] Step 3: Extend the ToDo interface
- [ ] Step 4: Bind handler to the microservice
- [ ] Step 5: Regenerate the mock
- [ ] Step 6: Test the handler
- [ ] Step 7: Housekeeping
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

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

Bind the ticker handler to the microservice in the `NewIntermediate` constructor in `intermediate.go`, after the corresponding `HINT` comment. If other tickers already exist under this HINT, add the new one after the last existing ticker.

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...
	svc.StartTicker("MyTicker", time.Minute, svc.MyTicker) // MARKER: MyTicker
	// ...
}
```

Customize the duration to indicate how often to invoke the ticker.

#### Step 5: Regenerate the Mock

Run `go run github.com/microbus-io/fabric/cmd/genmock --path .` from the microservice's directory. This regenerates both `mock.go` and `mock_test.go`.

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
- Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	err := svc.MyTicker(ctx)
	assert.NoError(err)
})
```

#### Step 7: Housekeeping

Follow the `housekeeping` skill.

---
name: add-ticker
description: TRIGGER when user asks to add a recurring job, periodic task, scheduled operation, or ticker.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: A ticker is declared as a `define.Ticker` var in `<name>api/definition.go` and implemented as a handler in `service.go`. Add the declaration and run `cmd/genservice`.

**CRITICAL**: Keep the `// MARKER: Name` comment on the `define.Ticker` var.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a ticker:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Declare the ticker in definition.go
- [ ] Step 3: Implement the handler in service.go
- [ ] Step 4: Generate the boilerplate
- [ ] Step 5: Test the handler
- [ ] Step 6: Housekeeping
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Declare the Ticker in `definition.go`

Append the `define.Ticker` var to `myserviceapi/definition.go`. `Interval` is how often the ticker runs; add the `"time"` import for the duration.

```go
// MyTicker does X.
var MyTicker = define.Ticker{ // MARKER: MyTicker
	Interval: time.Minute,
}
```

#### Step 3: Implement the Handler in `service.go`

Implement the ticker handler in `service.go`.

```go
// MyTicker does X.
func (svc *Service) MyTicker(ctx context.Context) (err error) { // MARKER: MyTicker
	// Implement logic here...
	return nil
}
```

#### Step 4: Generate the Boilerplate

From the microservice's directory, run the generator. It regenerates `intermediate.go` (the `ToDo` entry and `StartTicker` wiring), `mock.go`, `mock_test.go`, and `manifest.yaml` from the updated `definition.go`.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then verify the microservice compiles with `go vet ./...` from the project root.

#### Step 5: Test the Handler

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

Insert test cases at the bottom of the integration test function using the recommended pattern. Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	err := svc.MyTicker(ctx)
	assert.NoError(err)
})
```

#### Step 6: Housekeeping

Follow the `housekeeping` skill.

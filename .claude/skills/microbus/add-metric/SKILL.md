---
name: Adding a Metric
description: Creates or modify a metric of a microservice. Use when explicitly asked by the user to create or modify a custom metric for a microservice, or when it makes sense to measure a certain operation taken by the microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a metric:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine kind
- [ ] Step 3: Determine if observable just in time
- [ ] Step 4: Determine signature
- [ ] Step 5: Determine OpenTelemetry name
- [ ] Step 5: Extend the ToDo interface
- [ ] Step 6: Describe the metric
- [ ] Step 7: Implement the recorders
- [ ] Step 8: Observe with callback
- [ ] Step 9: Extend the mock
- [ ] Step 10: Test the callback
- [ ] Step 11: Update manifest
- [ ] Step 12: Document the microservice
- [ ] Step 13: Versioning
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Kind

Determine if the metric is a:
- **Counter** - an ever increasing number
- **Gauge** - a number that can go up or down
- **Histogram** - a distribution of cumulative values over time

If a histogram, determine the boundaries that determine its buckets.

#### Step 3: Determine If Observable Just in Time

Some metrics, and in particular gauges, it is enough to observe just in time before recording. For example, there's no reason to sample the available system memory constantly if only the last sampling will be reported. Determine if this metric is observable.

#### Step 4: Determine Signature

Determine the Go signature of the metric.

```go
func MyMetric(ctx context.Context, value int, label1 string, label2 string) (err error)
```

Constraints:
- The first input argument must be `ctx context.Context`
- The function must return an `err error`
- The second input argument is the numerical value of the metric and must be of type `int`, `float64` or `time.Duration`
- The remaining input arguments are labels and should be a `string` or any type that can be converted to a string

#### Step 5: Determine OpenTelemetry Name

The OpenTelemetry name of the metric:
- Should have a (single-word) application prefix relevant to the domain the metric belongs to. The prefix is usually the application name itself
- Should have a suffix describing the unit, in plural form

For example, `myapplication_my_metric_units`.

#### Step 6: Extend the `ToDo` Interface

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	MyMetric(ctx context.Context, value int, label1 string, label2 string) (err error) { // MARKER: MyMetric
}
```

#### Step 7: Describe the Metric

Describe the metric in `NewIntermediate` in `intermediate.go`.

- Use snake_case for the metric alias
- If the metric has a unit such as `seconds` or `mb`, append it to the alias
- Replace `MyMetric counts X` with the description of the metric

If a histogram:

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.DescribeHistogram("myapplication_my_metric_units", "MyMetric counts X", []float64{1, 2, 5, 10, 100}) // MARKER: MyMetric
}
```

If a gauge:

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.DescribeGauge("myapplication_my_metric_units", "MyMetric counts X") // MARKER: MyMetric
}
```

If a counter:

```go
func NewIntermediate() *Intermediate {
	// ...
	svc.DescribeCounter("myapplication_my_metric_units", "MyMetric counts X") // MARKER: MyMetric
}
```

#### Step 8: Implement the Recorders

Create the recording methods in `intermediate.go`.

- Cast `int` values to `float64`
- Convert `time.Duration` values to seconds using `dur.Seconds()`
- Set an appropriate comment that describes the metric
- Use snake_case for the metric alias and label names
- If the metric has a unit such as `seconds` or `mb`, append it to the alias

If the metric is a histogram, add the following code.

```go
/*
RecordMyMetric records X.
*/
func (svc *Intermediate) RecordMyMetric(ctx context.Context, value int, label1 string, label2 string) (err error) { // MARKER: MyMetric
	return svc.RecordHistogram(ctx, "my_metric_units", float64(value),
		"label_1", utils.AnyToString(label1),
		"label_2", utils.AnyToString(label2),
	)
}
```

If the metric is a counter, add the following code.

```go
/*
IncrementMyMetric counts X.
*/
func (svc *Intermediate) IncrementMyMetric(ctx context.Context, value int, label1 string, label2 string) (err error) { // MARKER: MyMetric
	return svc.IncrementCounter(ctx, "my_metric_units", float64(value),
		"label_1", utils.AnyToString(label1),
		"label_2", utils.AnyToString(label2),
	)
}
```

If the metric is a gauge, add the following code.

```go
/*
RecordMyMetric records X.
*/
func (svc *Intermediate) RecordMyMetric(ctx context.Context, value int, label1 string, label2 string) (err error) { // MARKER: MyMetric
	return svc.RecordGauge(ctx, "my_metric_units", float64(value),
		"label_1", utils.AnyToString(label1),
		"label_2", utils.AnyToString(label2),
	)
}
```

Note that `utils.AnyToString` is defined in `github.com/microbus-io/fabric/utils`.

#### Step 9: Observe With Callback

Skip this step if the metric is not observable just in time.

Define a callback in `service.go` that calculates the metric and records it using `RecordMyMetric` (if histogram or gauge) or `IncrementMyMetric` (if counter).

```go
func (svc *Service) OnObserveMyMetric(ctx context.Context) (err error) { // MARKER: MyMetric
	v, err := calculateMyMetric()
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.RecordMyMetric(ctx, v, "label1", "label2")
	return errors.Trace(err)
}
```

Call the `OnObserveMyMetric` callback in `doOnObserveMetrics` in `intermediate.go`.

```go
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
		// ...
		func() (err error) { return svc.OnObserveMyMetric(ctx) }, // MARKER: MyMetric
	)
}
```

#### Step 10: Extend the Mock

Skip this step if the metric is not observable just in time.

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockOnObserveMyMetric func(ctx context.Context) (err error) // MARKER: MyMetric
}
```

Add the stub to the `Mock`.

```go
// MockOnObserveMyMetric sets up a mock handler for OnObserveMyMetric.
func (svc *Mock) MockOnObserveMyMetric(handler func(ctx context.Context) (err error)) *Mock { // MARKER: MyMetric
	svc.mockOnObserveMyMetric = handler
	return svc
}

// OnObserveMyMetric executes the mock handler.
func (svc *Mock) OnObserveMyMetric(ctx context.Context) (err error) { // MARKER: MyMetric
	if svc.mockOnObserveMyMetric == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnObserveMyMetric(ctx)
	return errors.Trace(err)
}
```

#### Step 11: Test the Callback

Skip this step if the metric is not observable just in time.

Append the following code block to the end of `service_test.go`.

- Do not remove comments with `HINT`s. They are there to guide you in the future.
- Insert test cases at the bottom of the test function using the recommended pattern.

```go
func TestMyService_OnObserveMyMetric(t *testing.T) { // MARKER: MyMetric
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

			err := svc.OnObserveMyMetric(ctx)
			assert.NoError(err)
		})
	*/
}
```

#### Step 12: Update Manifest

Update the `metrics` and `downstream` sections of `manifest.yaml`.

#### Step 13: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, context, and design rationale. Focus on the reasons behind decisions rather than describing what the code does. Explain design choices, tradeoffs, and the context needed for someone to safely evolve this microservice in the future.

#### Step 14: Versioning

If this is the first edit to the microservice in this session, increment the `Version` const in `intermediate.go`.

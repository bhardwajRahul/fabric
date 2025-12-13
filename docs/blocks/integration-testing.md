# Integration Testing

Thorough testing is an important cornerstone of good software. Testing a microservice is generally difficult because it almost always depends on downstream microservices which are not easy to spin up during testing. Common workarounds include mocking the downstream microservices or testing against a live test deployment but each of these comes with its own drawbacks. A mock doesn't test the actual internal business logic of the microservice, obscuring changes made to it over time. A live test deployment doesn't suffer from the drawbacks of a mock but it is a single point of failure that can block an entire development team when it's down. It also tends to become inconsistent over time and is expensive to run 24/7.

## Testing App

`Microbus` takes a different approach and spins up the actual downstream microservices along with the microservice being tested into a single process. The microservices are collected into an isolated [`Application`](../structure/application.md) that is started up for the duration of running the test and shutdown immediately thereafter. The microservices communicate on a random [plane of communications](../blocks/unicast.md), which keeps them isolated from other tests that may run in parallel.

Mocks can be added to the application when it's impractical to run the actual downstream microservice, for example if that microservice is calling a third-party web service such as a payment processor. The preference however should be to include the actual microservice whenever possible and not rely on mocks. Note that in `Microbus` it is the microservices that are mocked rather than the clients. The upstream microservice still sends messages to the downstream, they are just responded to by the mock.

<img src="./integration-testing-1.drawio.svg">

## Code Generated Test Harness

This is all rather complicated to set up which is where the [code generator](../blocks/codegen.md) comes into the picture and automatically creates a test harness for each of the microservice's endpoints based on the specification of the microservice (`service.yaml`). It is left for the developer to initialize the testing app and implement the logic of each test.

### Initializing the Testing App

For each test, the code generator prepares a testing app `app` and includes in it the microservice under test `svc`. Dependencies on downstream microservices should be added to the app manually, using the `NewService` constructor of that service. During testing, the [configurator](../structure/coreservices-configurator.md) core microservice is disabled and microservices must be configured directly. If the microservice under test defines any configuration properties, they are pre-listed commented-out for convenience.

```go
func TestMyService_MyEndpoint(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetMyConfig(myConfig)

	// Initialize the testers
	tester := connector.New("myservice.myendpoint.tester")
	client := myservice.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
		downstream.NewService().Init(func(svc *downstream.Service) {
			downstream.SetKey("abcde")
		}),
	)
	app.RunInTest(t)

	// ...
}
```

The `Init` method is a convenient one-statement pattern for initialization of a microservice.

### Testing Functions

A test is generated in `service_test.go` for each corresponding endpoint of the microservice. The recommended test pattern differs based on the type of the endpoint. In the following example, `Arithmetic` is a functional endpoint.

```go
func TestCalculator_Arithmetic(t *testing.T) {
	// ...

	t.Run("addition", func(t *testing.T) {
		assert := testarossa.For(t)
		result, err := client.Arithmetic(ctx, 5, "+", 6)
		assert.Expect(result, 11, err, nil)
	})

	// ...
}
```

Notice the use of the `client` to call the endpoint of the microservice. Although it is possible to call `svc.Arithmetic` directly, using the client mimics more accurately the call path in a production setting.

It is recommended, but not required, to use the `testarossa` asserter.

### Testing Webs

Raw web endpoints are tested in a similar fashion, except that assertion is customized for a web request. In the following example, the `Hello` endpoint is method-agnostic and can be tested with various HTTP methods.

```go
func TestHello_Hello(t *testing.T) {
	// ...

	t.Run("hello", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Hello(ctx, "GET", "?name=Maria", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Hello, Maria")
			}
		}
	})

	// ...
}
```

URLs are resolved relative to the URL of the endpoint. The empty URL `""` resolves to the exact URL of the endpoint. A URL that starts with a `?` is the way to pass query arguments.

### Testing Tickers

[Tickers](../blocks/tickers.md) are disabled during testing in order to avoid the unpredictability of their running schedule. Instead, tickers have their own dedicated tests.

```go
func TestHello_TickTock(t *testing.T) {
	// ...

	t.Run("ticktock_runs", func(t *testing.T) {
		assert := testarossa.For(t)
		err := svc.TickTock(ctx)
		assert.NoError(err)
	})

	// ...
}
```

### Testing Config Callbacks

Callbacks that handle changes to config property values are tested by calling the corresponding setter. The callback will be triggered behind the scenes.

```go
func TestExample_OnChangedPort(t *testing.T) {
	// ...

	t.Run("switch_database", func(t *testing.T) {
		assert := testarossa.For(t)
		err := svc.SetPort(1234)
		assert.NoError(err)
	})

	// ...
}
```

### Testing Event Sources

Events require the setting of an event sink and the firing of the event via the appropriate trigger.
In the following example, `hook.OnRegistered` attaches an event sink to the `tester`, and `trigger.OnRegistered` fires the event.
An event can be processed by multiple recipients, hence the `range` loop and the check that the response was received from the `tester`.

```go
func TestExample_OnRegistered(t *testing.T) {
	// ...

	t.Run("registration_notification", func(t *testing.T) {
		assert := testarossa.For(t)
		hook.OnRegistered(func(ctx context.Context, email string) (err error) {
			assert.Expect(email, "peter@example.com")
			return nil
		})
		defer hook.OnRegistered(nil)
		for e := range trigger.OnRegistered(ctx, "peter@example.com") {
			if frame.Of(e.HTTPResponse).FromHost() == tester.Hostname() {
				err := e.Get()
				assert.NoError(err)
			}
		}
	})

	// ...
}
```

### Testing Event Sinks

Event sinks use the trigger of the event source to fire the event, and the service under test as its sink.
In the following example, `eventsourceTrigger.OnAllowRegister` fires the event to the event sink implemented by `svc.OnAllowRegister`.
An event can be processed by multiple recipients, hence the `range` loop and the check that the response was received from `svc`.

```go
func TestExample_OnAllowRegister(t *testing.T) {
	// ...

	t.Run("allowed_registrations", func(t *testing.T) {
		assert := testarossa.For(t)
		for e := range eventsourceTrigger.OnAllowRegister(ctx, "user@example.com") {
			if frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {
				allow, err := i.Get()
				assert.Expect(allow, true, err, nil)
			}
		}
	})

	// ...
}
```

### Testing Metric Callbacks

Callbacks that handle observation of a metric are tested by calling the callback directly via `svc`.

```go
func TestExample_OnObserveNumOperations(t *testing.T) {
	// ...

	t.Run("observe_num_operations", func(t *testing.T) {
		assert := testarossa.For(t)
		err := svc.OnObserveNumOperations(ctx)
		assert.NoError(err)
	})

	// ...
}
```

## Skipping Tests

A removed test will be regenerated on the next run of the code generator, so disabling a test is best achieved by placing a call to `t.Skip()` along with an explanation of why the test was skipped.

```go
func TestEventsink_OnRegistered(t *testing.T) {
	t.Skip() // Tested elsewhere
}
```

## Parallelism

The code generator specifies to run all tests in parallel. The assumption is that tests are implemented as to not interfere with one another. Comment out `t.Parallel()` to run that test separately from other tests. Be advised that the order of execution of tests is not guaranteed and care must be taken to reset the state at the end of a test that may interfere with another.

## Mocking

Sometimes, using the actual microservice is not possible because it depends on a resource that is not available in the testing environment. For example, a microservice that makes requests to a third-party web service should be mocked in order to avoid depending on that service for development.

In order to more easily mock microservices, the code generator creates a `Mock` for every microservice. This mock includes type-safe methods for mocking all the endpoints of the microservice. Mocks are added to testing applications in lieu of the real services.

```go
func TestPayment_ChargeUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("payment.chargeuser.tester")
	client := payment.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
		webpay.NewMock().
			MockCharge(func(ctx context.Context, userID string, amount int) (success bool, balance int, err error) {
				return true, 100, nil
			}),
	)
	app.RunInTest(t)

	// ...
}
```

## Shifting the Clock

At times it is desirable to test aspects of the application that have a temporal dimension. For example, an algorithm may place more weight on newer rather than older content, or perhaps generate a daily histogram of time-series data over a period of a year. In such cases, one would want to perform operations as if they occurred at different times, not only now.

`Microbus` enables this scenario by attaching a clock shift (offset) to the context using the `SetClockShift` method of the [frame](../structure/frame.md). The connector's `Now(ctx)` method then takes the clock shift into account before returning the "current" time.

To shift the clock in the test:

```go
func TestFoo_DoSomething(t *testing.T) {
	// ...

	t.Run("yesterday", func(t *testing.T) {
		assert := testarossa.For(t)
		ctx := frame.CloneContext(ctx)

		frame.Of(ctx).SetClockShift(-time.Hour * 24) // 24 hours ago
		err := client.DoSomething(ctx, userKey)
		assert.NoError(err)

		frame.Of(ctx).IncrementClockShift(time.Hour) // 23 hours ago
		err = client.DoSomething(ctx, userKey)
		assert.NoError(err)
	})

	// ...
}
```

Note that you need to clone the context before modifying its frame.

To obtain the "current" time in a microservice, use `svc.Now(ctx)` instead of `time.Now()`.

```go
func (svc *Service) DoSomething(ctx context.Context, n int) (err error) {
	now := svc.Now(ctx) // Now is offset by the clock shift
	// ...
	err = barapi.NewClient(svc).DoSomethingElse(ctx, n) // Clock shift is propagated downstream
	// ...
}
```

The clock shift is propagated down the call chain to downstream microservices. The success of this pattern depends on each of the microservices involved in the transaction using `svc.Now(ctx)` instead of the standard `time.Now()` to obtain the current time.

Shifting the clock outside the `TESTING` deployment should be done with extreme caution. Unlike the `TESTING` deployment, tickers are enabled in the `LOCAL`, `LAB` and `PROD` [deployments](../tech/deployments.md) and are always executed at the real time.

Note that shifting the clock will not cause any timeouts or deadlines to be triggered. It is simply a mechanism of transferring an offset down the call chain.

## Manipulating the Context

`Microbus` uses the `ctx` or `r.Context()` to pass-in adjunct data that does not affect the business logic of the endpoint. The context is extended with a [frame](../structure/frame.md) which internally holds an `http.Header` that includes various `Microbus` key-value pairs. Shifting the clock is one common example, another is the language.

Use the `frame.Frame` to access and manipulate this header:

```go
frm := frame.Of(ctx) // or frame.Of(r)
frm.SetClockShift(-time.Hour)
frm.SetLanguages("it", "fr")
```

## Maximizing Results

Some tips for maximizing the effectiveness of your testing:

### Code Coverage

A test is generated for each one of the microservice's endpoints. Use them to define numerous test cases and cover all aspects of the endpoint, including its edge cases. This is a quick way to achieve high code coverage. 

### Downstream Microservices

Take advantage of `Microbus`'s unique ability to run integration tests inside Go's unit test framework.
Include in the testing app all the downstream microservices that the microservice under test is dependent upon. Create tests for any of the assumptions that the microservice under test is making about the behavior of the downstream microservices.

### Scenarios

Don't be satisfied with the code-generated tests. High code coverage is not enough. Write tests that perform complex scenarios based on the business logic of the solution. For example, if the microservice under test is a CRUD microservice, perform a test that goes through a sequence of steps such as `Create`, `Load`, `List`, `Update`, `Load`, `List`, `Delete`, `Load`, `List` and check for integrity after each step. Involve as many of the downstream microservices as possible, if applicable.

# Features of a Microservice

A `Microbus` microservice is composed of several types of features that together define its API, behavior and observability. Each feature is implemented in `service.go` and cataloged in the microservice's `manifest.yaml`.

<img src="./features-1.drawio.svg">
<p><p>

## Functional Endpoints

Functional endpoints are typed request/response functions that make a microservice's capabilities available to other microservices. They are the primary means of service-to-service communication in `Microbus`.

A functional endpoint has a Go-style signature with input and output arguments. `Microbus` takes care of marshaling and unmarshaling arguments behind the scenes. Input arguments are extracted from the HTTP request path, query string, or JSON body. Output arguments are marshaled as JSON in the response body.

```go
func (svc *Service) Add(ctx context.Context, x int, y int) (sum int, err error) {
	return x + y, nil
}
```

An endpoint can be restricted to a specific HTTP method (`GET`, `POST`, etc.) or left as `ANY` to accept all methods. The route defaults to the function name in kebab-case. For example, a function named `CalculatePayment` is typically reachable at `/calculate-payment`. Routes may include [path arguments](../tech/path-arguments.md) for RESTful-style URLs:

```yaml
# manifest.yaml representation
functions:
  LoadArticle:
    signature: LoadArticle(id int) (article *Article)
    description: LoadArticle returns an article by its ID.
    method: GET
    route: /article/{id}
```

Upstream microservices call functional endpoints using generated [client stubs](../blocks/client-stubs.md):

```go
article, err := articleapi.NewClient(svc).LoadArticle(ctx, 42)
```

## Web Handlers

Web handlers give the microservice full control over the HTTP request and response. They are used to serve HTML pages, static files, or any response that requires direct access to the `http.ResponseWriter`. Like functional endpoints, web handlers can be restricted to a specific HTTP method or left as `ANY`.

```yaml
# manifest.yaml representation
webs:
  Dashboard:
    signature: Dashboard()
    description: Dashboard renders the main dashboard page.
    method: GET
    route: //dashboard
```

```go
func (svc *Service) Dashboard(w http.ResponseWriter, r *http.Request) (err error) {
	data := struct {
		Title string
	}{
		Title: "My Dashboard",
	}
	err = svc.WriteResTemplate(w, "dashboard.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

Web handlers can use [path arguments](../tech/path-arguments.md) to extract values from the URL:

```go
// Route: /avatar/{uid}/{size}
func (svc *Service) Avatar(w http.ResponseWriter, r *http.Request) (err error) {
	uid := r.PathValue("uid")
	size := r.PathValue("size")
	// ...
	return nil
}
```

## Outbound Events

Outbound events are messages that a microservice fires to notify others of something that happened, without knowing in advance who (if anyone) is listening. Events decouple microservices and allow new consumers to be added without changing the producer.

Events are cataloged in the manifest:

```yaml
# manifest.yaml representation
outboundEvents:
  OnOrderPlaced:
    signature: OnOrderPlaced(orderID string, total float64)
    description: OnOrderPlaced is triggered when a new order is placed.
    method: POST
    route: :417/on-order-placed
```

Events use port `:417` by default to differentiate them from standard endpoints on port `:443`. The microservice fires an event using the generated multicast trigger:

```go
for range orderapi.NewMulticastTrigger(svc).OnOrderPlaced(ctx, orderID, total) {
}
```

## Inbound Event Sinks

Event sinks are the flip side of outbound events. A sink subscribes to consume events emitted by another microservice. The sink's signature must match the source event.

```yaml
# manifest.yaml representation
inboundEvents:
  OnOrderPlaced:
    signature: OnOrderPlaced(orderID string, total float64)
    description: OnOrderPlaced handles new orders from the order service.
    source: github.com/example/orderservice
```

The generated handler receives the event's arguments:

```go
func (svc *Service) OnOrderPlaced(ctx context.Context, orderID string, total float64) (err error) {
	svc.LogInfo(ctx, "New order received", "order", orderID, "total", total)
	// React to the event...
	return nil
}
```

If two different event sources use the same event name, the sink can rename the handler to avoid conflicts:

```yaml
# manifest.yaml representation
inboundEvents:
  OnDeleteUser:
    signature: OnDeleteUser(userID string)
    event: OnDelete
    source: github.com/example/userstore
  OnDeleteGroup:
    signature: OnDeleteGroup(groupID string)
    event: OnDelete
    source: github.com/example/groupstore
```

## Configuration Properties

[Configuration](../blocks/configuration.md) properties are runtime settings that a microservice declares for itself. Their values are managed externally in `config.yaml` (or `config.local.yaml` for secrets) and delivered to the microservice by the [configurator](../structure/coreservices-configurator.md) core microservice.

```yaml
configs:
  APIKey:
    description: APIKey is the key for the external payments API.
    type: string
    validation: str ^[A-Za-z0-9]+$
  MaxRetries:
    description: MaxRetries is the maximum number of retry attempts.
    type: int
    validation: int [1,10]
    default: 3
```

Supported types are `string`, `bool`, `int`, `float64` and `time.Duration`. Validation rules are enforced before a new value is accepted. Configuration values are accessed through generated accessor methods:

```go
func (svc *Service) CallPaymentAPI(ctx context.Context) error {
	apiKey := svc.APIKey()
	maxRetries := svc.MaxRetries()
	// ...
}
```

Values are set in `config.yaml` using the microservice's hostname:

```yaml
payments.example:
  APIKey: sk_live_abc123
  MaxRetries: 5
```

An optional callback can be generated to react to configuration changes at runtime:

```go
// OnChangedMaxRetries is triggered when the value of the MaxRetries config property changes.
func (svc *Service) OnChangedMaxRetries(ctx context.Context) (err error) {
	newVal := svc.MaxRetries()
	// React to the change...
	return nil
}
```

## Tickers

Tickers trigger a recurring operation on a periodic basis. They are useful for polling, cleanup jobs, cache refreshes and other scheduled tasks.

```yaml
# manifest.yaml representation
tickers:
  RefreshRates:
    description: RefreshRates fetches the latest exchange rates.
    interval: 1h
```

The generated handler is called at the specified interval:

```go
func (svc *Service) RefreshRates(ctx context.Context) (err error) {
	// Fetch latest rates from external API...
	return nil
}
```

The `ctx` passed to the ticker is canceled when the microservice shuts down, allowing long-running iterations to terminate gracefully.

## Metrics

Metrics provide observability into the microservice's behavior. `Microbus` supports three metric types aligned with [OpenTelemetry](https://opentelemetry.io/docs/specs/otel/metrics/data-model/):

- **Counter** - a monotonically increasing value, useful for counting occurrences (e.g. requests handled, errors encountered)
- **Gauge** - a value that can go up or down, useful for measuring current state (e.g. active connections, queue depth)
- **Histogram** - a distribution of values across configurable buckets, useful for measuring things like response times or payload sizes

```yaml
# manifest.yaml representation
metrics:
  RequestsHandled:
    description: RequestsHandled counts the number of API requests.
    kind: counter
    otelName: requests_handled
  ActiveConnections:
    description: ActiveConnections tracks the number of open connections.
    kind: gauge
    otelName: active_connections
  ResponseTime:
    description: ResponseTime measures endpoint latency in milliseconds.
    kind: histogram
    otelName: response_time_ms
    buckets: [5, 10, 25, 50, 100, 250, 500, 1000]
```

A metric can optionally be marked as observable, in which case a callback is invoked just before the value is pushed to the OpenTelemetry collector. This is useful for metrics that are expensive to compute and should only be measured on demand.

Metrics are recorded using generated helper methods:

```go
func (svc *Service) HandleRequest(ctx context.Context) error {
	t0 := svc.Now(ctx)
	svc.IncrementRequestsHandled(ctx, 1)
	// ... handle the request ...
	elapsed := svc.Now(ctx).Sub(t0).Milliseconds()
	svc.RecordResponseTime(ctx, float64(elapsed))
	return nil
}
```

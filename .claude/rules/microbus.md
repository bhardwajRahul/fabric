# Developing With the Microbus Framework

## Instructions for Agents

**CRITICAL**: This project follows a microservices architecture based on the Microbus framework. Always follow the patterns and conventions in this file.

**CRITICAL**: Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

**CRITICAL**: After compaction of the context window, re-read this file.

**CRITICAL**: Comments in the code that include the word `HINT` or `MARKER` in all-caps are there to guide you. Do not remove them.

## Overview

Microbus is a holistic open source framework for developing, testing, deploying and operating microservices at scale. It combines best-in-class OSS, tooling and best practices into a dramatically simplified engineering experience.

## Key Concepts

### Microservices and Features

A Microbus application comprises a multitude of microservices that communicate over a messaging bus using the HTTP protocol.

A microservice consists of the following features:
- **Web handler endpoints** - raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Functional endpoints (RPCs)** - typed request/response functions with input/output structs, marshaling, and client stubs
- **Outbound events** - messages this microservice fires for others to consume
- **Inbound event sinks** - handlers that react to events emitted by other microservices
- **Configuration properties** - runtime settings (strings, durations, booleans, etc.)
- **Task endpoints** - typed handlers for agentic workflow steps, receiving and returning state via a `workflow.Flow` carrier
- **Workflow graphs** - definitions of agentic workflow structure, describing task transitions and conditions
- **Tickers** - recurring operations on a schedule
- **Metrics** - counters, gauges, and histograms for observability

### Subscription Route

The subscription route of an endpoint is resolved relative to the hostname of the microservice to form an internal Microbus URL. External clients reach the endpoint through the HTTP ingress proxy, which prepends the microservice hostname to the path. For example, route `/my/path` on `myservice.hostname` is internally `https://myservice.hostname/my/path` and externally `https://webserver.hostname/myservice.hostname/my/path`.

It's recommended to set the route of a subscription to match the name of its handler, in kebab-case, e.g. `/my-handler`.

An empty route `""` or `/` maps to the root path of the microservice.

Set a port by prepending it to the route, e.g. `:123/my-path`. A port alone with no path is also valid, e.g. `:123`. The default port is `:443`.

Encase path arguments in curly braces, e.g. `/foo/{foo}/bar/{bar}`. Append a `...` to the name of the last path argument to capture the remainder of the path, e.g. `/my-path/{greedy...}`. Path arguments must span an entire segment of the route. A route such as `/x{bad}x` is not allowed.

Start a route with `//` or `https://` to set an absolute path mapped to the root of the ingress proxy (e.g. `//my/path` becomes `https://webserver.hostname/my/path`). The special route `//root` maps to `/`.

### Ports

Ports provide access isolation. Use port `:0` to accept requests on any port. Conventional port assignments:

- `:443` - default port for standard endpoints (functions and web handlers)
- `:444` - internal-only endpoints that should not be accessible from outside the bus
- `:417` - dedicated port for outbound events, blocking external requests from reaching event endpoints
- `:428` - dedicated port for task endpoints, blocking external requests from reaching workflow tasks
- `:888` - internal management and control endpoints
- `:0` - wildcard, accepts requests on any port (used for cross-cutting concerns like OpenAPI)

### Magic HTTP Arguments

Functional endpoints use typed Go function signatures for their inputs and outputs. Three special argument names - `httpRequestBody`, `httpResponseBody` and `httpStatusCode` - provide direct control over HTTP request/response semantics from within a functional endpoint.

- **`httpRequestBody`** - Input argument deserialized directly from the HTTP request body. Other inputs come from query/path arguments.
- **`httpResponseBody`** - Output argument serialized as the sole HTTP response body. Other outputs are excluded.
- **`httpStatusCode`** - `int` output argument that sets the HTTP status code (defaults to `200`).

These magic arguments are commonly used together to expose a functional endpoint as a REST API:

```go
// CreateREST is a POST /objects endpoint
func (svc *Service) CreateREST(ctx context.Context, httpRequestBody *Object) (objKey ObjectKey, httpStatusCode int, err error) {
	// httpRequestBody is deserialized from the request body
	objKey, err = svc.Create(ctx, httpRequestBody)
	if err != nil {
		return objKey, 0, errors.Trace(err)
	}
	return objKey, http.StatusCreated, nil
}

// LoadREST is a GET /objects/{key} endpoint
func (svc *Service) LoadREST(ctx context.Context, key ObjectKey) (httpResponseBody *Object, httpStatusCode int, err error) {
	obj, ok, err := svc.Load(ctx, key)
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	if !ok {
		return nil, http.StatusNotFound, nil
	}
	return obj, http.StatusOK, nil
}
```

### Authentication and Authorization

Microbus uses JWT-based authentication. Endpoints can require specific claims using `requiredClaims` boolean expressions (e.g. `roles.admin && iss=~"access.token.core"`). See the manifest sections below for syntax.

**IMPORTANT**: If the microservice uses `act.Of(ctx)`, imports auth-related packages (`bearertokenapi`, `accesstokenapi`), or the task involves setting up authentication infrastructure, read `.claude/rules/auth.txt` before proceeding.

### Required JWT Claims

`requiredClaims` is a boolean expression over access token claims. Supported syntax:

- Boolean operators `&&`, `||` and `!`
- Comparison operators `==` and `!=`
- Order operators `>`, `>=`, `<` and `<=`
- Regexp operators `=~` and `!~`. The regular expression must be quoted and on the right, e.g. `prop=~"regexp"`
- Grouping operators `(` and `)`
- String quotation marks `"` or `'`
- Dot notation `.` for traversing nested objects. For this purpose, arrays are treated as sets, enabling expressions such as `roles.admin` to check whether the value `"admin"` is present in the claim `"roles": ["admin", "elevated"]`

Best practice: include `iss=~"access.token.core"` to ensure requests carry a properly minted access token. The original identity provider is available in the `idp` claim.

### SQL CRUD Microservices

SQL CRUD microservices are Microbus microservices that expose a CRUD API to persist and retrieve objects in and out of a SQL database. They follow a standardized pattern for multi-tenant isolation, schema migration, column mapping, query filtering, and optimistic concurrency control via revisions.

**IMPORTANT**: If the microservice uses SQL (imports `database/sql` or `github.com/microbus-io/sequel`), read `.claude/rules/sequel.txt` before proceeding.

### Agentic Workflows

Agentic workflows allow microservices to collaborate on multi-step processes. A workflow is a directed graph of tasks orchestrated by the Foreman core service. Each task is a standalone endpoint that reads from and writes to a shared state carried by a `*workflow.Flow`. Tasks are registered on port `:428` by default.

**IMPORTANT**: If the microservice defines tasks or workflows (has `tasks` or `workflows` in its `manifest.yaml`), read `.claude/rules/workflows.txt` before proceeding.

### Manifest

Each microservice contains a `manifest.yaml` that documents its features. Read all manifests when starting work on a project to build a mental map of the system.

**IMPORTANT**: The manifest describes the code but does not generate it. When changing the code of the microservice, update the manifest to match.

#### General

The `general` section of the manifest describes its general properties.

- The `hostname` must be unique across the application
- The `frameworkVersion` is the version of the `github.com/microbus-io/fabric` package in `go mod` at the time when the microservice was last edited
- If the microservice depends on either the `github.com/microbus-io/sequel` or the `database/sql` packages, enter `SQL` for the `db` property
- If the microservice makes outgoing calls to a web API (likely via the HTTP egress proxy), enter the hostname of the web API for `cloud`. If more than one hostname is contacted, enter `various`
- The `modifiedAt` timestamp records when the manifest was last updated, in RFC 3339 format with the actual current UTC time (not midnight)

```yaml
general:
  name: My service
  hostname: my.service.hostname
  description: MyService does X.
  package: github.com/mycompany/myproject/myservice
  frameworkVersion: 1.23.0
  db: SQL
  cloud: api.example.com
  modifiedAt: "2025-01-15T14:30:00Z"
```

#### Downstream Dependencies

The `downstream` section lists microservices whose `*api` package is imported for `Client` or `MulticastClient` use. `MulticastTrigger` or `Hook` usage does not count.

```yaml
downstream:
  - hostname: another.service.hostname
    package: github.com/mycompany/myproject/anotherservice
```

#### Webs

The `webs` section of the manifest describes the web handler endpoints of the microservice.

- The `method` can be any valid HTTP method, or `ANY` to indicate that the endpoint handles all methods
- The `loadBalancing` indicates how requests to this endpoint are distributed among peers:
  - `default` - load-balanced among peers using the hostname as the queue name
  - `none` - multicast to all peers (no queue)
  - a custom queue name (e.g. `worker.pool`) - load-balanced among peers that share the same queue name, and multicast across groups with different queue names. Queue names must match `^[a-zA-Z0-9\.]+$`
- `requiredClaims` is a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied

```yaml
webs:
  MyWeb:
    description: MyWeb does X.
    method: GET
    route: :1234/my-web
    loadBalancing: default
    requiredClaims: roles.manager || roles.director
```

#### Functions

The `functions` section of the manifest describes the functional endpoints (RPCs) of the microservice. Fields `method`, `loadBalancing`, and `requiredClaims` follow the same conventions as webs.

```yaml
functions:
  MyFunction:
    signature: MyFunction(argIn1 int, argIn2 MyStruct) (argOut1 map[string]bool, argOut2 bool)
    description: MyFunction does X.
    method: GET
    route: /my-function
    loadBalancing: default
    requiredClaims: level>5 && !guest
```

#### Outbound Events

The `outboundEvents` section of the manifest describes the outbound events triggered by the microservice.

```yaml
outboundEvents:
  OnMyEvent:
    signature: OnMyEvent(argIn1 int, argIn2 MyStruct) (argOut1 map[string]bool, argOut2 bool)
    description: OnMyEvent is triggered when X.
    method: POST
    route: :417/on-my-event
```

#### Inbound Events

The `inboundEvents` section of the manifest describes the inbound events that the microservice is listening to. Fields `loadBalancing` and `requiredClaims` follow the same conventions as webs.

```yaml
inboundEvents:
  OnMyEvent:
    signature: OnMyEvent(argIn1 int, argIn2 MyStruct) (argOut1 map[string]bool, argOut2 bool)
    description: OnMyEvent is triggered when X.
    loadBalancing: default
    requiredClaims: admin || manager
    source: package/path/of/event/source/microservice
```

#### Configuration Properties

The `configs` section of the manifest describes the configuration properties of the microservice.

- The `signature` describes the getter of the configuration property, including its return type: `string`, `int`, `float64`, `time.Duration` or `bool`
- `secret` indicates the value should not be logged
- `callback` indicates an `OnChanged` callback is triggered when the value changes
- The `validation` pattern is enforced over the value of the configuration property
  - `str ^[a-zA-Z0-9]+$` - string that matches a regular expression
  - `bool` - boolean
  - `int [0,60]` - integer in range
  - `float [0.0,1.0)` - floating point number in range
  - `dur (0s,24h]` - duration in range
  - `set Red|Green|Blue` - one of a set of options
  - `url` - URL
  - `email` - email address
  - `json` - serialized JSON
- Range notation: `[` inclusive, `(` exclusive. Either bound can be omitted for open-ended ranges, e.g. `int [1,]` (minimum 1, no maximum) or `dur [,24h]` (no minimum, maximum 24h)

```yaml
configs:
  MyConfig:
    signature: MyConfig() (myConfig int)
    description: MyConfig is X.
    validation: int [1,100]
    default: 10
    secret: false
    callback: false
```

#### Tickers

The `tickers` section of the manifest describes recurring operations of the microservice.

- The `interval` is the duration between iterations

```yaml
tickers:
  MyTicker:
    signature: MyTicker()
    description: MyTicker does X.
    interval: 5m
```

#### Metrics

The `metrics` section of the manifest describes metrics produced by the microservice.

- `signature` describes the value type and labels of the metric
- The metric `kind` is either `counter`, `gauge` or `histogram`
- `buckets` are the boundaries of a histogram's buckets
- `otelName` is the name of the metric in OpenTelemetry
- `observable` metrics are measured just-in-time

```yaml
metrics:
  MyMetric:
    signature: MyMetric(value int, label1 string, label2 string)
    description: MyMetric measures X.
    kind: histogram
    buckets: [1, 5, 10, 50, 100]
    otelName: my_metric
    observable: false
```

#### Tasks

The `tasks` section of the manifest describes task endpoints used in agentic workflows.

- The `signature` excludes `ctx context.Context`, `flow *workflow.Flow`, and `err error`
- For output arguments with an `Out` suffix (read-modify-write pattern), include them as-is in the signature

```yaml
tasks:
  MyTask:
    signature: MyTask(inArg1 string, inArg2 float64) (outArg1 bool)
    description: MyTask does X.
    route: :428/my-task
    requiredClaims: roles.manager || roles.director
```

#### Workflows

The `workflows` section of the manifest describes workflow graph endpoints that define the structure of agentic workflows.

- The `signature` lists the workflow's declared input and output fields. Input fields (before the return parens) are what the workflow expects in its initial state. Output fields (inside the return parens) are what the workflow produces in its final state. Field types are informational - the actual state is untyped JSON.

```yaml
workflows:
  MyWorkflow:
    signature: MyWorkflow(inputField1 string, inputField2 float64) (outputField1 bool)
    description: MyWorkflow defines the workflow graph for X.
    route: :428/my-workflow
```

### Markers

Each feature of a microservice has corresponding code scattered across multiple files: the handler in `service.go`, the subscription in `intermediate.go`, the client stub in `*api/client.go`, the mock in `mock.go`, and the test in `service_test.go`. To locate all code related to a specific feature, search for `// MARKER: FeatureName` comments within the microservice's directory. For example, to find all code related to the `Hello` feature of the `hello.example` microservice, search for `MARKER: Hello` under its directory.

Markers are scoped to a single microservice - different microservices can define features with the same name, so always search within the specific microservice's directory.

## Project Structure

- Each microservice lives in its own directory with a `*api/` subdirectory for its public interface (client stubs, types)
- The main app is at `main/main.go`
- `config.yaml` sets configuration properties, scoped by microservice hostname or `all:`. Secrets go in git-ignored `config.local.yaml`
  ```yaml
  my.service:
    MyConfig: value
  all:
    SharedConfig: true
  ```
- `env.yaml` overrides OS environment variables. Secrets go in git-ignored `env.local.yaml`

## Common Patterns

### Working With Static Resource Files

Place static resource files in the `resources` directory of the microservice, or a subdirectory thereof. These files are embedded inside the binary using `go:embed`.

Resource files are accessed in the code using `svc.ReadResFile` or `svc.ResFS`.

If the file is a text or HTML template, use `svc.WriteResTemplate(w, "template.html", data)` to read and execute it in a single operation. HTML templates must be named with a `.html` extension for proper escaping. `svc.WriteResTemplate` does not support func maps or custom delimiters - use the standard library pattern if either is required.

### Calling Downstream Microservices

Use the generated client stub for type-safe communication with downstream microservices. Four client types are available in each `*api` package:

- **`NewClient`** - unicast (one-to-one) request/response. Returns a single result, load-balanced among peers.
- **`NewMulticastClient`** - multicast (one-to-many) request/response. Returns an iterator of zero or more responses from all peers.
- **`NewMulticastTrigger`** - fires outbound events defined in the microservice's API. Used by the event source to notify subscribers.
- **`NewHook`** - subscribes to inbound events from another microservice. Used by event sinks to react to events.

If making a one-to-one request/response call, use the standard client.

```go
import "package/path/of/downstream/downstreamapi"

func (svc *Service) ProcessOrder(ctx context.Context, orderID string) error {
	validated, err := downstreamapi.NewClient(svc).ValidateOrder(ctx, orderID)
	if err != nil {
		return errors.Trace(err)
	}
	// ...
	return nil
}
```

If making a one-to-many pub/sub call, use the multicast client to iterate through the zero or more responses.

```go
import "package/path/of/downstream/downstreamapi"

func (svc *Service) ProcessOrder(ctx context.Context, orderID string) error {
	for e := range downstreamapi.NewMulticastClient(svc).ValidateOrder(ctx, orderID) {
		validated, err := e.Get()
		if err != nil {
			return errors.Trace(err)
		}
		// ...
	}
	// ...
	return nil
}
```

To fire an outbound event, use the multicast trigger from within the event source microservice. To fire and forget, call the trigger without iterating over its responses.

```go
myserviceapi.NewMulticastTrigger(svc).OnOrderCreated(ctx, order)
```

To fire and wait for responses from event sinks, iterate over the returned sequence.

```go
for r := range myserviceapi.NewMulticastTrigger(svc).OnOrderCreated(ctx, order) {
	result, err := r.Get()
	if err != nil {
		return errors.Trace(err)
	}
	// ...
}
```

To subscribe to an inbound event from another microservice, use the hook. This is typically done in `OnStartup`.

```go
import "package/path/of/eventsource/eventsourceapi"

func (svc *Service) OnStartup(ctx context.Context) (err error) {
	hook := eventsourceapi.NewHook(svc)
	hook.OnOrderCreated(svc.onOrderCreated)
	return nil
}

func (svc *Service) onOrderCreated(ctx context.Context, order Order) error {
	// React to the event...
	return nil
}
```

### Logging

Use `svc.LogDebug`, `svc.LogInfo`, `svc.LogWarn` and `svc.LogError` to print to the log. Logs include attributes in the `slog` name=value pair pattern, e.g. `svc.LogError(ctx, "Job failed", "job", jobID, "error", err)`. Use the label `"error"` when logging an `error`. In most cases there is no need to log errors - any error returned from an endpoint is automatically logged by Microbus.

### Distributed Tracing

Every call to an endpoint is automatically wrapped with a trace span. The span can be accessed via `svc.Span(ctx)` and extended with `SetAttributes` or `LogInfo` using the slog name=value pair pattern. All downstream service calls automatically participate in the trace.

### Goroutines

Use `svc.Go(ctx, func)` to launch a goroutine in the context of a microservice. Use `svc.Parallel(func1, func2, ...)` to launch multiple goroutines and wait for all to complete.

### Making HTTP Web Requests

HTTP requests to the web should use the HTTP egress proxy rather than the standard Go `http.Client`.

```go
func (svc *Service) FetchListFromWeb(ctx context.Context) (result []string, err error) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := httpegressapi.NewClient(svc).Do(ctx, req)
	// Process response here...
	return result, nil
}
```

Add the HTTP egress proxy to the main app in `main/main.go`, if not already added.

### Miscellaneous

- **Current Time**: Use `svc.Now(ctx)` to get the current time rather than `time.Now()`

### Recording Metrics

Use `svc.IncrementMyMetric(ctx, delta)` for counters and `svc.RecordMyMetric(ctx, value)` for gauges and histograms. The method names are generated from the metric name defined in the manifest.

### Mocking

#### Mocking Microservices

A microservice's `Mock` provides type-safe methods for mocking all its endpoints. Add mocks to testing applications in lieu of real microservices.

```go
func TestPayment_ChargeUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Create a mock of the webpay microservice, mocking its Charge endpoint
	webpayMock := webpay.NewMock()
	webpayMock.MockCharge(func(ctx context.Context, userID string, amount int) (success bool, balance int, err error) {
		return true, 100, nil
	})

	// Initialize the testers
	tester := connector.New("tester.client")
	client := payment.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
		webpayMock,
	)
	app.RunInTest(t)

	// ...
}
```

#### Mocking the HTTP Egress Proxy

Mock the HTTP egress proxy with `httpegress.NewMock()` to avoid real network requests. Extract the proxied request with `http.ReadRequest(bufio.NewReader(r.Body))`.

```go
func TestMyService_ExternalAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	httpEgressMock := httpegress.NewMock()

	tester := connector.New("tester.client")
	client := myserviceapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		httpEgressMock,
		tester,
	)
	app.RunInTest(t)

	t.Run("fetch_data", func(t *testing.T) {
		httpEgressMock.MockMakeRequest(func(w http.ResponseWriter, r *http.Request) (err error) {
			req, _ := http.ReadRequest(bufio.NewReader(r.Body))
			if req.Method == "GET" && req.URL.String() == "https://api.example.com/data" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"ok"}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return nil
		})
		defer httpEgressMock.MockMakeRequest(nil)

		// Test against the mock...
	})
}
```

Only the wrapper's own tests should mock the egress proxy directly. Upstream microservices should mock the wrapper itself using its `NewMock()`.

### Error Handling

#### Tracing Errors

Use `errors.Trace(err)` to associate the current stack location with the error. Stack traces are preserved across microservice boundaries. Attach properties using the `slog` name=value pair pattern, and HTTP status codes as unpaired arguments: `errors.Trace(err, "userID", userID, http.StatusBadRequest)`.

#### New Errors

Use `errors.New` to create new errors. Do not use the deprecated constructors `errors.Newc`, `errors.Newf` or `errors.Newcf`.
Error strings should not be capitalized (unless beginning with proper nouns or acronyms), nor end with punctuation.

```go
if count == 0 {
	return errors.New("no objects")
}
```

`errors.New` supports `fmt`-style formatting, `slog` name=value properties, HTTP status codes as unpaired arguments, and error wrapping (unpaired error argument, equivalent to `%w`). All can be combined:

```go
file, err := os.Open(fileName)
if err != nil {
	return errors.New("failed to open file '%s'", fileName,
		"fileName", fileName,
		http.StatusBadRequest,
		err,
	)
}
```

### Caching

The distributed cache allows multiple replicas of the same microservice to share a single cache.
Access the distributed cache at `svc.DistribCache`.
Use `Get`, `Set`, `Delete`, `Peek`, `Has` or `Clear` to interact with the distributed cache.
Initialize the cache in the microservice's `OnStartup` callback, with the distributed cache's `SetMaxAge` and `SetMaxMemory`.

### Reading Path Argument Values

To read the value of a path argument in a web handler, use `r.PathValue("argumentName")`. Do not use `httpx.PathValues`.

```go
// RenderObjectProperty's route is /object/{id}/property/{prop}

func (svc *Service) RenderObjectProperty(w http.ResponseWriter, r *http.Request) (err error) {
	id := r.PathValue("id")
	prop := r.PathValue("prop")
	// ...
}
```

To read the value of a path argument in a functional endpoint, name an argument the same as the path argument. If the function argument type is not a string, an attempt will be made to convert it to the right type.

```go
// LoadObject's route is /load/{category}/{name...}

func (svc *Service) LoadObject(ctx context.Context, category int, name string) (object SomeObject, err error) {
	// ...
}
```

### Connecting to Remote APIs

Wrap each remote API (e.g. a third-party web service) in its own microservice. This:
- Encapsulates connection logic, URLs, and request/response structures in one place
- Stores credentials in a single secret configuration property
- Generates type-safe client stubs for use by upstream microservices
- Enables mocking of the remote system in tests

Create a functional endpoint or web handler for each operation of the remote API. Use the HTTP egress proxy for making the outbound HTTP requests.

## Conventions

### Naming Conventions

- PascalCase for types: `User`, `OrderStatus`
- PascalCase for public functions: `GetUserById`, `ProcessOrder`
- camelCase for private functions: `getUserById`, `processOrder`
- lowercase for hostnames: `userservice.example`
- kebab-case for URL routes and paths: `/annual-report`
- lowercase for file names: `userservice.go, mytype.go`
- camelCase for JSON tags: `json:"myProperty,omitzero"`

### OpenAPI Parameter Descriptions

Two complementary conventions enrich the OpenAPI spec with per-field descriptions, improving documentation quality and LLM tool-calling accuracy.

#### Godoc `Input:` and `Output:` Sections

When a functional endpoint has non-trivial parameters or results, include structured `Input:` and/or `Output:` sections in its godoc comment. The OpenAPI generator extracts these sections and populates per-field descriptions in the JSON Schema output. The full godoc is also preserved as the endpoint's overall description.

```go
/*
Forecast returns the weather forecast for a location.

Input:
  - city: The city name, e.g. "San Francisco"
  - days: Number of days to forecast, 1-14

Output:
  - forecast: Daily forecast summaries
  - confidence: Model confidence score, 0.0 to 1.0
*/
func (svc *Service) Forecast(ctx context.Context, city string, days int) (forecast []DayForecast, confidence float64, err error)
```

These sections are optional. If absent, the endpoint description is still the full godoc and per-field descriptions are simply empty.

#### `jsonschema` Description Tags on Custom Types

When defining custom struct types used in APIs, add short `jsonschema:"description=..."` tags to each field. The `invopop/jsonschema` library reads these tags and emits `description` fields in the generated JSON Schema, which flow through to the OpenAPI spec.

```go
type DayForecast struct {
    Date    string  `json:"date,omitzero" jsonschema:"description=Date is the forecast date in ISO 8601 format"`
    High    float64 `json:"high,omitzero" jsonschema:"description=High is the high temperature in Fahrenheit"`
    Low     float64 `json:"low,omitzero" jsonschema:"description=Low is the low temperature in Fahrenheit"`
    Summary string  `json:"summary,omitzero" jsonschema:"description=Summary is a brief weather summary"`
}
```

Together, godoc sections cover scalar function arguments while `jsonschema` tags cover fields within complex types.

## Development Workflow

### Building a Microservice With Skills

Always use skills in `.claude/skills/` to build microservices. Scaffold with the appropriate skill (e.g. `microbus/add-microservice` or `sequel/add-microservice`), then use `add-feature` skills for each feature.

The available feature skills are:

| Skill | Feature |
|---|---|
| `microbus/add-config` | Configuration property |
| `microbus/add-metric` | Metric |
| `microbus/add-outbound-event` | Outbound event |
| `microbus/add-function` | Functional endpoint (RPC) |
| `microbus/add-web` | Web handler endpoint |
| `microbus/add-inbound-event` | Inbound event sink |
| `microbus/add-task` | Task endpoint (agentic workflow step) |
| `microbus/add-workflow` | Workflow graph (agentic workflow definition) |
| `microbus/add-ticker` | Ticker |

The recommended order is configs, metrics, outbound events, functions, webs, inbound events, tasks, workflows, then tickers. This order is not mandatory but it follows the natural dependency chain - for example, a function may reference a configuration property or record a metric, so those should exist first. Workflows reference task endpoints, so tasks should be defined first.

### Building the Project

Use `go vet ./main/...` to verify compilation (not `go build`, which conflicts with the `main/` directory name). For a binary: `go build -o app ./main/...`.

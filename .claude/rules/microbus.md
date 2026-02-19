# Developing With the Microbus Framework

## Instructions for Agents

**CRITICAL**: This project follows a microservices architecture based on the Microbus framework. Always follow the patterns and conventions in this file.

**CRITICAL**: Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

**CRITICAL**: After compaction of the context window, re-read this file.

**CRITICAL**: Comments in the code that include the word `HINT` or `MARKER` in all-caps are there to guide you. Do not remove them.

## Overview

Microbus is a holistic open source framework for developing, testing, deploying and operating microservices at scale. It combines best-in-class OSS, tooling and best practices into a dramatically simplified engineering experience.

## Key Concepts

A Microbus application comprises of a multitude of microservices that communicate over a messaging bus using the HTTP protocol.

A microservice consists of the following features:
- **Web handler endpoints** — raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Functional endpoints (RPCs)** — typed request/response functions with input/output structs, marshalling, and client stubs
- **Outbound events** — messages this microservice fires for others to consume
- **Inbound event sinks** — handlers that react to events emitted by other microservices
- **Configuration properties** — runtime settings (strings, durations, booleans, etc.)
- **Tickers** — recurring operations on a schedule
- **Metrics** — counters, gauges, and histograms for observability

### Internal vs External URLs

Microbus microservices are reachable from other microservices by means of an internal URL that is only addressable inside the messaging bus. An external client, such as an HTTP browser, must use the HTTP ingress microservice to bridge the gap between real HTTP and the messaging bus. The proxy transforms the external-facing URL to the internal one. For example, the external URL `https://webserver.hostname/myservice.hostname/my/path` is transformed into the internal Microbus URL `https://myservice.hostname/my/path` by using the first part of the path as the internal hostname.

### Subscription Route

The subscription route of an endpoint is resolved relative to the hostname of the microservice to form the internal Microbus URL of the endpoint. In turn, this becomes the path of the external URL. For example, the route `/my/path` of a microservice with hostname `myservice.hostname` is resolved to the internal Microbus URL `https://myservice.hostname/my/path` which is accessible from the external URL `https://webserver.hostname/myservice.hostname/my/path`.

It's recommended to set the route of a subscription to match the name of its handler, in kebab-case, e.g. `/my-handler`.

Set a port by prepending it to the route, e.g. `:123/my-path`. Use port `:0` to accept requests on any port, e.g. `:0/my-path`. The default port is `:443`.

Encase path arguments in curly braces, e.g. `/foo/{foo}/bar/{bar}`. Append a `...` to the name of the last path argument to captures the remainder of the path, e.g. `/my-path/{greedy...}`. Path arguments must span an entire segment of the route. A route such as `/x{bad}x` is not allowed.

Start a route with `//` to override the default hostname of the microservice and expose a clean URL to external users. For example, the route `//my/path` results in the external URL `https://webserver.hostname/my/path` which omits the hostname of the microservice. A good use of these types of routes is for web handlers that produce user-facing HTML pages.

### Required JWT Claims

Required JWT claims are expressed as a boolean expression over the JWT claims associated with the request, that when not met causes the request to be denied. The boolean expression supports the following syntax:
- Boolean operators `&&`, `||` and `!`
- Comparison operators `==` and `!=`
- Order operators `>`, `>=`, `<` and `<=`
- Regexp operators `=~` and `!~`. The regular expression must be quoted and on the right, e.g. `prop=~"regexp"`
- Grouping operators `(` and `)`
- String quotation marks `"` or `'`
- Dot notation `.` for traversing nested objects. For this purpose, arrays are construed as `map[string]bool`, enabling expressions such as `array.value` to match the claim `"array": ["value", "another_value"]`

### Manifest

Each microservice contains a `manifest.yaml` that concisely catalogs its features. The manifest is documentation of the code, not a definition used to generate it. Reading a microservice's manifest is a much faster way to understand what it does than reading its code. When starting to work on a project, read all `manifest.yaml` files across the project to build a mental map of the system and keep it in mind.

**IMPORTANT**: The manifest describes the code but does not generate it. When changing the code of the microservice, update the manifest to match.

#### General

The `general` section of the manifest describes its general properties.

- The `hostname` must be unique across the application
- The `frameworkVersion` is the version of `Microbus` when the microservice was last modified

```yaml
general:
  hostname: my.service.hostname
  description: MyService does X.
  package: github.com/mycompany/myproject/myservice
  frameworkVersion: 1.22.0 
```

#### Downstream Dependencies

The `downstream` section of the manifest lists other microservices that this microservice depends on. Microservice X depends on microservice Y when X imports Y's `*api` package to use its `Client`, `MulticastClient`, event `MulticastTrigger`, or event `Hook`.

```yaml
downstream:
  - hostname: another.service.hostname
    package: github.com/mycompany/myproject/anotherservice
```

#### Webs

The `webs` section of the manifest describes the web handler endpoints of the microservice.

- The `method` can be any valid HTTP method, or `ANY` to indicate that the endpoint handles all methods
- The `loadBalancing` indicates how requests to this endpoint are distributed among peers:
  - `default` - load-balanced among peers
  - `none` - multicast to all peers
  - other - load-balanced among peers with the same queue name, and multicast to others
- `requiredClaims` is a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied

```yaml
webs:
  MyWeb:
    description: MyWeb does X.
	method: GET
	route: :1234/my-web
	loadBalancing: default
	requiredClaims: roles=~"manager|director"
```

#### Functions

The `functions` section of the manifest describes the functional endpoints (RPCs) of the microservice.

- The `method` can be any valid HTTP method, or `ANY` to indicate that the endpoint handles all methods
- The `loadBalancing` indicates how requests to this endpoint are distributed among peers:
  - `default` - load-balanced among peers
  - `none` - multicast to all peers
  - other - load-balanced among peers with the same queue name, and multicast to others
- `requiredClaims` is a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied

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

The `inboundEvents` section of the manifest describes the inbound events that the microservice is listening to.

- The `loadBalancing` indicates how requests to this endpoint are distributed among peers:
  - `default` - load-balanced among peers
  - `none` - multicast to all peers
  - other - load-balanced among peers with the same queue name, and multicast to others
- `requiredClaims` is a boolean expression over the JWT claims associated with the request that if not met will cause the request to be denied

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

- The `type` is the Go data type of the configuration property: `string`, `int`, `float64`, `time.Duration` or `bool`
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

```yaml
configs:
  MyConfig:
    description: MyConfig is X.
    type: int
	validation: int [1,100]
	default: 10
```

#### Tickers

The `tickers` section of the manifest describes recurring operations of the microservice.

- The `internal` is the duration between iterations

```yaml
tickers:
  MyTicker:
    description: MyTicker does X.
	internal: 5m
```

#### Metrics

The `metrics` section of the manifest describes metrics produced by the microservice.

- The `otelName` is the name of the metric in OpenTelemetry
- The metric `kind` is either `counter`, `gauge` or `histogram`
- The `buckets` are the boundaries of a histogram's buckets
- `observable` metrics are measured just-in-time

```yaml
metrics:
  MyMetric:
	otelName: my_metric
    description: MyMetric measures X.
	kind: histogram
	buckets: [1, 5, 10, 50, 100]
	observable: false
```

### Markers

Each feature of a microservice has corresponding code scattered across multiple files: the handler in `service.go`, the subscription in `intermediate.go`, the client stub in `*api/client.go`, the mock in `mock.go`, and the test in `service_test.go`. To locate all code related to a specific feature, search for `// MARKER: FeatureName` comments within the microservice's directory. For example, to find all code related to the `Hello` feature of the `hello.example` microservice, search for `MARKER: Hello` under its directory.

Markers are scoped to a single microservice — different microservices can define features with the same name, so always search within the specific microservice's directory.

## Project Structure

```
├── .claude/                    # Claude Code setup
│   ├── rules/                  # Instructions for coding agents
│   └── skills/                 # Claude Code skills
├── .vscode/
│   └── launch.json             # VSCode launch file
├── main/
│   ├── env.yaml                # Environment variables for main app
│   └── main.go                 # Main application
├── microservice/               # Each microservice has its own directory
│   ├── microserviceapi/        # The public interface of this microservice
│   |   ├── client.go           # Generated client API
│   │   └── [type.go]           # Generated definition for each type used in the API
│   ├── resources/              # Embedded resource files
│   │   ├── embed.go            # go:embed directive
│   │   └── [your files]        # Static files, configs, etc.
│   ├── AGENTS.md               # Local instructions to the coding agent
│   ├── CLAUDE.md               # Local instructions for Claude
│   ├── intermediate.go         # Service infrastructure
│   ├── manifest.yaml           # Manifest of features
│   ├── mock.go                 # Mock for testing purposes
│   ├── PROMPTS.md              # Audit trail of prompts
│   ├── service_test.go         # Integration tests
│   └── service.go              # Implementation
├── .gitignore
├── AGENTS.md                   # Instructions for coding agents
├── CLAUDE.md                   # Instructions for Claude
├── config.yaml                 # Configuration properties
├── config.local.yaml           # git ignored configuration properties
├── env.yaml                    # Environment variables
├── env.local.yaml              # git ignored environment variables
├── go.mod
└── go.sum
```

## Common Patterns

### Working With Static Resource Files

Place static resource files in the `resources` directory of the microservice, or a subdirectory thereof. These files are embedded inside the binary using `go:embed`.

Resource files are accessed in the code using `svc.ReadResFile` or `svc.ResFS`

```go
func (svc *Service) LoadSQLFile() error {
	data, err := svc.ReadResFile("sql/create-table.sql")
	if err != nil {
		return errors.Trace(err)
	}
	// Process the data...
	return nil
}
```

If the file is a text or HTML template, you may use `svc.WriteResTemplate` to read and execute it in a single operation.
HTML templates must be named with a `.html` extension for variable content to be appropriately escaped.
`svc.WriteResTemplate` does not support customizing execution with a func map or changing the delimiters.
If either is required, use the standard library pattern instead.

```go
func (svc *Service) GreetingPage(w http.ResponseWriter, r *http.Request) (err error) {
	data := struct {
		Name string
	}{
		Name: "Peter",
	}
	err = svc.WriteResTemplate(w, "greeting-template.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

### Calling Downstream Microservices

Use the generated client stub of the downstream microservice for type-safe communication with other microservices. Client stubs are lightweight and can be created on a per-call basis.

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

### Logging

Use the logging functions `svc.LogDebug`, `svc.LogInfo`, `svc.LogWarn` and `svc.LogError` to print to the log.
Logs may include attribute in the `slog` name=value pair pattern. Use the label `"error"` when logging an `error`.

```go
func (svc *Service) RunJob(ctx context.Context, jobID string) (err error) {
	t0 := svc.Now(ctx)
	svc.LogInfo(ctx, "Starting job", "job", jobID)
	err := runJob(jobID)
	if err != nil {
		svc.LogError(ctx, "Job failed",
			"job", jobID,
			"error", err,
		)
	} else {
		svc.LogInfo(ctx, "Job succeeded",
			"job", jobID,
			"dur", svc.Now(ctx).Sub(t0),
		)
	}
    return errors.Trace(err)
}
```

In most cases there is no need to log errors. Any error returned from an endpoint is automatically logged by Microbus.

### Launching Goroutines

Use `svc.Go` to launch goroutines in the context of a microservice.

```go
func (svc *Service) RunJobAsync(ctx context.Context) (err error) {
	svc.Go(ctx, func(ctx context.Context) (err error) {
		// Implement Go routine here...
		return err
	})
	return nil
}
```

### Parallel Execution

Use `svc.Parallel` to launch and wait for the completions of multiple goroutines in the context of a microservice.

```go
func (svc *Service) Multitask(ctx context.Context) (err error) {
	err = svc.Parallel(
		func() (err error) {
			// Implement job 1 here...
			return err
		},
		func() (err error) {
			// Implement job 2 here...
			return err
		},
		// etc...
	)
	return errors.Trace(err)
}
```

### Making HTTP Web Requests

HTTP requests to the web should use the HTTP egress proxy rather than the standard Go `http.Client`.

#### Step 1: Add the HTTP Egress Proxy to the Main App

Add the HTTP egress proxy to the main app in `main/main.go`, if not already added.
`main.go` is typically located in the `main` directory of the project, not inside the directory of the microservice.

#### Step 2: Use the Client of the HTTP Egress Proxy

Use the client of the HTTP egress proxy to make the request rather than Go's standard HTTP client.
The HTTP egress proxy takes care of setting a timeout and recording metrics.

```go
import "github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"

func (svc *Service) FetchListFromWeb(ctx context.Context) (result []string, err error) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	if err != nil {
		return errors.Trace(err)
	}
	resp, err := httpegressapi.NewClient(svc).Do(ctx, req)
	if err != nil {
		return errors.Trace(err)
	}
	// Process response here...
	return result, nil
}
```

### Configuration File

Configuration property values can be set in a `config.yaml` placed in the working directory of the application, or in an ancestor directory.

```yaml
# Set configuration properties for microservices with a hostname of my.service or *.my.service
my.service:
  ExampleString: my value
  ExampleDuration: 1h

# Set configuration properties for all microservices
all:
  ExampleBoolean: true
```

Secret configuration property values can be set in `config.local.yaml`, which is git ignored.

### Environment Variables File

The environment variables of the operating system can be overridden by setting their values in an `env.yaml` file placed in the working directory of the application, or in an ancestor directory.

```yaml
ENVAR_NAME: ENVAR_VALUE
```

Some common environment variables:

```yaml
# NATS connection settings
MICROBUS_NATS: nats://127.0.0.1:4222
MICROBUS_NATS_USER: my-nats-user
MICROBUS_NATS_PASSWORD: my-nats-password
MICROBUS_NATS_TOKEN: my-nats-authentication-token

# The deployment environment: LOCAL, TESTING, LAB, PROD
MICROBUS_DEPLOYMENT: LOCAL

# OpenTelemetry endpoint
OTEL_EXPORTER_OTLP_PROTOCOL: grpc
OTEL_EXPORTER_OTLP_ENDPOINT: http://127.0.0.1:4317
```

Secret environment variables can be set in `env.local.yaml`, which is git ignored.

### Extending the Service Struct

You can extend the `Service` struct of the microservice with member variables.

```go
type Service struct {
    *Intermediate  // IMPORTANT: Do not remove
    
    // Add custom fields here...
    cache map[string]any
    db    *sql.DB
}
```

### Lifecycle Callbacks

Initialize the microservice's resources in the `OnStartup` callback, and clean them up in the `OnShutdown` callback appropriately.

```go
// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
    // Initialize the microservice here...
    return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
    // Clean up resources here...
    return nil
}
```

### Miscellaneous

- **Context Propagation**: Always pass the context through the call chain for proper tracing
- **Current Time**: Use `svc.Now(ctx)` to get the current time rather than `time.Now()`
- **Main App**: The main app is typically located at `main/main.go` relative to the root directory of the project

### Naming Conventions

- PascalCase for types: `User`, `OrderStatus`
- PascalCase for public functions: `GetUserById`, `ProcessOrder`
- camelCase for private functions: `getUserById`, `processOrder`
- lowercase for hostnames: `userservice.example`
- kebab-case for URL routes and paths: `/annual-report`
- UPPER_CASE for constants: `MAX_RETRIES`, `API_VERSION`
- lowercase for file names: `userservice.go, mytype.go`
- camelCase for JSON tags: `json:"myProperty,omitzero"`  

### Recording Metrics

Use `IncrementMyMetric` function to count occurrences of an operation or event in a counter metric.

```go
func (svc *Service) Hello(ctx context.Context, name string) (result string, err error) {
	svc.IncrementHelloOccurrences(ctx, 1)
    return "Hello, " + name, nil
}
```

Use `RecordMyMetric` function to set the value of a gauge metric. Gauge values can go up or down over time.

```go
func (svc *Service) Hello(ctx context.Context, name string) (result string, err error) {
	concurrent := svc.atomicCounter.Add(1)
    defer svc.atomicCounter.Add(-1)
    svc.RecordConcurrentHellos(ctx, concurrent)
    return "Hello, " + name, nil
}
```

Use `RecordMyMetric` function to update the value of a histogram metric.

```go
func (svc *Service) Hello(ctx context.Context, name string) (result string, err error) {
    svc.RecordHelloNameLengths(ctx, len(name))
    return "Hello, " + name, nil
}
```

### Import Structure

```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "net/http"

    // 2. Microbus packages
    "github.com/microbus-io/fabric/connector"
    "github.com/microbus-io/errors"

    // 3. Third-party packages
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"

    // 4. Local imports
    "mycompany/myproject/myservice/myserviceapi"
)
```

### Mocking Microservices

Sometimes using the actual microservice in tests is impossible or undesirable because it depends on a resource that is unavailable in the testing environment. For example, a microservice that makes requests to a third-party web service could be mocked in order to avoid depending on that service for development.

A microservice's `Mock` includes type-safe methods for mocking all its endpoints. Mocks can be added to testing applications in lieu of the real microservices.

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

### Error Handling

#### Tracing Errors

Use `errors.Trace` to associate the current stack location with the error. Stack traces are preserved across microservice boundaries.

```go
err := doSomething()
if err != nil {
	return errors.Trace(err)
}
```

Attach properties to errors using the `slog` name=value pair pattern. These can be used to provide more context about the circumstances of the error or to attach meta-data to it.
To associate an HTTP status code with the error, add it as an unpaired argument.

```go
err := doSomething(userID)
if err != nil {
	return errors.Trace(err,
		"userID", userID,
		http.StatusBadRequest,
	)
}
```

#### New Errors

Use `errors.New` to create new errors. Do not use the deprecated constructors `errors.Newc`, `errors.Newf` or `errors.Newcf`.
Error strings should not be capitalized (unless beginning with proper nouns or acronyms), nor end with punctuation.

```go
if count == 0 {
	return errors.New("no objects")
}
```

`errors.New` supports the `fmt.Error` pattern.

```go
file, err := os.Open(fileName)
if errors.Is(err, os.ErrNotExist) {
	return errors.New("file not found '%s'", fileName)
}
```

Attach properties to errors using the `slog` name=value pair pattern. These can be used to provide more context about the circumstances of the error or to attach meta-data to it.
To associate an HTTP status code with the error, add it as an unpaired argument.
To wrap another error, add it as an unpaired argument. Wrapping another error is the equivalent of using `%w` at the end of the format string.

```go
file, err := os.Open(fileName)
if errors.Is(err, os.ErrNotExist) {
	return errors.New("file not found",
		"fileName", fileName,
		http.StatusNotFound,
		err,
	)
}
```

The `fmt.Error` pattern can be combined with attached properties, status codes and wrapper errors.

```go
file, err := os.Open(fileName)
if err != nil && !errors.Is(err, os.ErrNotExist) {
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

### Keep Track of Prompts

When working on a microservice, keep track of the prompts affecting it in a `PROMPTS.md` file in its directory. Rephrase the language of the saved prompts to include context that was not made explicit in the original language. The intent is have an auditable trail of the prompts, and to allow a future agent to reproduce the functionality of the microservice from these prompts.

Save each prompt under a `## Title`.

```md
## Prompt title

Prompt comes here...
```

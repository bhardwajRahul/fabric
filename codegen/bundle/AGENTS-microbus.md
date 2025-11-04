# Developing with the Microbus Framework

## Instructions for Agents

**CRITICAL**: This project follows a microservices architecture based on the Microbus framework. Always follow the patterns and conventions in this file.

**CRITICAL**: Before performing any Microbus-related tasks, check if there's a relevant skill in `.claude/skills/microbus/` and follow the relevant skill workflow.

**CRITICAL**: After compaction of the context window, re-read this file.

## Overview

Microbus is a holistic open source framework for developing, testing, deploying and operating microservices at scale. It combines best-in-class OSS, tooling and best practices into a dramatically simplified engineering experience.

## Key Concepts

A Microbus application comprises of a multitude of microservices that communicate over a messaging bus using the HTTP protocol.

A microservice consists of the following features:
- Configuration properties
- Functional endpoints (RPCs)
- Outbound events
- Inbound event sinks
- Web handler endpoints
- Tickers (recurring operations)
- Metrics

The HTTP ingress and egress microservices bridge the gap between real HTTP and Microbus.

## Project Structure

```
├── .claude/                     # Claude Code setup
│   └── skills/				     # Claude Code skills
├── .vscode/
│   └── launch.json              # VSCode launch file
├── main/
│   ├── config.yaml              # Configuration file
│   ├── env.yaml                 # Environment settings, supplementing and overriding the system environment variables
│   └── main.go                  # Main application
├── microservice/                # Each microservice has its own directory
│   ├── intermediate/            # Generated intermediate files
│   │   ├── intermediate-gen.go  # Service infrastructure (do not edit)
│   │   └── mock-gen.go          # Mock of the microservice, for testing purposes (do not edit)
│   ├── microserviceapi/         # The public interface of this microservice
│   |   ├── client-gen.go        # Generated client API (do not edit)
│   │   └── type.go              # Generated definition for each type used in the API
│   ├── resources/               # Embedded resource files
│   │   ├── embed-gen.go         # go:embed directive (do not edit)
│   │   └── [your files]         # Static files, configs, etc.
│   ├── AGENTS.md                # Local instructions to the coding agent
│   ├── CLAUDE.md                # Refer Claude to AGENTS.md
│   ├── doc.go                   # Package doc with the go:generate directive
│   ├── service_test.go          # Integration tests
│   ├── service-gen.go           # Generated boilerplate (do not edit)
│   ├── service.go               # Microservice implementation
│   └── service.yaml             # Microservice definition (start here)
├── AGENTS-MICROBUS.md           # Instructions to coding agents for Microbus (do not edit)
├── AGENTS.md                    # Instructions to coding agents for this project
└── CLAUDE.md                    # Refer Claude to AGENTS.md
```

## Common Patterns

### Working with Static Resource Files

Place static resource files in the `resources` directory of the microservice, or a sub-directory thereof, to embed them inside the binary using `go:embed`.

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

If the file is a text or HTML template, you may use `svc.ExecuteResTemplate` to read and execute it in one operation.
HTML templates must be named with a `.html` extension for variable content to be properly escaped.

```go
func (svc *Service) GreetingPage(w http.ResponseWriter, r *http.Request) (err error) {
	data := struct {
		Name string
	}{
		Name: "Peter",
	}
	rendered, err := svc.ExecuteResTemplate("greeting-template.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	w.Write(rendered)
	return nil
}
```

### Calling Downstream Microservices

Use the generated client stub of the downstream microservice for type-safe communication.

```go
import "path/to/downstreamapi"

func (svc *Service) ProcessOrder(ctx context.Context, orderID string) error {
    // Create client for the downstream microservice and make type-safe call
    validated, err := downstreamapi.NewClient(svc).ValidateOrder(ctx, orderID)
    if err != nil {
        return errors.Trace(err)
    }
    // ...
    return nil
}
```

### Logging

Use the logging functions `svc.LogDebug`, `svc.LogInfo`, `svc.LogWarn` and `svc.LogError` to print to the log.

Use the label `"error"` when logging an `error`.

In most cases, there is no need to log errors. Any error returned from an endpoint is automatically logged by Microbus.

```go
func (svc *Service) RunJob(ctx context.Context, jobID string) (err error) {
	t0 := svc.Now(ctx)
	svc.LogInfo(ctx, "Starting job", "job", jobID)
	err := runJob(jobID)
	if err != nil {
		svc.LogError(ctx, "Job failed", "job", jobID, "error", err)
	} else {
		svc.LogInfo(ctx, "Job succeeded", "job", jobID, "dur", svc.Now(ctx).Sub(t0))
	}
    return errors.Trace(err)
}
```

### Launching Goroutines

Use `svc.Go` to launch goroutines in the context of a microservice.

```go
func (svc *Service) RunJobAsync(ctx context.Context) (err error) {
	svc.Go(ctx, func(ctx context.Context) (err error) {
		// Implement Go routine here
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
			// Implement job 1 here
			return err
		},
		func() (err error) {
			// Implement job 2 here
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

Add the HTTP egress proxy to the `app` in `main.go`, if not already added.
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
	// Process response...
	return result, nil
}
```

### Configuration File

Configuration property values can be set in a `config.yaml` file that must be placed in the working directory when launching the application.

```yaml
# Set configuration properties for the myservice.hostname microservice
myservice.hostname:
  ExampleString: my value
  ExampleDuration: 1h

# Set configuration properties for all microservices
all:
  UseDatabase: true
```

A microservice must define a configuration property of the same name in `service.yaml`.

### Environment Variables File

The environment variables of the operating system can be overridden by setting their values in an `env.yaml` file that must be placed in the working directory when launching the application.

```yaml
ENVAR_NAME: ENVAR_VALUE
```

The `env.yaml` file must be placed in the working directory when launching the application.

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

### Extending the Service Struct

You can extend the `Service` struct of the microservice with custom fields.
Do not remove the `Intermediate`!

```go
type Service struct {
    *intermediate.Intermediate  // DO NOT REMOVE
    
    // Add custom fields here
    cache map[string]any
    db *sql.DB
}
```

### Lifecycle Callbacks

Initialize the microservice's resources in the `OnStartup` callback, and clean them up in the `OnShutdown` callback appropriately.

```go
// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
    // Initialize the microservice here
    return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
    // Clean up resources here
    return nil
}
```

### Miscellaneous

1. **Generated Files**: Never edit files with `-gen.go` or `-gen_test.go` suffix - they will be overwritten
2. **Context Propagation**: Always pass the context through the call chain for proper tracing
3. **Routing**: The URL to RPC functional endpoints and web handler endpoints is prefixed with the host name of the microservice, e.g. `/myservice.myproject.mycompany/my-endpoint`
4. **Current Time**: Use `svc.Now(ctx)` to get the current time rather than `time.Now()`
5. **Main App**: The main app is typically located in `main.go` in the `main` directory of the project

### Naming Conventions

- PascalCase for types: User, OrderStatus
- PascalCase for public functions: GetUserById, ProcessOrder  
- camelCase for private functions: getUserById, processOrder  
- lowercase for host names: userservice.example
- kebab-case for URL paths: /annual-report
- UPPER_CASE for constants: MAX_RETRIES, API_VERSION
- lowercase for file names: userservice.go, mytype.go
- camelCase for JSON tags: `json:"myProperty,omitzero"`  

### Recording Metrics

Use the code-generated `AddMetricName` function to count occurrences of an operation or event in a `counter` metric.

```go
func (svc *Service) Hello(ctx context.Context, name string) (result string, err error) {
	svc.AddHelloOccurrences(ctx, 1)
    return "Hello, " + name, nil
}
```

Use the code-generated `RecordMetricName` function to update the value of a `gauge` metric.
Gauge values can go up or down over time.

```go
func (svc *Service) Hello(ctx context.Context, name string) (result string, err error) {
	concurrent := svc.atomicCounter.Add(1)
    defer svc.atomicCounter.Add(-1)
    svc.RecordConcurrentHellos(ctx, concurrent)
    return "Hello, " + name, nil
}
```

Use the code-generated `RecordMetricName` function to update the value of a `histogram` metric.

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
    "github.com/microbus-io/fabric/errors"

    // 3. Third-party packages
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"

    // 4. Local imports
    "mycompany/myproject/myservice/myserviceapi"
)
```

### Mocking Microservices

Sometimes, using the actual microservice in tests is impossible or undesirable because it depends on a resource that is unavailable in the testing environment. For example, a microservice that makes requests to a third-party web service could be mocked in order to avoid depending on that service for development.

The code generator creates a `Mock` for every microservice that includes type-safe methods for mocking all its endpoints. Mocks are created with `NewMock` and added to testing applications in lieu of the real microservices.

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
	tester := connector.New("payment.chargeuser.tester")
	client := payment.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
		webpayMock,
	)
	app.RunInTest(t)

	// ...
}
```

### Error Handling

Use `errors.Trace(err)` to preserve stack traces across microservice boundaries.

```go
err := doSomething()
if err != nil {
	return errors.Trace(err)
}
```

Attach properties to errors using the slog name=value pair pattern. These can be used to provide more context about the circumstances.
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

Use `errors.New` to create new errors.
Do not use the deprecated constructors `errors.Newc`, `errors.Newf` or `errors.Newcf`.
Error strings should not be capitalized (unless beginning with proper nouns or acronyms), nor end with punctuation.

```go
if count == 0 {
	return errors.New("no objects")
}
```

`errors.New` supports the `fmt.Error` pattern as well.

```go
file, err := os.Open(fileName)
if errors.Is(err, os.ErrNotExist) {
	return errors.New("file not found '%s'", fileName)
}
```

Properties can be attached in `errors.New` using the slog name=value pair pattern.
To associate an HTTP status code with the error, add it as an unpaired argument.
To wrap another error, add it as an unpaired argument.

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
Access the distributed cache at `svc.DistribCache()`.
Use `Get`, `Set`, `Delete`, `Peek`, `Has` or `Clear` to interact with the distributed cache.
If appropriate, initialize the cache in the microservice's `OnStartup` callback, with the distributed cache's `SetMaxAge` and `SetMaxMemory`.

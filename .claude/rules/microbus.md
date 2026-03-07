# Developing With the Microbus Framework

## Instructions for Agents

**CRITICAL**: This project follows a microservices architecture based on the Microbus framework. Always follow the patterns and conventions in this file.

**CRITICAL**: Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

**CRITICAL**: After compaction of the context window, re-read this file.

**CRITICAL**: Comments in the code that include the word `HINT` or `MARKER` in all-caps are there to guide you. Do not remove them.

## Overview

Microbus is a holistic open source framework for developing, testing, deploying and operating microservices at scale. It combines best-in-class OSS, tooling and best practices into a dramatically simplified engineering experience.

## Key Concepts

A Microbus application comprises a multitude of microservices that communicate over a messaging bus using the HTTP protocol.

A microservice consists of the following features:
- **Web handler endpoints** — raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Functional endpoints (RPCs)** — typed request/response functions with input/output structs, marshaling, and client stubs
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

An empty route `""` or `/` maps to the root path of the microservice.

Set a port by prepending it to the route, e.g. `:123/my-path`. A port alone with no path is also valid, e.g. `:123`. The default port is `:443`.

Encase path arguments in curly braces, e.g. `/foo/{foo}/bar/{bar}`. Append a `...` to the name of the last path argument to capture the remainder of the path, e.g. `/my-path/{greedy...}`. Path arguments must span an entire segment of the route. A route such as `/x{bad}x` is not allowed.

Start a route with `//` or `https://` to set an absolute path that is mapped to the root of the HTTP ingress proxy by the Root middleware. For example, the route `//my/path` results in the external URL `https://webserver.hostname/my/path` which omits the hostname of the microservice. The special route `//root` maps to the absolute root `/` of the ingress, i.e. `https://webserver.hostname/`. A good use of these types of routes is for web handlers that produce user-facing HTML pages.

### Ports

Ports provide access isolation. The HTTP ingress proxy can be configured to only forward external requests to specific ports, making endpoints on other ports unreachable from external clients.

Ports are set by prepending them to the route as described in [Subscription Route](#subscription-route). Use port `:0` to accept requests on any port, e.g. `:0/openapi.json`. If not specified, the default port is `:443`.

Conventional port assignments:

- `:443` — default port for standard endpoints (functions and web handlers)
- `:444` — internal-only endpoints that should not be accessible from outside the bus
- `:417` — dedicated port for outbound events, blocking external requests from reaching event endpoints
- `:888` — internal management and control endpoints
- `:0` — wildcard, accepts requests on any port (used for cross-cutting concerns like OpenAPI)

### Magic HTTP Arguments

Functional endpoints use typed Go function signatures for their inputs and outputs. Three special argument names — `httpRequestBody`, `httpResponseBody` and `httpStatusCode` — provide direct control over HTTP request/response semantics from within a functional endpoint.

- **`httpRequestBody`** — When used as an input argument, the HTTP request body is deserialized directly into this argument rather than being parsed as part of the standard input struct. All other input arguments are sourced from query parameters or path arguments. Can be any JSON-deserializable type: pointer to struct, slice, map, or primitive.
- **`httpResponseBody`** — When used as an output argument, only this argument is serialized as the HTTP response body. All other output arguments are excluded from the response. Can be any JSON-serializable type: pointer to struct, slice, map, or primitive.
- **`httpStatusCode`** — When used as an `int` output argument, its value sets the HTTP response status code. If zero or omitted, the default status code `200 OK` is used.

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

#### Enabling Authentication

The `microbus/init-project` skill sets up the token issuers, authorization middleware, and `act` package automatically. The remaining piece each solution must provide is an **authenticator** — a microservice that validates credentials (e.g. username/password, OAuth code) and issues a bearer token:

```go
bearertokenapi.NewClient(svc).Mint(ctx, map[string]any{
	"sub": "user@example.com",
	"uid": 12345,
	"tid": 123,
})
```

The authenticator returns the signed JWT to the caller (e.g. via a `Set-Cookie` header or JSON response). From that point, the ingress middleware handles token exchange and claims propagation automatically.

#### Bearer Tokens and Access Tokens

Microbus uses a two-token architecture for authentication and authorization.

A **bearer token** is a long-lived JWT issued by an external identity provider (e.g. Auth0, Okta) or by the built-in `bearer.token.core` service. It identifies the actor (user or system) and is presented to the system via the `Authorization: Bearer <token>` header or an `Authorization` cookie.

An **access token** is a short-lived internal JWT minted by the `access.token.core` service. It is signed with ephemeral keys that rotate automatically and expires quickly (default 20 seconds). Access tokens are not meant to be stored or reused — they are created on-the-fly for each request.

#### Token Exchange Flow

When the HTTP ingress proxy receives a request with a bearer token, it automatically exchanges it for an access token:

1. The ingress authorization middleware extracts the bearer token from the request
2. It verifies the bearer token's signature against the issuer's JWKS (JSON Web Key Set)
3. It calls `access.token.core` to mint a short-lived access token, passing in the verified claims
4. The access token service applies any registered claims transformers to enrich the claims (e.g. adding roles, tenant ID, or permissions from a user database)
5. The access token service sets critical claims (`iss`, `idp`, `iat`, `exp`, `jti`) and signs the token
6. The signed access token is attached to the request and propagated automatically to all downstream microservices via the `Microbus-Actor` header

#### Claims Enrichment

Claims transformers allow enriching the access token with additional claims during minting. Register transformers in `main/main.go` when initializing the access token service:

```go
accesstoken.NewService().Init(func(svc *accesstoken.Service) (err error) {
	svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
		// Look up the user's roles and tenant from your database and add them to the claims
		claims["roles"] = []string{"admin", "user"}
		claims["tid"] = 1234
		return nil
	})
	return nil
})
```

#### The Actor Pattern

The `Actor` struct in the `act` package is a convenience pattern for representing the claims associated with a request. It is created during project setup and can be extended to fit the needs of the solution. Its properties are organized into four groups: standard claims (`iss`, `sub`, `exp`, `idp`), identifiers (e.g. tenant ID, user ID), security claims (roles, groups, scopes), and user preferences (name, locale, time zone). JSON tag names should follow the [IANA JWT Claims Registry](https://www.iana.org/assignments/jwt/jwt.xhtml#claims) where a registered claim name exists.

Use `act.Of(ctx)` to extract the actor from the request context:

```go
func (svc *Service) MyEndpoint(ctx context.Context) error {
	actor, err := act.Of(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	// ...
	return nil
}
```

#### Actor Impersonation

To call a downstream microservice with modified claims, mint a new access token with the adjusted claims and attach it via `pub.Token`:

```go
func (svc *Service) ElevatedAction(ctx context.Context) error {
	actor, err := act.Of(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	actor.Roles = append(actor.Roles, "admin")
	elevatedToken, err := accesstokenapi.NewClient(svc).Mint(ctx, actor)
	if err != nil {
		return errors.Trace(err)
	}
	err = downstreamapi.NewClient(svc).
		WithOptions(pub.Token(elevatedToken)).
		AdminAction(ctx)
	return errors.Trace(err)
}
```

### Required JWT Claims

Required JWT claims are expressed as a boolean expression over the JWT claims of the access token associated with the request, that must be met for the request to be allowed. The boolean expression supports the following syntax:

- Boolean operators `&&`, `||` and `!`
- Comparison operators `==` and `!=`
- Order operators `>`, `>=`, `<` and `<=`
- Regexp operators `=~` and `!~`. The regular expression must be quoted and on the right, e.g. `prop=~"regexp"`
- Grouping operators `(` and `)`
- String quotation marks `"` or `'`
- Dot notation `.` for traversing nested objects. For this purpose, arrays are treated as sets, enabling expressions such as `roles.admin` to check whether the value `"admin"` is present in the claim `"roles": ["admin", "elevated"]`

It is best practice to include a condition on the `iss` claim to ensure that the request carries a properly minted access token rather than an unexchanged bearer token or other credential. When using the default access token service, this can be done with `iss=~"access.token.core"` or `iss=="microbus://access.token.core"`. The original identity provider (the issuer of the bearer token that was exchanged for the access token) is available in the `idp` claim.

### Manifest

Each microservice contains a `manifest.yaml` that concisely catalogs its features. The manifest is documentation of the code, not a definition used to generate it. Reading a microservice's manifest is a much faster way to understand what it does than reading its code. When starting to work on a project, read all `manifest.yaml` files across the project to build a mental map of the system and keep it in mind.

**IMPORTANT**: The manifest describes the code but does not generate it. When changing the code of the microservice, update the manifest to match.

#### General

The `general` section of the manifest describes its general properties.

- The `hostname` must be unique across the application
- The `frameworkVersion` is the version of the `github.com/microbus-io/fabric` package in `go mod` at the time when the microservice was last edited
- If the microservice depends on either the `github.com/microbus-io/sequel` or the `database/sql` packages, enter `SQL` for the `db` property
- If the microservice makes outgoing calls to a web API (likely via the HTTP egress proxy), enter the hostname of the web API for `cloud`. If more than one hostname is contacted, enter `various`
- The `modifiedAt` timestamp records when the manifest was last updated, in RFC 3339 format

```yaml
general:
  name: My service
  hostname: my.service.hostname
  description: MyService does X.
  package: github.com/mycompany/myproject/myservice
  frameworkVersion: 1.23.0
  db: SQL
  cloud: api.example.com
  modifiedAt: "2025-01-15T00:00:00Z"
```

#### Downstream Dependencies

The `downstream` section of the manifest lists other microservices that this microservice depends on. Microservice X depends on microservice Y when X imports Y's `*api` package to use its `Client` or `MulticastClient`. Usage of Y's `MulticastTrigger` or `Hook` does not constitute a downstream dependency in this context.

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
    requiredClaims: roles=~"manager|director"
```

#### Functions

The `functions` section of the manifest describes the functional endpoints (RPCs) of the microservice.

- The `method` can be any valid HTTP method, or `ANY` to indicate that the endpoint handles all methods
- The `loadBalancing` indicates how requests to this endpoint are distributed among peers:
  - `default` - load-balanced among peers using the hostname as the queue name
  - `none` - multicast to all peers (no queue)
  - a custom queue name (e.g. `worker.pool`) - load-balanced among peers that share the same queue name, and multicast across groups with different queue names. Queue names must match `^[a-zA-Z0-9\.]+$`
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
  - `default` - load-balanced among peers using the hostname as the queue name
  - `none` - multicast to all peers (no queue)
  - a custom queue name (e.g. `worker.pool`) - load-balanced among peers that share the same queue name, and multicast across groups with different queue names. Queue names must match `^[a-zA-Z0-9\.]+$`
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

## Common Patterns

### Working With Static Resource Files

Place static resource files in the `resources` directory of the microservice, or a subdirectory thereof. These files are embedded inside the binary using `go:embed`.

Resource files are accessed in the code using `svc.ReadResFile` or `svc.ResFS`.

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

Use the generated client stub of the downstream microservice for type-safe communication with other microservices. Client stubs are lightweight and can be created on a per-call basis. Four client types are available in each microservice's `*api` package:

- **`NewClient`** — unicast (one-to-one) request/response. Returns a single result, load-balanced among peers.
- **`NewMulticastClient`** — multicast (one-to-many) request/response. Returns an iterator of zero or more responses from all peers.
- **`NewMulticastTrigger`** — fires outbound events defined in the microservice's API. Used by the event source to notify subscribers.
- **`NewHook`** — subscribes to inbound events from another microservice. Used by event sinks to react to events.

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

Use the logging functions `svc.LogDebug`, `svc.LogInfo`, `svc.LogWarn` and `svc.LogError` to print to the log.
Logs may include attributes in the `slog` name=value pair pattern. Use the label `"error"` when logging an `error`.

```go
func (svc *Service) RunJob(ctx context.Context, jobID string) (err error) {
	t0 := svc.Now(ctx)
	svc.LogInfo(ctx, "Starting job", "job", jobID)
	err = runJob(jobID)
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

### Distributed Tracing

Every call to an endpoint is automatically wrapped with a trace span. The span can be accessed via `svc.Span(ctx)` and extended with attributes or error recordings. The span uses the slog name=value pair pattern for attributes.

```go
func (svc *Service) ProcessOrder(ctx context.Context, orderID string) error {
	span := svc.Span(ctx)
	span.SetAttributes("order.id", orderID)

	// All downstream service calls automatically participate in the trace
	user, err := userserviceapi.NewClient(svc).GetUser(ctx, orderID)
	if err != nil {
		return errors.Trace(err)
	}

	span.LogInfo("User retrieved successfully",
		"user", user.Name,
	)
	return nil
}
```

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
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := httpegressapi.NewClient(svc).Do(ctx, req)
	if err != nil {
		return errors.Trace(err)
	}
	// Process response here...
	return result, nil
}
```

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

### Mocking

#### Mocking Microservices

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

#### Mocking the HTTP Egress Proxy

When testing a microservice that makes outbound HTTP calls via the HTTP egress proxy, mock the proxy to avoid real network requests. Use `httpegress.NewMock()` and `MockMakeRequest` to intercept and simulate external responses.

The mock handler receives the proxied HTTP request inside the body of the handler's `*http.Request`. Use `http.ReadRequest(bufio.NewReader(r.Body))` to extract the original request, then write the simulated response to the `http.ResponseWriter`.

```go
import (
	"bufio"
	"net/http"

	"github.com/microbus-io/fabric/coreservices/httpegress"
)

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

This pattern is especially important for microservices that wrap remote APIs — every such microservice should include tests with a mocked egress proxy to validate request construction and response handling without depending on the external service. Setting up the mock per test case keeps each case self-contained and avoids leaking mock behavior between cases.

Upstream microservices that depend on a remote API wrapper should mock the wrapper microservice itself (using its `NewMock()`), not the HTTP egress proxy. Only the wrapper's own tests should mock the egress proxy directly.

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

### Connecting to Remote APIs

Wrap each remote API (e.g. a third-party web service) in its own microservice. This:
- Encapsulates connection logic, URLs, and request/response structures in one place
- Stores credentials in a single secret configuration property
- Generates type-safe client stubs for use by upstream microservices
- Enables mocking of the remote system in tests

Create a functional endpoint or web handler for each operation of the remote API. Use the HTTP egress proxy for outbound calls.

## Conventions

### Naming Conventions

- PascalCase for types: `User`, `OrderStatus`
- PascalCase for public functions: `GetUserById`, `ProcessOrder`
- camelCase for private functions: `getUserById`, `processOrder`
- lowercase for hostnames: `userservice.example`
- kebab-case for URL routes and paths: `/annual-report`
- UPPER_CASE for constants: `MAX_RETRIES`, `API_VERSION`
- lowercase for file names: `userservice.go, mytype.go`
- camelCase for JSON tags: `json:"myProperty,omitzero"`

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

## Development Workflow

### Building a Microservice With Skills

Always use the skills in `.claude/skills/` to build a microservice. Start by scaffolding the microservice with the appropriate skill (e.g. the standard `microbus/add-microservice`, or a specialized skill like `sequel/add-microservice`), then use the appropriate `add-feature` skill for each feature. Each skill must be followed sequentially to completion before starting the next.

The available feature skills are:

| Skill | Feature |
|---|---|
| `microbus/add-config` | Configuration property |
| `microbus/add-metric` | Metric |
| `microbus/add-outbound-event` | Outbound event |
| `microbus/add-function` | Functional endpoint (RPC) |
| `microbus/add-web` | Web handler endpoint |
| `microbus/add-inbound-event` | Inbound event sink |
| `microbus/add-ticker` | Ticker |

The recommended order is configs, metrics, outbound events, functions, webs, inbound events, then tickers. This order is not mandatory but it follows the natural dependency chain — for example, a function may reference a configuration property or record a metric, so those should exist first.

### Parallel Development

Independent microservices can be developed in parallel.

### Building the Project

To build the project, use `go vet` instead of `go build` to verify compilation without producing a binary. This avoids the "build output already exists and is a directory" error caused by the `main/` directory conflicting with the default output binary name.

```shell
go vet ./main/...
```

If a binary is actually needed, specify an explicit output name:

```shell
go build -o app ./main/...
```

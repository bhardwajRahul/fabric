---
name: Adding a new microservice
description: Creates and initializes a new microservice. Use when explicitly asked by the user to create a new microservice.
---

**CRITICAL**: Do NOT explore or analyze existing microservices before starting. The templates in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Creating a new microservice:
- [ ] Step 1: Create a directory structure for the new microservice
- [ ] Step 2: Prepare coding agent files
- [ ] Step 3: Prepare client.go
- [ ] Step 4: Prepare embed.go
- [ ] Step 5: Prepare service.go
- [ ] Step 6: Prepare intermediate.go
- [ ] Step 7: Prepare mock.go
- [ ] Step 8: Prepare service_test.go
- [ ] Step 9: Prepare manifest.yaml
- [ ] Step 10: Add to main app
- [ ] Step 11: Design microservice
```

#### Step 1: Create a Directory Structure for the New Microservice

Each microservice must be placed in a separate directory. Create a new directory for the new microservice.
Use only lowercase letters `a` through `z` for the name of the directory.

In smaller projects, place the new directory under the root directory of the project.
In larger projects, consider using a nested directory structure to group similar microservices together.

```shell
mkdir -p myservice
```

```shell
cd myservice
```

Create two subdirectories.

- The first should be a concatenation of the microservice directory and the suffix `api`
- The second subdirectory should be named `resources`

```shell
mkdir -p myservice/myserviceapi
mkdir -p myservice/resources
```

The directory structure should look like this.

```
myproject/
└── myservice/
    ├── myserviceapi/
    └── resources/
```

**IMPORTANT**: File names in the following steps are relative to the new `myservice` directory, unless indicated otherwise.

File preparation steps can be performed in parallel.

#### Step 2: Prepare Coding Agent Files

Create `AGENTS.md` with the following content verbatim.

```md
**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

**IMPORTANT**: Keep track of prompts affecting this microservices in `PROMPTS.md`.

**IMPORTANT**: Keep track of features of this microservices in `manifest.yaml`.
```

Create `CLAUDE.md` with the following content verbatim.

```md
**CRITICAL**: Read `AGENTS.md` immediately.
```

Create `PROMPTS.md` with the prompt to create this microservice.

#### Step 3: Prepare `client.go`

Create `myserviceapi/client.go` with the following content.

- The `Hostname` constant holds the hostname in which this microservice will be addressable. It must be unique across the application. Use reverse domain notation based on the module path, up to the name of the organization. For example, if the module path is `github.com/my-company/myproject/some/path/myservice`, set the hostname to `myservice.path.some.myproject.my-company`. Only letters `a-z`, numbers `0-9`, hyphens `-` and the dot `.` separator are allowed in the hostname

```go
package myserviceapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
)

var (
	_ context.Context
	_ json.Encoder
	_ *http.Request
	_ *errors.TracedError
	_ *httpx.BodyReader
)

// Hostname is the default hostname of the microservice.
const Hostname = "myservice.myproject.mycompany"

// Endpoint routes.
const ()

// Endpoint URLs.
var ()

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c Client) ForHost(host string) Client {
	return Client{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c Client) WithOptions(opts ...pub.Option) Client {
	return Client{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{
		svc:  listener,
		host: Hostname,
	}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{
		svc:  c.svc,
		host: host,
		opts: c.opts,
	}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{
		svc:  c.svc,
		host: c.host,
		opts: append(c.opts, opts...),
	}
}
```

#### Step 4: Prepare `embed.go`

Create `resources/embed.go` with the following content verbatim.

```go
package resources

import "embed"

//go:embed *
var FS embed.FS
```

#### Step 5: Prepare `service.go`

Create `service.go` with the following content.

- Match the package name to the directory name
- Set the comment of the type definition of `Service` to describe this particular microservice. The provided value is a template. Do not copy it verbatim

```go
package myservice

import (
	"context"
	"net/http"
	
	"github.com/microbus-io/errors"

	"github.com/mycompany/myproject/myservice/myserviceapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ myserviceapi.Client
)

/*
Service implements myservice which does X.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}
```

#### Step 6: Prepare `intermediate.go`

Create `intermediate.go` with the following content.

- In `svc.SetDescription`, replace `MyService does X` with a description of this particular microservice.

```go
package myservice

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"
	
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/mycompany/myproject/myservice/myserviceapi"
	"github.com/mycompany/myproject/myservice/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ myserviceapi.Client
)

const (
	Hostname    = myserviceapi.Hostname
	Version     = 1
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	impl ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Service{
		Connector: connector.New(Hostname),
		impl:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`MyService does X.`)
	svc.SetOnStartup(svc.impl.OnStartup)
	svc.SetOnShutdown(svc.impl.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
		// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	return nil
}
```

#### Step 7: Prepare `mock.go`

Create `mock.go` with the following content.

```go
package myservice

import (
	"context"
	"net/http"
	
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/errors"

	"github.com/mycompany/myproject/myservice/myserviceapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ myserviceapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}
```

Note that it is the intention of `Mock` to shadow the functions of `Service`.

#### Step 8: Prepare `service_test.go`

Create `service_test.go` with the following content.

- Match the package name to the directory name
- The imports are pre-declared for convenience now, before adding test code later
- Be sure to include `TestMyService_OpenAPI` even though initially it is a no op

```go
package myservice

import (
	"context"
	"io"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/mycompany/myproject/myservice/myserviceapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ testarossa.Asserter
	_ myserviceapi.Client
)

func TestMyService_OpenAPI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	ports := []string{
		// HINT: Include all ports of functional or web endpoints
	}
	for _, port := range ports {
		t.Run("port_" + port, func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := tester.Request(
				ctx,
				pub.GET(httpx.JoinHostAndPath(myserviceapi.Hostname, ":"+port+"/openapi.json")),
			)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.Contains(body, "openapi")
				}
			}
		})
	}
}

func TestMyService_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})
}
```

#### Step 9: Prepare `manifest.yaml`

Look in `go.mod` and identify the current version of the `github.com/microbus-io/fabric` dependency. This is the framework version. Set it in `manifest.yaml` next.

Create `manifest.yaml` with the following content.

```yaml
general:
  hostname: myservice.myproject.mycompany
  description: MyService does X.
  package: github.com/mycompany/myproject/myservice
  frameworkVersion: 1.22.0 
```

#### Step 10: Add to Main App

Find `main/main.go` relative to the project root. Add the new microservice to the app in the `main` function. Add the appropriate import statement at the top of the file

```go
import (
	// ...
	"github.com/mycompany/myproject/myservice"
)

func main() {
	// ...
	app.Add(
		// HINT: Add solution microservices here
		myservice.NewService(),
	)
	// ...
}
```

#### Step 11: Design Microservice

Skip this step if instructed to be "quick" or to skip designing this microservice.

Based on the context provided by the user, propose a set of features for this microservice for each of the following categories.

- **Configuration properties** — runtime settings (strings, durations, booleans, etc.) read via the connector
- **Functional endpoints (RPCs)** — typed request/response functions with input/output structs, marshalling, and client stubs
- **Web handler endpoints** — raw `http.ResponseWriter`/`http.Request` handlers for serving HTML, files, or custom HTTP responses
- **Outbound events** — messages this microservice fires for others to consume
- **Inbound event sinks** — handlers that react to events emitted by other microservices
- **Tickers** — recurring operations on a schedule
- **Metrics** — counters, gauges, and histograms for observability

Save the design to `DESIGN.md`, show it to the user and seek additional instructions.
Do not implement any of the proposed features without explicit approval from the user, unless instructed to run autonomously.

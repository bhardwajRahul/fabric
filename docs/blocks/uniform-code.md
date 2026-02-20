# Uniform Code Structure

Coding agents are instructed to create numerous files and subdirectories in the directory of a microservice.

```
myservice/                  # Each microservice has its own directory
├── myserviceapi/           # The public interface of this microservice
│   ├── client.go           # Generated client API
│   └── [type].go           # Generated definition for each type used in the API
├── resources/              # Embedded resource files
│   ├── embed.go            # go:embed directive
│   └── [your files]        # Static files, configs, etc.
├── AGENTS.md               # Local instructions to the coding agent
├── CLAUDE.md               # Local instructions for Claude
├── intermediate.go         # Service infrastructure
├── manifest.yaml           # Manifest of [features](../blocks/features.md)
├── mock.go                 # Mock for testing purposes
├── PROMPTS.md              # Audit trail of prompts
├── service_test.go         # Integration tests
└── service.go              # Implementation
```

The `*api` directory (and package) defines the `Client` and `MulticastClient` of the microservice and the complex types (structs) that they use. `MulticastTrigger` and `Hook` are also defined and populated if the microservice is a source of events. Together these represent the public-facing API of the microservice to upstream microservices. The name of the API directory is derived from that of the microservice in order to make it easily distinguishable in code completion tools.

The `resources` directory is a place to put [static files to be embedded](../blocks/embedded-res.md) into the executable of the microservice. Templates, images and scripts are some examples of what can potentially be embedded.

`AGENTS.md` allows setting instructions to coding agents that should be respected in the context of this microservice only. `CLAUDE.md` refers Claude Code to `AGENTS.md`. `PROMPTS.md` keeps an audit trail of the prompts that affected this microservice.

`intermediate.go` defines the `Intermediate` that serves as the base of the microservice via anonymous inclusion, in turn extending the [`Connector`](../structure/connector.md).

The `Mock` is a mockable stub of the microservice that can be used in [integration testing](../blocks/integration-testing.md) when a live version of the microservice cannot. It is defined in `mock.go`.

A test is created in `service_test.go` for each testable web handler, functional endpoint, event, event sink, ticker, config callback and metric callback of the microservice.

`service.go` is where the business logic of the microservice lives. `service.go` implements `Service`, which extends `Intermediate` as mentioned earlier. Most of the tools that a microservice needs are available through the receiver `(svc *Service)` which points to the `Intermediate` and by extension the `Connector`. It includes the methods of the `Connector` as well as type-specific methods defined in the `Intermediate`.

```go
type Intermediate struct {
	*connector.Connector
}

type Service struct {
	*intermediate.Intermediate
}

func (svc *Service) DoSomething(ctx context.Context) (err error) {
	// svc points to the Intermediate and by extension the Connector
}
```

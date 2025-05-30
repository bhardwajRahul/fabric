# Layered Architecture

Onions have layers, ogres have layers, and so does a good software architecture.

`Microbus` solutions are constructed in 5 layers:

* At the bottom of the stack is a curated selection of OSS technologies that are utilized and abstracted away by the next layer, the connector
* The [connector](../blocks/structure/connector.md) construct is the base class from which all microservices are derived. It provides a consistent API to most of the building blocks that are required for a microservice to operate and mesh with other microservices. Quite often they rely on OSS under the hood
* A [code generator](../blocks/codegen.md) brings type-safe RAD that is specific to the semantics of each individual microservice
* The core microservices and the solution microservices are built using the code generator
* Microservices are bundled together into [applications](../structure/application.md) according to the desired [topology](../blocks/topology.md)

<p></p>
<img src="./layers-1.drawio.svg">
<p></p>

## Applications

Microservices in `Microbus` are not by themselves runnable, rather they are bundled in applications that manage their lifecycle.

Typically, all microservices are bundled into a single application for [local development](../tech/local-dev.md).

Similarly, [integration tests](../blocks/integration-testing.md) are executed against an application that contains the microservice under test and its downstream dependencies.

Applications can contain any number of microservices, making them a flexible vehicle in the construction of the production [topology](../blocks/topology.md).

## Solution Microservices

These are the microservices that implement the business logic of the solution.

## Core Microservices

`Microbus` comes bundled with a few [core microservices](../structure/coreservices.md) that implement common functionality required by most if not all `Microbus` applications.

The [HTTP ingress proxy](../structure/coreservices-httpingress.md) bridges the gap between HTTP-based clients and microservices running on `Microbus`.

The [HTTP egress proxy](../structure/coreservices-httpegress.md) relays HTTP requests to non-`Microbus` URLs.

The [SMTP ingress](../structure/coreservices-smtpingress.md) microservice captures incoming emails and transforms them to actionable events.

The [configurator](../structure/coreservices-configurator.md) is responsible for delivering configuration values to microservices that define configuration properties. It is a must-have in practically all applications.

The [metrics](../structure/coreservices-metrics.md) microservice aggregates metrics from all microservices in response to a request from Prometheus.

The [OpenAPI portal](../structure/coreservices-openapiportal.md) microservice renders a catalog of the OpenAPI documents of each and every microservices.

The [token issuer](../structure/coreservices-tokenissuer.md) microservice issues and validates tokens in the form of JWTs. Tokens enable the authentication of actors and the authorization of their requests based on a set of claims.

## Code Generator

[Code generation](../blocks/codegen.md) facilitates rapid application development (RAD) by generating boilerplate and skeleton code from declarations in a [`service.yaml`](../tech/service-yaml.md) file. The developer needs only fill in the gaps and implement the business logic.

[Skeletons](../blocks/skeleton-code.md) are created for each of the microservice's endpoints with `TODO` markers for the developer to fill in the gaps. For functional endpoints (RPCs), a wrapper takes care of unmarshaling the request's JSON payload into type-safe arguments.

[Client stubs](../blocks/client-stubs.md) are created for the microservice's public endpoints. These stubs are used by upstream clients to call the microservice in a type-safe fashion. For functional endpoints (RPCs), the stubs take care of marshaling the request arguments into a JSON payload.

[Events](../blocks/events.md) are a type-safe abstraction of publish/subscribe.

The [integration test harness](../blocks/integration-testing.md) spins up the microservice under test along with the actual downstream microservices it depends on into a single testable application, allowing full-blown integration tests to run inside `go test`.

An [OpenAPI document](../blocks/openapi.md) is automatically created with descriptors for each of the microservice's endpoints.

A [uniform code structure](../blocks/uniform-code.md) is a byproduct of using code generation. A familiar code structure helps engineers get oriented quickly also when they are not the original authors of the code.

## Connector Construct

The [`Connector`](../structure/connector.md) is the base class of all microservices and provides most of their core capabilities.

### Transport

`Microbus` uses a messaging bus for the transport layer of service-to-service communications.

[Unicast request/response](../blocks/unicast.md) is an emulation of the familiar synchronous 1:1 request/response pattern of HTTP over the asynchronous messaging pattern of the underlying transport bus.

[Multicast publish/subscribe](../blocks/multicast.md) enhances the publish/subscribe pattern of the bus by introducing a familiar HTTP interface and a bi-directional 1:N request/response pattern.

Microservice are connected to the messaging bus with a [persistent multiplexed connection](../blocks/multiplexed.md) that enables holding multiple concurrent conversations on a single connection. Multiplexing results in lower resource requirements and a simplified network topology that is less prone to error.

[Time budget](../blocks/time-budget.md) is a depleting timeout that is passed downstream along the call stack. It is the proper way to handle client-to-server timeouts.

[Ack or fail fast](../blocks/ack-or-fail.md) is a pattern by which the server responds with an ack to the client before processing the request. The client knows to wait for the response only if an ack is received, and fail quickly if it's not.

A microservices transparently makes itself [discoverable](../blocks/discovery.md) by subscribing to the messaging bus. Once subscribed to a subject it immediately starts receiving message from the corresponding queue. An external service discovery system is not required.

[Load balancing](../blocks/lb.md) is handled transparently by the messaging bus. Multiple microservices that subscribe to the same queue are delivered messages randomly. An external load balancer is not required.

With [locality-aware routing](../blocks/locality-aware-routing.md) unicast requests are routed to the replica of the downstream microservice whose locality is nearest to the upstream's locality.

A microservice is [alive](../blocks/connectivity-liveness-test.md) when it is connected to the messaging bus and can send and receive messages. The bus validates the connection using regular pings. Explicit liveness checks are unnecessary.

### Observability

[Structured logs](../blocks/logging.md) are sent to `stderr`.

[Distributed tracing](../blocks/distrib-tracing.md) enables the visualization of the flow of function calls across microservices and processes. Tracing spans are automatically captured for each endpoint call.

[Metrics](../blocks/metrics.md) such as latency, duration, byte size and count are collected automatically for all microservice endpoint calls. Custom metrics may be defined by the developer. Metrics are collected and visualized by Grafana.

[Errors](../blocks/error-capture.md) are unavoidable. When they occur, they are captured, augmented with a full stack trace, logged, metered, traced and propagated up the stack to the upstream microservice.

### General

[Configuration](../blocks/configuration.md) properties are common means to initialize and customize microservices without the need for code changes. In `Microbus`, microservices define their configuration properties but obtain the runtime values of those properties from the [configurator](../structure/coreservices-configurator.md).

Microservices may stipulate that incoming requests be authenticated and authorized. Requests that do not satisfy the [authorization](../blocks/authorization.md) requirements are denied.

[Tickers](../blocks/tickers.md) are jobs that run on a recurring basis.

Images, scripts, templates and any other [static resources](../blocks/embedded-res.md) are made available to each microservice by association of a file system (`FS`).

A specially-named resource `strings.yaml` enables [internationalization (i18n)](../blocks/i18n.md) of user-facing display strings.

The [distributed cache](../blocks/distrib-cache.md) is an in-memory cache that is shared among the replica peers of microservice.

Microservices are [shutdown gracefully](../blocks/graceful-shutdown.md). All pending requests, goroutines and jobs are drained before the microservice quits.

## OSS

[NATS](https://www.nats.io) sits at the core of `Microbus` and makes much of its magic possible. NATS is a full-mesh, highly-available, lighting-fast, real-time, at-most-once, messaging bus that supports dynamic subscriptions. It enables request/response, publish/subscribe, load-balancing and dynamic discovery.

[OpenTelemetry](https://opentelemetry.io) is a standard for the collection of metrics, distributed tracing and logs.

[Grafana](https://grafana.com)'s LGTM stack is a bundle of applications (Loki, Grafana, Tempo, Mimir) that collect and visualize OpenTelemetry.

[OpenAPI](https://www.openapis.org) is a widely used API description standard. The endpoints of all microservices on `Microbus` are publicly described with OpenAPI.

[JSON web token (JWT)](https://jwt.io/introduction) is an open standard that defines a compact and self-contained way for securely transmitting information between parties as a JSON object.

[Cascadia](https://github.com/andybalholm/cascadia) implements CSS selectors for use with parsed HTML trees produced by Go's `html` package. Used in unit and integration tests, it facilitates assertions against an HTML document. 

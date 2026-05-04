## Control Core Service

Create a core microservice at hostname `control.core` whose sole purpose is to generate the client API stubs for the `:888` control subscriptions that every Microbus connector implements internally. The `Service` itself does nothing — `OnStartup` always returns `errors.New("unstartable")` — and should never be included in an application.

All control endpoints are implemented inside the Microbus connector (`connector/`) itself, not in this service. This package exists only to give other services a typed, generated client for calling those built-in endpoints.

### Endpoints (all on `:888`, all multicast/no-queue)

- `Ping` — responds with a pong `int`. Used by the metrics service to discover live microservices.
- `ConfigRefresh` — tells the connector to pull fresh config values from the configurator. Called by the configurator when values change.
- `Trace` — accepts a span `id string` and forces the connector to export that tracing span.
- `OpenAPI` on `GET :888/openapi.json` — returns the connector's OpenAPI 3.1 document (`*controlapi.Document`) for this microservice, filtered by the caller's actor claims. Load-balanced (not multicast).

### Web Endpoint

- `Metrics` on `:888/metrics` (multicast/no-queue) — exposes Prometheus metrics collected by the connector. Consumed by the metrics aggregator service.

### Outbound Event

- `OnNewSubs` on `POST :888/on-new-subs` — fired by the connector to notify listeners that new subscriptions have been registered on the bus.

### The `Document` Type

The `controlapi` package defines the `Document` struct used as the OpenAPI response body. This type models the OpenAPI 3.1 document structure returned by each microservice's built-in `:888/openapi.json` handler.

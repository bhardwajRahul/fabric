# Package `coreservices/control`

The `control.core` microservice doesn't run - it's a code-generation artifact. Its role is to publish a typed client (`controlapi`) that other services use to invoke the control-plane endpoints every Microbus connector serves automatically.

## Control-plane endpoints

Every connector exposes a small set of built-in endpoints used for operations and introspection:

| Endpoint | Purpose |
|---|---|
| `Ping` | Discovery and health check |
| `ConfigRefresh` | Pull the latest configuration values from the configurator |
| `Trace` | Force-export a tracing span |
| `Metrics` | Prometheus metrics scrape |
| `OnNewSubs` | Notification of new subscriptions on the bus |
| `OpenAPI` | The microservice's OpenAPI 3.1 document |

These run on the [control plane port](../tech/ports.md) and are reachable from anywhere on the bus, but typically not from the outside.

## Typed client

`controlapi` exposes a client whose methods mirror the endpoints above. To target a specific microservice, use `ForHost`:

```go
doc, _, err := controlapi.NewClient(svc).ForHost("calculator.example").OpenAPI(ctx)
```

To address every microservice at once, target the special `all` hostname through the multicast client:

```go
for r := range controlapi.NewMulticastClient(svc).ForHost("all").Ping(ctx) {
    fromHost := frame.Of(r.HTTPResponse).FromHost()
    fromID := frame.Of(r.HTTPResponse).FromID()
}
```

`ForHost` is required because the default host (`control.core`) doesn't exist as a runnable service.

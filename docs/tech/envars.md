# Environment Variables

The `Microbus` framework uses environment variables for various purposes:

- Initializing the connection to NATS
- Identifying the deployment environment (`PROD`, `LAB`, `LOCAL`, `TESTING`)
- Designating a plane of communication
- Enabling output of debug-level messages
- Configuring the URL to the OpenTelemetry collector endpoint
- Designating a geographic locality

Environment variables may also be set by placing an `env.yaml` or `env.local.yaml` file in the working directory of the executable running the microservice, or any ancestor directory thereof. The bundled example application includes such a file at `main/env.yaml`.

### Transports

A microservice can't communicate with other microservices, including the configurator microservice, before connecting to the transport. [Initializing the NATS connection](../tech/nats-connection.md) therefore relies on environment variables: `MICROBUS_NATS`, `MICROBUS_NATS_USER`, `MICROBUS_NATS_PASSWORD` and `MICROBUS_NATS_TOKEN`.

`MICROBUS_SHORT_CIRCUIT` can be set to `0` or `false` to disable the [short-circuit transport](../tech/short-circuit.md). It is generally recommended to keep the short-circuit transport enabled.

### Deployment

The `MICROBUS_DEPLOYMENT` environment variable determines the [deployment environment](../tech/deployments.md) of the microservice: `PROD`, `LAB`, `LOCAL` or `TESTING`. If not specified, `PROD` is assumed, unless connecting to `nats://localhost:4222` or `nats://127.0.0.1:4222` in which case `LOCAL` is assumed.

### Plane of Communication

The plane of communication is a unique prefix set for all communications sent or received over NATS.
It is used to isolate communication among a group of microservices over a NATS cluster that is shared with other microservices.

If not explicitly set via the `SetPlane` method of the `Connector`, the value is pulled from the `MICROBUS_PLANE` environment variable. The plane must include only alphanumeric characters and is case-sensitive.

Applications created with `application.NewTesting` set a random plane to eliminate the chance of collision when tests are executed in parallel in the same NATS cluster, e.g. using `go test ./... -p=8`.

In most cases there is no need to customize the plane of communications.

### Locality

The `MICROBUS_LOCALITY` environment variable sets the locality of the microservice, which is used as the basis for [locality-aware routing](../blocks/locality-aware-routing.md).

### Logging

Setting the `MICROBUS_LOG_DEBUG` environment variable to `1` or `true` is required for microservices to [log](../blocks/logging.md) debug-level messages.
 
### OpenTelemetry

`Microbus` pushes telemetry to Grafana via its OpenTelemetry collector. The endpoint of the collector is configured using the `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` environment variables.

The `OTEL_METRIC_EXPORT_INTERVAL` variable can be used to set the interval (in milliseconds) between pushes of metrics. It defaults to 15 seconds in `LOCAL` or `TESTING` deployments, and 60 seconds in `PROD` or `LAB` deployments.

Other [OpenTelemetry environment variables](https://opentelemetry.io/docs/languages/sdk-configuration/) are respected.

Set the `MICROBUS_PROMETHEUS_EXPORTER` variable to `1` or `true` to enable Prometheus metric collection in the microservice. This enables the [metrics](../structure/coreservices-metrics.md) core microservice to collect and aggregate metrics from the microservice.

# Package `service`

Package `service` defines the interfaces of a microservice. The [`Connector`](connector.md) implements all of them.

The top-level `Service` interface is a composition of narrower interfaces, each covering a distinct capability: `Publisher` and `Subscriber` for [transport](../blocks/unicast.md), `Logger` for [logging](../blocks/logging.md), `Meter` and `MeterDescriber` for [metrics](../blocks/metrics.md), `Tracer` for [distributed tracing](../blocks/distrib-tracing.md), `StarterStopper` for lifecycle management, `Identifier` for addressing, `Configurable` for [configuration](../blocks/configuration.md), `Resourcer` for [embedded resources](../blocks/embedded-res.md), `Ticker` for [scheduled jobs](../blocks/tickers.md), `Timer` for time management, and `Executor` for launching goroutines.

These fine-grained interfaces allow downstream code to depend on only the capabilities it needs.

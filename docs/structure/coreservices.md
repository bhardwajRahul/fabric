# Package `coreservices`

The `coreservices` package is a collection of microservices that implement common functionality required by most if not all Microbus applications.

- The [access token issuer](../structure/coreservices-accesstoken.md) generates short-lived JWTs signed with ephemeral Ed25519 keys for internal actor propagation
- The [bearer token issuer](../structure/coreservices-bearertoken.md) issues long-lived JWTs signed with Ed25519 keys for external actor authentication
- The [configurator](../structure/coreservices-configurator.md) is responsible for delivering configuration values to microservices that define configuration properties. Such microservices will not start if they cannot reach the configurator
- [Control](../structure/coreservices-control.md) is not actually a microservice but rather a stub microservice used to generate a client for the `:888` [control subscriptions](../tech/control-subs.md)
- The [foreman](../structure/coreservices-foreman.md) orchestrates agentic workflow execution
- The [HTTP egress proxy](../structure/coreservices-httpegress.md) relays HTTP requests to non-Microbus URLs
- The [HTTP ingress proxy](../structure/coreservices-httpingress.md) bridges the gap between HTTP clients and the microservices running on Microbus
- The [LLM](../structure/coreservices-llm.md) microservice bridges LLM tool-calling protocols with Microbus endpoint invocations
- The [metrics](../structure/coreservices-metrics.md) microservice aggregates metrics from all microservices in response to a request from Prometheus
- The [OpenAPI portal](../structure/coreservices-openapiportal.md) microservice renders a catalog of the OpenAPI endpoints of all microservices.
- The [SMTP ingress](../structure/coreservices-smtpingress.md) microservice transforms incoming emails to actionable events

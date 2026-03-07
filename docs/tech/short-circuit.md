# Short-Circuit Transport

The short-circuit is an alternative transport that enables microservices to communicate in-memory rather than over the network. It kicks in only among microservices that are co-located together inside the same executable, reducing the latency of service-to-service calls by a factor of 10. Not all communication can be short-circuited. Communication with remote microservices still occurs over the messaging bus. Multicasts too require using the messaging bus in order for all potential subscribers to be reached.

<img src="./short-circuit-1.drawio.svg">
<p></p>

Tightly-coupled microservices that communicate on a [request/response](../blocks/unicast.md) basis, such as the [HTTP ingress proxy](../structure/coreservices-httpingress.md) and the [access token](../structure/coreservices-accesstoken.md) and [bearer token](../structure/coreservices-bearertoken.md) issuers, should see the most benefit when co-located inside the same executable.

The `MICROBUS_SHORT_CIRCUIT` [environment variable](../tech/envars.md) can be used to disable the short-circuit transport and force all communication to go over the messaging bus.

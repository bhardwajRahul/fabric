# Ports

The concept of a port in `Microbus` can mean one of two things: a real TCP port, or a virtual emulated port.

### Real TCP Ports

`Microbus` uses actual TCP ports for a few use cases.

<img src="./ports-1.drawio.svg"><br>

The [HTTP ingress proxy](../structure/coreservices-httpingress.md) listens for incoming HTTP requests on one or more TCP ports, by default `:8080`. A public-facing HTTP ingress proxy in a production setting will most likely be configured to listen on the standard HTTP ports `:443` and `:80`.

Similarly, the [SMTP ingress proxy](../structure/coreservices-smtpingress.md) listens on port `:25` for incoming SMTP messages.

All microservices connect to the NATS messaging bus, by default on port `:4222`. Microservice exchange messages with other microservice over this bi-directional multiplexed connection.

The UI of [Prometheus](https://prometheus.io) by default runs on port `:9090`, [Grafana](https://grafana.com/)'s on port `:3000` and [Jaeger](https://www.jaegertracing.io)'s on port `:16686`.

### Emulated Ports

Microservices communicate with each other using an [emulation of the HTTP protocol](../blocks/unicast.md) that includes the concept of ports. Real TCP ports are not opened. Rather, the virtual port number is made [part of the bus subject](../blocks/unicast.md#notes-on-subscription-subjects) on which microservices listen for messages.

By convention, some of these internal ports have a special purpose.

Port `:888` is reserved for the [control plane](../tech/control-subs.md).

Endpoints [defined](../tech/service-yaml.md) on port `:443` or `:80` are typically considered public and exposed by the [HTTP ingress proxy](../structure/coreservices-httpingress.md) to external users.

Port `:444` is used by convention for endpoints that should remain internal. Any port not exposed by the ingress proxy would serve the same purpose.

Port `:417` is the default port used for [events](../blocks/events.md).

An endpoint that subscribes on port `:0` receives messages on any port. 

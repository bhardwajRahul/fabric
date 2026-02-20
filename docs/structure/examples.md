# Package `examples`

The `examples` package holds several examples that demonstrate how the `Microbus` framework is used to create microservices. When studying an example, start by looking at `manifest.yaml` to get a quick overview of the functionality of the microservice. Then go deep into the code in `service.go`.

- [HelloWorld](../structure/examples-helloworld.md) demonstrates the classic minimalist example
- [Hello](../structure/examples-hello.md) demonstrates the key capabilities of the framework
- [Calculator](../structure/examples-calculator.md) demonstrates functional handlers
- [Messaging](../structure/examples-messaging.md) demonstrates load-balanced unicast, multicast and direct addressing messaging
- [Event source and sink](../structure/examples-events.md) shows how events can be used to reverse the dependency between two microservices
- [Browser](../structure/examples-browser.md) is an example of a microservice that uses the [HTTP egress core microservice](../structure/coreservices-httpegress.md)
- [Login](../structure/examples-login.md) employs authentication and authorization to restrict access to certain endpoints
- [Yellow Pages](../structure/examples-yellowpages.md) is an example of a SQL CRUD microservice that persists records in a database

In case you missed it, the [quick start guide](../howto/quick-start.md) explains how to setup your system to run the examples.

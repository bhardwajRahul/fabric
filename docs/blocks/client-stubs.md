# Client Stubs

The `GET`, `POST`, `Request` and `Publish` methods of the `Connector` allow an upstream microservice to make requests over the messaging bus. These methods provide HTTP-like semantics: the caller needs to explicitly designate the URL, headers and payload in HTTP form. This approach is broadly applicable but suffers from lack of forward compatibility and type safety. If the downstream microservice changes the signature of its endpoints, the upstream's requests may start failing. It is also somewhat inconvenient to work at the HTTP level, especially when dealing with marshaling.

To address these challenges, the [coding agent](../blocks/coding-agents.md) creates a client stub for each of the endpoints of the downstream microservice. A stub is a type-safe function that wraps the call to the `Connector`'s low-level publishing methods. It is customized for each endpoint based on its type and signature. In the code, the stubs almost appear to be regular function calls.

Client stubs are defined in a sub-package of the microservice named after the package of the service with an `api` suffix added. For example, the clients of the `calculator` microservice are defined in `calculator/calculatorapi`. This naming convention is intended to facilitate type-ahead code completion.

The standard `Client` is used to make [unicast](../blocks/unicast.md) requests and is the more commonly used.

```go
sum, err := calculatorapi.NewClient(svc).Add(ctx, x, y)
```

The aptly-named `MulticastClient` is used to make [multicast](../blocks/multicast.md) requests.

```go
for ri := range providerapi.NewMulticastClient(svc).Discover(ctx) {
	name, err := ri.Get()
	// ...
}
```

The `MulticastTrigger` is to be used by the microservice itself to fire its own events.

```go
for ri := range userstoreapi.NewMulticastTrigger(svc).OnCanDelete(ctx, id) {
	allowed, err := ri.Get()
	// ...
}
```

The `Hook` facilitate registration of event sinks by downstream microservices.

```go
userstoreapi.NewHook(svc).OnCanDelete(svc.OnCanDelete)
```

Client requests can be customized with either `ForHost` or `WithOptions`. The former allows to direct requests at an alternative hostname, while the latter allows fine-grained customization of the request.

```go
sum, err := calculatorapi.NewClient(svc).
	ForHost("my.calculator").
	WithOptions(
		pub.Method("POST"),
		pub.Header("Foo", "Bar"),
	).
	Add(ctx, x, y)
```

The API package also defines the types used by the public endpoints of the microservice. For example, a user store microservice will likely define a `type User struct`. If the public endpoints of a microservice refer to a type owned by another microservice, an alias to it is defined. For example, if a hypothetical registration microservice accepts a `User` object in any of its endpoints, it defines `type User = userstoreapi.User` to alias the original definition.

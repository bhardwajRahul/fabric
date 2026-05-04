# hello.example

## Design Rationale

This microservice is the main showcase example for the Microbus framework. It intentionally collects multiple patterns into a single service rather than separating concerns — the goal is to demonstrate breadth, not model production architecture.

The `Greeting` and `Repeat` configs are read via their typed accessors (`svc.Greeting()`, `svc.Repeat()`), which are generated in `intermediate.go`. These accessors hide the raw string-to-type conversion and should always be preferred over `svc.Config("...")` + manual parsing in the service implementation.

The `Ping` handler multicasts to `https://all:888/ping` using `pub.Multicast()` and reads `frame.Of(res).FromHost()` / `FromID()` to extract service identity from response frames. The `:888` port is the management port — requests to it are not routed through the HTTP ingress proxy and are only accessible internally. `all` is the special hostname that broadcasts to every connected microservice.

The `Calculator` handler deliberately calls `calculatorapi.NewClient(svc).Arithmetic(...)` rather than computing the result itself, to demonstrate a service-to-service call. Arithmetic errors are written into the HTML output rather than returned, since this is a UI handler.

The `Localization` handler uses `svc.LoadResString(ctx, "Hello")` to resolve the locale-appropriate greeting from `resources/text.yaml`. The framework matches the request's `Accept-Language` header against the keys in the YAML map and selects the best-fit translation.

The `Root` endpoint uses route `//root` (absolute path) to map to the ingress root `/` rather than `/hello.example/root`. This is the correct pattern for a landing page that should be reachable at the server root.

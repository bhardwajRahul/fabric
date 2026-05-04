# helloworld.example

## Design Rationale

This microservice is the canonical minimal Microbus example. It exists to show the smallest possible complete microservice: a single GET endpoint that returns a static string.

The `HelloWorld` handler simply writes `"Hello, World!"` to the response and returns. There is no configuration, no downstream calls, no state. This is intentional — the goal is to make the framework's basic structure visible without any application noise.

`OnStartup` and `OnShutdown` are empty, demonstrating that they are optional lifecycle hooks. A real microservice would initialize caches, subscribe to events, or validate config there.

The route `/hello-world` is exposed on the default port `:443` and is reachable externally via the HTTP ingress proxy at `/helloworld.example/hello-world`.

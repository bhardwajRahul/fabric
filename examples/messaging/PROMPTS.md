## Messaging Microservice

Create an example microservice at hostname `messaging.example` that demonstrates Microbus messaging patterns: unicast, direct-addressing unicast, multicast, direct-addressing multicast, and distributed caching.

## Endpoints

Five web handler endpoints:

- `Home` on `GET /home` — the showcase endpoint that exercises all four request patterns in sequence and returns the results as `text/plain`:
  1. Logs which instance processed the current request using `svc.ID()`.
  2. Unicast `GET https://messaging.example/default-queue` via `svc.Request(...)`. Extracts the responder's ID from `frame.Of(res).FromID()`.
  3. Direct-addressing unicast to the same responder: `GET https://<responderID>.messaging.example/default-queue`, bypassing load balancing.
  4. Multicast `GET https://messaging.example/no-queue` via `svc.Publish(...)`, iterating all responses. Captures the last responder's ID.
  5. Direct-addressing multicast to that specific instance only.
  Appends `"Refresh the page to try again"` at the end.

- `NoQueue` on `GET /no-queue`, `loadBalancing: none` — responds with `"NoQueue <instanceID>"`. Because load balancing is disabled, every running instance responds to each multicast request.

- `DefaultQueue` on `GET /default-queue` — responds with `"DefaultQueue <instanceID>"`. Standard load-balanced subscription; only one instance responds per request.

- `CacheLoad` on `GET /cache-load` — reads `?key=` from the query string (returns error if missing), looks up the key in `svc.DistribCache()` using `Get(ctx, key, &value)`, and returns a plain-text report indicating whether the key was found, its value, and which instance served the request.

- `CacheStore` on `GET /cache-store` — reads `?key=` and `?value=` from the query string (returns error if either is missing), stores the value via `svc.DistribCache().Set(ctx, key, []byte(value))`, and returns a plain-text confirmation with the storing instance's ID.

## Non-obvious Details

- `Home` uses the `frame` package to read Microbus-specific metadata from HTTP response frames (`FromID`, `FromHost`). Import `github.com/microbus-io/fabric/frame` and `github.com/microbus-io/fabric/pub`.
- Direct addressing prepends the instance ID and a dot to the hostname: `https://<id>.messaging.example/...`. This routes to the exact replica regardless of load balancing.
- The distributed cache does not require initialization in `OnStartup` for this example (no TTL or memory limits are set).

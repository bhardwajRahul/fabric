## Design Rationale

### Groups *are* the dependency mechanism

Each call to `Add(...)` creates a new group. Within a group, services start in parallel goroutines; between groups, startup is sequential. This is the entire dependency-ordering API — there is no `DependsOn` or topological sort. A service that must come up after another goes in a later `Add` call:

```go
app.Add(httpingress)            // group 0
app.Add(authn, authz)           // group 1: parallel, after group 0
app.Add(myservice)              // group 2: after group 1
```

Shutdown reverses this: groups shut down in reverse order, services within each group in parallel. So a service shuts down strictly before any service it depends on.

### Startup retries until the deadline, not until first error

`group.Startup` does *not* fail fast. If a service's `Startup` returns an error, the goroutine waits one second and retries — until the context deadline expires. The deadline is the only stopping condition.

The retry exists for the case where one service is required to be up before another can start. The classic motivator was an anti-pattern in which microservices declared SQL foreign keys against another service's tables, so service B's startup would fail until service A had created its schema. The framework's preferred design is the opposite: a service should start regardless of its peers and fail at runtime when a missing dependency is actually needed, not at startup. The retry behavior is a safety net for the older patterns and for genuine boot-order races (NATS not yet up, etc.), not a feature to lean on.

`Run` allots 20 seconds for the entire `Startup`. `RunInTest` allots 8 seconds. So an unhealthy service in tests will hold up the whole suite for 8s before failing.

### `RunInTest` overrides plane and deployment *at run time*

`Add` propagates `app.plane` and `app.deployment` (read from `MICROBUS_PLANE` / `MICROBUS_DEPLOYMENT` env vars at `New()` time) onto each added service. `RunInTest` then *overwrites* these on the app and re-walks every group, calling `SetPlane` / `SetDeployment` on every already-added service. Two consequences:

- The plane chosen by `RunInTest` is a fresh 12-char `RandomIdentifier` per call, *not* the SHA-256-of-test-name plane that a bare connector would derive in `Startup`. Every service in one test app shares one plane (so they can talk to each other), and that plane is unique per `RunInTest` call (so concurrent test apps don't collide on shared NATS).
- The deployment is forced to `connector.TESTING`, regardless of any prior setting.

The override happens before `Startup`, so individual `s.SetPlane(...)` calls a test makes between `Add` and `RunInTest` are clobbered. If a test needs a particular plane, set it via `MICROBUS_PLANE` before `New()`.

### Cleanup is registered *before* startup in `RunInTest`

`t.Cleanup(shutdown)` is registered *before* `Startup` is called. If startup fails halfway through (some services started, others didn't), the cleanup still runs and `Shutdown` walks the groups in reverse, shutting down whatever managed to come up. `Run` has no equivalent — a failed `Startup` from `Run` leaves any started services running, and the caller is expected to handle that or just exit the process.

### `RunInTest`'s 8s budget vs `Run`'s 20s

`RunInTest` uses 8-second budgets for both startup and shutdown; `Run` uses 20s. The shorter budget is feasible because tests run on the in-process short-circuit transport (no NATS round-trip on co-located calls) and there are no cold-start dependencies to wait for. The looser 20s in `Run` is sized to absorb the retry-until-deadline behavior in production cold boots.

### Plane / deployment are app-level, hostname is service-level

The `Application` itself carries only `plane` and `deployment` — never hostnames. There is no notion of "the app's services" beyond bookkeeping. The plane/deployment fields exist purely to stamp the same value on every service the app manages, so the resulting microservices form a coherent isolated unit on the bus. Add a service to two apps and it sees the second app's plane/deployment writes (last write wins).

### Stagger on parallel start/shutdown

Each goroutine in a group is launched with an additional 1ms `time.Sleep` (services 0, 1, 2 start at +0ms, +1ms, +2ms, ...). This is a startup-thunder-mitigation hack: tens of services hitting NATS in the same scheduler tick produced bursty connect failures in practice. The stagger spreads the load enough to avoid that without meaningfully delaying total startup time. Same logic in `Shutdown`.

### `Remove` does not shut down

`Remove(svc)` detaches a service from the app's lifecycle bookkeeping but leaves it running on the same plane. Use case is almost exclusively tests that want to transfer lifecycle ownership of a service or detach it from the auto-shutdown chain. Production code rarely needs this.

### Group startup error reporting is lossy

Each group's startup goroutines push into a buffered error channel that the parent drains, but only `lastErr` is returned. If three services in one group fail to start, you see one error, not three. The contract — "if an error is returned, there is no guarantee as to the state of the microservices" — is what makes this acceptable, but if you're debugging a startup failure with multiple suspects, look at the per-service log lines, not the returned error.

### `AddAndStartup` is for late-binding only

`AddAndStartup` is the only way to add services to an already-running application. It creates a new group containing the new services and starts that group immediately. Use it sparingly; the standard pattern is to declare every service in a group up-front via `Add`, then `Run` / `Startup` once.

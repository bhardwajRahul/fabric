## EventSink Microservice

Create an example microservice at hostname `eventsink.example` that demonstrates the inbound event (sink) pattern: subscribing to events fired by `eventsource.example` and maintaining a shared in-memory registration list.

## In-memory State

A package-level `map[string][]string` named `registrations`, keyed by deployment plane (`svc.Plane()`), protected by a package-level `sync.Mutex`. The plane key isolates data between concurrent test runs. No initialization in `OnStartup` is needed (map is initialized at package level).

## Inbound Events

Two inbound event handlers sourced from `github.com/microbus-io/fabric/examples/eventsource`:

- `OnAllowRegister` — signature `OnAllowRegister(email string) (allow bool)`. Blocks registrations matching `@gmail.com`, `.gmail.com`, `@hotmail.com`, or `.hotmail.com` (case-insensitive). Also blocks emails already present in the `registrations` map for the current plane. Uses `mail.ParseAddress` to normalize the email. Returns `false` for any parse error or blocked domain/duplicate; returns `true` otherwise.
- `OnRegistered` — signature `OnRegistered(email string)`. Appends the lowercased email to the `registrations` slice for the current plane under the mutex.

Both handlers log at `Info` level (`svc.LogInfo`) with the `email` attribute.

## Endpoint

One functional endpoint:

- `Registered` on `ANY :443/registered` — returns `emails []string`. Copies the registrations slice for the current plane under the mutex and returns it. Returns an empty slice (not nil) if no registrations exist.

## Non-obvious Details

- The plane-keyed map ensures that parallel test cases using different deployment planes do not interfere with each other. `svc.Plane()` returns the current deployment plane string.
- `mail.ParseAddress` is used for case-insensitive parsing and normalization; if parsing fails the registration is denied.

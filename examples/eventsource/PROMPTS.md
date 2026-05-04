## EventSource Microservice

Create an example microservice at hostname `eventsource.example` that demonstrates the outbound event pattern in Microbus: firing a synchronous gating event and an asynchronous notification event.

## Outbound Events

Two outbound events on port `:417`:

- `OnAllowRegister` on `POST :417/on-allow-register` — signature `OnAllowRegister(email string) (allow bool)`. Fired before registering a user to ask all event sinks whether the registration is allowed. Any sink returning `false` blocks registration.
- `OnRegistered` on `POST :417/on-registered` — signature `OnRegistered(email string)`. Fired after a successful registration to notify all sinks. Fire-and-forget; no return value consumed.

## Endpoint

One functional endpoint:

- `Register` on `ANY :443/register` — accepts `email string`, returns `allowed bool`. Implementation:
  1. Fires `OnAllowRegister` synchronously using `eventsourceapi.NewMulticastTrigger(svc).OnAllowRegister(ctx, email)`, iterating all responses. If any response returns `allow == false`, returns `allowed = false` immediately.
  2. If all sinks allow, proceeds with registration (placeholder comment — no actual persistence).
  3. Fires `OnRegistered` asynchronously in a goroutine using `svc.Go(ctx, func(...))` with `eventsourceapi.NewMulticastTrigger(svc).OnRegistered(ctx, email)` (fire-and-forget, responses not iterated).
  4. Returns `allowed = true`.

## Non-obvious Details

- The synchronous gate uses the multicast trigger's iterator: any `allow == false` from any sink short-circuits. This is the standard Microbus pattern for distributed veto checks.
- The asynchronous notification is launched with `svc.Go` to avoid blocking the caller while sinks process the event.
- The trigger is called without iterating its responses for the fire-and-forget case.

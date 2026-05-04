## Design Rationale

### Frames are a thin façade over `http.Header`

A `Frame` is just a wrapper around an `http.Header`. There is no separate state — every getter and setter reads/writes the underlying headers directly. This is why a frame obtained from a request, response, or context modifies the *same* underlying header map: there is no "snapshot" or "view" behavior.

The wire representation *is* the in-memory representation. Microbus control headers (`Microbus-*`) ride on the same `http.Header` as ordinary HTTP headers, so a frame on an outbound request modifies the same map that's about to be serialized onto the wire.

### Nil frames silently no-op on writes

`Of(x)` returns a frame whose internal `h` is nil when:

- The input is `nil`.
- A `context.Context` is passed but it has no frame attached.
- An unsupported type is passed (the type switch falls through).

A nil frame is "read-only" — getters return empty values, setters silently do nothing. There is no error or panic. `IsNil()` reports the state. This means a typo or wrong-type argument to `Of` produces a working-looking frame that drops writes on the floor.

### Four context variants — pick the right one

There are four ways to attach or copy a frame to a context. The differences are subtle and the wrong choice is a footgun:

- **`ContextWithFrame(parent)`** — idempotent: if `parent` already has a frame, returns it unchanged; otherwise attaches a fresh empty frame. Use when you need to *guarantee* a frame is present without disturbing existing state.

- **`ContextWithFrameOf(parent, x)`** — copies the parent context but attaches `x`'s underlying header *by reference*. Mutations to the returned context's frame flow back to `x`. Currently only the HTTP ingress proxy uses this: it builds an `http.Request` whose context's frame points at the exact same memory as the request's `Header`, so headers and frame don't drift apart. Don't reach for this in normal code.

- **`ContextWithClonedFrameOf(parent, x)`** — copies the parent context and attaches a deep clone of `x`'s header. Mutations are isolated.

- **`CloneContext(parent)`** — convenience for "give me a context with a fresh isolated frame derived from `parent`'s frame (or a new empty one if `parent` has none)."

The default in user code should be `CloneContext` or `ContextWithClonedFrameOf`.

### `SetActor` vs `SetToken` — unsigned vs signed JWT

`SetActor(claims)` constructs an *unsigned* JWT with `alg=none` and the claims as the payload. `SetToken(token)` accepts a pre-signed JWT string from the access-token service. Both write to the same `Microbus-Actor` header.

The split exists because the connector's `verifyToken` accepts `alg=none` only when the deployment is `TESTING`. Production code calls `SetToken` with a real signed token; tests call `SetActor` to skip the signing dance. Outside `TESTING`, an unsigned actor token is rejected as `401 Unauthorized` even though it parses syntactically. The same `Microbus-Actor` wire format covers both cases — what changes is whether the receiver enforces the signature.

### `ParseActor` and `IfActor` ignore non-JWT actors silently

If `Microbus-Actor` is present but doesn't `LooksLikeJWT` (i.e. not three base64url segments separated by dots), it's treated as if no actor were attached. `ParseActor` returns `(false, nil)`; `IfActor` evaluates against an empty claims map. There is no error.

This is the framework's "the actor header is for JWTs only" boundary — anything else is dropped. If you stash a plain string in `Microbus-Actor` expecting `Get(HeaderActor)` to round-trip it, `ParseActor` will not see it.

### `Tenant()` falls back from `tid` to `tenant`

`Tenant()` looks for `tid` first, then `tenant`. The fallback exists for compatibility across identity providers that use one name or the other. If your access-token issuer emits a different field, parse the actor yourself and read the field directly — `Tenant()` is a convenience for the two known conventions, not a general API.

### Wire formats are not all the same

Two duration headers, two different wire formats:

- **`Microbus-Time-Budget`** is stored as a Go duration string (`budget.String()`, e.g. `"5ms"`, `"1h30m"`), parsed via `time.ParseDuration` on read. The accessor still accepts a bare integer for backward compatibility — older frames serialized as a millisecond count round-trip cleanly. Sub-millisecond `SetTimeBudget(0)` (or any non-positive duration) deletes the header. Each hop reads the budget, subtracts elapsed network time, and writes a fresh `String()` form; the parse cost per hop is negligible compared to the cost of the actual network round-trip the budget is gating.
- **`Microbus-Clock-Shift`** is stored as a Go duration string (`shift.String()`, e.g. `"1h30m"`), read with `time.ParseDuration`. Clock shift is rarely modified in flight, so the human-readable form is fine.

### `Fragment()` returns `(1, 1)` on any parse failure

A missing, malformed, or partially-numeric `Microbus-Fragment` header yields `(1, 1)` — i.e. "this is the sole fragment." This defensive default lets the defragger treat any malformed input as a complete message, but it also means the connector can't distinguish "header missing" from "header garbage." A function that returned an error would make this obvious; the current shape silently swallows it. If the distinction matters for your case, read `Get(HeaderFragment)` directly.

### Baggage prefix convention

Baggage entries land in headers named `Microbus-Baggage-<name>`. Because HTTP headers are case-insensitive, baggage names are too — `SetBaggage("UserID", ...)` and `Baggage("userid")` access the same value. This also means baggage names are reserved against the control namespace by prefix: a baggage entry named `Time-Budget` does *not* collide with `Microbus-Time-Budget` because the storage is `Microbus-Baggage-Time-Budget`.

`Publish` in the connector copies all `Microbus-Baggage-*` headers across hops by prefix match — that's how baggage propagates downstream without an explicit allowlist of names.

### Op codes are short wire constants

`OpCodeError` (`Err`), `OpCodeAck` (`Ack`), `OpCodeRequest` (`Req`), `OpCodeResponse` (`Res`) are the four control-message types carried in `Microbus-Op-Code`. They're three-letter strings to keep header overhead low; new op codes should follow the same pattern.

### `XForwardedBaseURL` / `XForwardedFullURL` are not Microbus headers

These methods read standard `X-Forwarded-Proto` / `Host` / `Prefix` / `Path` headers, set by the HTTP ingress proxy. They live on `Frame` purely as a convenience because they're frequently combined with Microbus control header reads on the same request. Don't read this as "Microbus owns the X-Forwarded family" — they're standard HTTP, and the proxy is what populates them.

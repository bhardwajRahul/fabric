## Access Token Core Service

Create a core microservice at hostname `access.token.core` that issues short-lived JWTs signed with ephemeral Ed25519 keys for internal actor propagation.

Each replica generates its own ephemeral key pair in memory at startup — keys are never persisted or shared via distributed cache. This avoids a single point of failure and keeps the threat surface small. Each replica holds at most two keys: `currentKey` and `previousKey`, allowing verification of tokens signed just before a rotation. All key field access is protected by a `sync.RWMutex`.

The `keyPair` struct stores `kid` (a random 16-character identifier), the private and public keys, and `createdAt` time.

### Key Rotation

A `RotateKey` ticker fires every 10 minutes and compares `time.Since(currentKey.createdAt)` against `KeyRotationInterval`. If the interval has elapsed, `generateKey` is called, which promotes the current key to `previousKey` and creates a fresh Ed25519 key pair. This allows dynamic config changes without restart.

### Endpoints

- `Mint` on `:444/mint` — accepts arbitrary claims (typed as `any`), converts them to `jwt.MapClaims` via a JSON round-trip, saves the original `iss` claim as `idp`, runs any registered `ClaimsTransformer` functions in order, then sets non-overridable claims: `idp` (original issuer before transformers), `iss` (`"https://access.token.core"`), `microbus: "1"`, `iat` (5 seconds in the past for clock skew), `exp` (lifetime + 5 seconds grace), and a random `jti`. Signs with EdDSA using the current key, embeds `kid` in the JWT header. Token lifetime is `min(timeBudget, MaxTokenLifetime)`, falling back to `DefaultTokenLifetime` when no time budget is set in the request frame.
- `JWKS` on `:888/jwks` — multicasts to `LocalKeys` on all peers and aggregates the results into a single `[]JWK` slice.
- `LocalKeys` on `:444/local-keys` (no-queue multicast, so every replica responds) — returns this replica's current and previous public keys as `[]JWK` in standard format (`kty: OKP`, `crv: Ed25519`, `alg: EdDSA`, `use: sig`, `x: base64url`).

The `JWK` type lives in `accesstokenapi/jwks.go`.

### Extension Point

Expose a public `AddClaimsTransformer(ClaimsTransformer)` method on the `Service` struct. `ClaimsTransformer` is `func(ctx context.Context, claims jwt.MapClaims) error`. Transformers must be registered before startup (`IsStarted()` guard). They are called in registration order during `Mint`, after the JSON round-trip and before the non-overridable claims are set.

### Config Properties

- `KeyRotationInterval` — duration between key rotations, default `6h`, minimum `2h`
- `DefaultTokenLifetime` — fallback token lifetime when no request time budget exists, default `20s`, range `[1s, 15m]`
- `MaxTokenLifetime` — hard cap on token lifetime regardless of request budget, default `15m`, range `[1s, 15m]`

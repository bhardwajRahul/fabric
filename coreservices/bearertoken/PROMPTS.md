## Bearer Token Core Service

Create a core microservice at hostname `bearer.token.core` that issues long-lived JWTs signed with Ed25519 keys for external actor authentication.

The service holds up to two key entries (primary and alternative) to support graceful key rotation. Each key entry stores the parsed Ed25519 private/public key pair and a `kid` derived from the first 8 bytes of the SHA-256 hash of the public key. Keys are loaded from `PrivateKey` and `AltPrivateKey` secret config properties, which accept either full PEM format or raw base64-encoded DER. Both configs have `OnChanged` callbacks that reload the respective key entry without restart. In `LOCAL` and `TESTING` deployments, if no primary key is configured, an ephemeral key is auto-generated at startup.

Expose two endpoints:

- `Mint` on `:444/mint` — accepts arbitrary claims (`any`), converts them to `jwt.MapClaims` via a JSON round-trip, runs any registered `ClaimsTransformer` functions in order, then sets the following non-overridable claims: `iss` (the service hostname as an HTTPS URL), `microbus: "1"`, `iat` (5 minutes in the past to handle clock skew), `exp` (TTL + 5 minutes of grace), and a random `jti`. Signs with EdDSA using the primary key and embeds `kid` in the JWT header. Returns `503` if no primary key is configured.
- `JWKS` on `:888/jwks` — returns the public keys for both primary and alternative entries (whichever are non-nil) as a `[]JWK` slice in standard JWKS format (`kty: OKP`, `crv: Ed25519`, `alg: EdDSA`, `use: sig`).

Expose a public `AddClaimsTransformer(ClaimsTransformer)` method on the `Service` struct that allows callers to register claim mutation hooks before the service starts. Return an error if called after startup.

Config properties: `AuthTokenTTL` (duration, default `720h`, minimum `1m`), `PrivateKey` (secret, callback), `AltPrivateKey` (secret, callback).

Use a `sync.RWMutex` to protect the two key entry fields against concurrent reads during `Mint`/`JWKS` and writes during key reload.

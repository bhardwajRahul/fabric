## Design Rationale

The access token service exists to replace plain JSON claims in the `Microbus-Actor` header with signed JWTs. Each replica generates its own ephemeral Ed25519 key pair in memory - keys are never persisted or shared via distributed cache. This avoids a single point of failure and keeps the threat surface small.

Key rotation happens on a configurable interval (default 6h, minimum 2h). The ticker fires every 10 minutes and checks elapsed time against the configured interval, allowing dynamic config changes without restart. Each replica keeps at most two keys (current + previous) to allow verification of tokens signed just before rotation.

The JWKS endpoint multicasts to `LocalKeys` on all peers to aggregate public keys. Downstream connectors cache these keys and refresh on unknown `kid`. The `LocalKeys` endpoint uses `sub.NoQueue()` so every replica responds to the multicast.

Token expiration is tied to the request's time budget, reinforcing the maximum transaction lifetime set at the HTTP ingress.

### Mint is a trust root

`Mint` runs on `:666` and signs arbitrary caller-supplied claims by design (only the critical claims `iss`, `idp`,
`iat`, `exp`, `jti` are stamped), which is what enables claims enrichment and impersonation. It therefore cannot be
secured with `requiredClaims` or an in-handler authorization check - that is circular, since minting is how a caller
obtains a verified claim in the first place. The only technical control is the `:666` `PUB` ACL; isolation and a CI
allow-list (in the production deployment guide's trust-root hardening section) are the operational controls.

### Newly rotated keys are not used for signing immediately

`generateKey` promotes a new key to `currentKey`, but `Mint` keeps signing with `previousKey` until the current key
has existed for `keyActivationDelay` (10s). The new key is published in JWKS (via `LocalKeys`) from the moment it is
generated, so verifiers can fetch it in advance of the first token it signs. This is what makes the JWKS fetch
debounce in the connector and the ingress safe: a verifier always has `keyActivationDelay` to pick up a new `kid`
before any token carries it, far longer than the 1s fetch cooldown those verifiers apply. At startup there is no
previous key, so the first key signs immediately.

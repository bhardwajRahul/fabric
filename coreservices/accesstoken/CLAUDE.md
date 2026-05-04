## Design Rationale

The access token service exists to replace plain JSON claims in the `Microbus-Actor` header with signed JWTs. Each replica generates its own ephemeral Ed25519 key pair in memory - keys are never persisted or shared via distributed cache. This avoids a single point of failure and keeps the threat surface small.

Key rotation happens on a configurable interval (default 6h, minimum 2h). The ticker fires every 10 minutes and checks elapsed time against the configured interval, allowing dynamic config changes without restart. Each replica keeps at most two keys (current + previous) to allow verification of tokens signed just before rotation.

The JWKS endpoint multicasts to `LocalKeys` on all peers to aggregate public keys. Downstream connectors cache these keys and refresh on unknown `kid`. The `LocalKeys` endpoint uses `sub.NoQueue()` so every replica responds to the multicast.

Token expiration is tied to the request's time budget, reinforcing the maximum transaction lifetime set at the HTTP ingress.

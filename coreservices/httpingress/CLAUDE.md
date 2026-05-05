# http.ingress.core

## Agent Instructions

This microservice uses authentication APIs. See `.claude/rules/auth.txt` for the conventions.

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Design Rationale

### `resolveInternalURL` lowercases but does not validate

The hostname segment of an external URL is lowercased before publishing because Microbus identities are canonical lowercase, but external URLs may arrive with uppercase characters (browsers don't always preserve case in path segments). Without normalization, requests for `/MyService/path` would fail to route despite a perfectly valid `myservice` subscription on the bus.

The hostname is not otherwise validated here. A malformed or unknown hostname surfaces as a no-responder ack-timeout (404) from `Publish`, which is the same shape as any other unresolved internal target. Validating up front would shift the error to a different layer for marginal benefit - the ingress is gated by network position, not by hostname legality.

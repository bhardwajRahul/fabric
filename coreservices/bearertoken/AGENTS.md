**CRITICAL**: This directory contains the codebase of a microservice based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`.

**CRITICAL**: The instructions and guidelines in this `AGENTS.md` file only apply when working on the microservice in this directory and take precedence over the more general instructions and guidelines of the project.

## Design Rationale

BearerToken is the external token issuer that signs long-lived JWTs for end users using Ed25519 (EdDSA).
It is separate from the access token service (`accesstoken`) which uses ephemeral in-memory keys.
The external issuer uses PEM-configured private keys that are manually rotated, with an alternative key
for graceful rotation.

Key design decisions:
- Ed25519 private keys are configured via `PrivateKey` and `AltPrivateKey` configs (PEM or raw base64 format, secret)
- The JWKS endpoint exposes corresponding public keys for external verification
- Token TTL is configurable (default 720h) for long-lived end-user tokens
- Two keys can be active simultaneously during rotation: current + alternative
- The `kid` header in issued JWTs identifies which key was used for signing

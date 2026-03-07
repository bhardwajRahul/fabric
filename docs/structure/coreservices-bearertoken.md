# Package `coreservices/bearertoken`

The bearer token core microservice issues long-lived [JWTs](https://jwt.io/introduction) signed with Ed25519 keys for external actor authentication. These tokens are typically returned to end users after successful authentication and presented on subsequent requests via the `Authorization: Bearer` header or an `Authorization` cookie.

### Token Minting

The `Mint` endpoint signs a JWT with a given set of claims. The token's lifetime is controlled by the `AuthTokenTTL` configuration property (default 720h / 30 days).

The minted token includes several critical claims that are set automatically and cannot be overridden:

| Claim | Description |
|---|---|
| `iss` | Set to `microbus://bearer.token.core`, identifying the bearer token service as the issuer |
| `iat` | Issued-at timestamp, backdated 5 minutes to account for clock skew |
| `exp` | Expiration timestamp, with 5 minutes of grace for clock skew |
| `jti` | A unique token identifier for replay protection |

### Claims Transformation

Claims transformers can be registered to enrich the token with additional claims before signing. Transformers are called in the order they were added and mutate the claims map in place. They cannot override the critical claims listed above, which are set after transformation.

```go
bearertoken.NewService().Init(func(svc *bearertoken.Service) error {
    err := svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
        claims["welcome"] = true
        return nil
    })
    return err
})
```

Transformers must be registered during initialization, before the service starts.

### Key Management

Unlike the [access token](../structure/coreservices-accesstoken.md) service which uses ephemeral in-memory keys, the bearer token service uses PEM-configured Ed25519 private keys. Two keys can be active simultaneously — `PrivateKeyPEM` and `AltPrivateKeyPEM` — to enable graceful key rotation. Tokens are always signed with the primary key; the alternative key is only used to continue verifying tokens signed before rotation.

In `LOCAL` and `TESTING` deployments, a key is auto-generated if none is configured. In `LAB` and `PROD` deployments, a key must be explicitly configured.

The `JWKS` endpoint exposes the corresponding public keys in standard [JWKS](https://datatracker.ietf.org/doc/html/rfc7517) format, enabling external systems to verify tokens issued by this service.

### Token Exchange

On each incoming request, the [HTTP ingress proxy](../structure/coreservices-httpingress.md) looks for a bearer token, verifies its signature against the issuer's JWKS endpoint, and exchanges it for a short-lived [access token](../structure/coreservices-accesstoken.md). The original `iss` of the bearer token is preserved in the access token's `idp` claim.

### Configuration

| Property | Default | Description |
|---|---|---|
| `AuthTokenTTL` | `720h` | TTL of issued JWTs (minimum 1m) |
| `PrivateKeyPEM` | | Ed25519 private key in PEM format (secret) |
| `AltPrivateKeyPEM` | | Alternative Ed25519 private key for rotation (secret) |

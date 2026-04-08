# Package `coreservices/accesstoken`

The access token core microservice generates short-lived [JWTs](https://jwt.io/introduction) signed with ephemeral Ed25519 keys for internal actor propagation. On each incoming request, the [HTTP ingress proxy](../structure/coreservices-httpingress.md) exchanges the external bearer token for an internal access token. The access token's claims serve as the basis for [authorization](../blocks/authorization.md) decisions throughout the call stack.

### Token Minting

The `Mint` endpoint signs a JWT with a given set of claims. The token's lifetime is derived from the request's [time budget](../blocks/time-budget.md), falling back to `DefaultTokenLifetime` (default 20s) if no budget is set, and capped at `MaxTokenLifetime` (default 15m).

The minted token includes several critical claims that are set automatically and cannot be overridden:

| Claim | Description |
|---|---|
| `iss` | Set to `microbus://access.token.core`, identifying the access token service as the issuer |
| `idp` | Preserves the original `iss` of the input claims, identifying the identity provider (typically the bearer token service) |
| `iat` | Issued-at timestamp, backdated 5 seconds to account for clock skew |
| `exp` | Expiration timestamp, with 5 seconds of grace for clock skew |
| `jti` | A unique token identifier for replay protection |

### Claims Transformation

Claims transformers can be registered to enrich the token with dynamic claims before signing. Transformers are called in the order they were added and mutate the claims map in place. They see the original input claims but cannot override the critical claims listed above, which are set after transformation.

```go
accesstoken.NewService().Init(func(svc *accesstoken.Service) error {
    err := svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) (err error) {
        userID := int(claims["userid"].(float64))
        user, err := usersapi.NewClient(svc).Load(ctx, userID)
        if err != nil {
            return errors.Trace(err)
        }
        claims["roles"] = user.Roles
        claims["groups"] = user.Groups
        return nil
    })
    return err
})
```

Transformers must be registered during initialization, before the service starts. See [enabling auth](../howto/enabling-auth.md) for the full setup guide.

### Key Management

Each replica generates its own ephemeral Ed25519 key pair in memory on startup. Keys are never persisted or shared. Key rotation happens on a configurable interval (default 6h, minimum 2h). Each replica keeps at most two keys (current + previous) to allow verification of tokens signed just before rotation.

The `JWKS` endpoint aggregates public keys from all replicas by multicasting to `LocalKeys`, returning them in standard [JWKS](https://datatracker.ietf.org/doc/html/rfc7517) format. Downstream connectors cache these keys and refresh on unknown `kid`.

### Configuration

| Property | Default | Description |
|---|---|---|
| `KeyRotationInterval` | `6h` | Duration between Ed25519 key rotations (minimum 2h) |
| `DefaultTokenLifetime` | `20s` | Token lifetime when no time budget is present |
| `MaxTokenLifetime` | `15m` | Maximum token lifetime regardless of time budget |

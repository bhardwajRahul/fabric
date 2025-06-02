# Package `coreservices/tokenissuer`

The token issuer core microservice issues and validates tokens in the form of [JWTs](https://jwt.io/introduction). JWTs enable the authentication of actors and the authorization of their requests based on a set of claims.

The `IssueToken` endpoint creates a JWT with a set of claims and signs it using the `HMAC-SHA512` algorithm with a configurable `SecretKey`. The `roles` and `groups` claims are [commonly used](https://www.iana.org/assignments/jwt/jwt.xhtml) but the JWT's schema is flexible and claims may be of any valid JSON type.

```json
{
    "name":     "Harry Potter",
    "roles":    "student wizard",
    "groups":   ["Gryffindor"],
    "whatever": "anything goes here",
}
```

JWTs created by the token issuer core microservice include a `validator` claim with the hostname `tokenissuer.core` to inform the authorization [middleware](../structure/coreservices-httpingress-middleware.md) where to validate the token.

The `ValidateToken` endpoint checks a JWT for validity and returns the actor associated with it. To be considered valid, the JWT's `iss` claim must match, it must not have expired, and its signature must match either `SecretKey` or `AltSecretKey`.

The token issuer only serves to manage JWTs. A different microservice, such as the [login example](../structure/examples-login.md) microservice, is responsible for authentication and the association of the JWT with the user.

You may need to implement a [custom token issuer](../howto/enabling-auth.md#step-2-token-issuer-and-validator) if for example you'd like it to support the revocation of tokens or use a different signature method.

# Enabling Authentication

[Authentication and authorization](../blocks/authorization.md) in a Microbus application comprises of several elements.

### Actor

The `Actor` struct is a convenience pattern for representing the claims associated with an inbound request. It is created during [project setup](../howto/init-project.md) in the `act` package and can be extended to fit the needs of the solution. The `Of` function extracts the actor from the request context by parsing the access token's claims.

The properties of the `Actor` are organized into four groups:

- **Standard claims** — Claims defined by the JWT specification, such as `iss` (issuer), `sub` (subject), and `exp` (expiration). The `idp` claim is set by the access token service to record the original identity provider.
- **Identifiers** — Application-specific identifiers that uniquely identify the actor, such as user ID and tenant ID. These are typically constant for the life of the actor.
- **Security claims** — Claims used as the basis for authorization, such as roles, groups, or scopes. These are typically assigned by an administrator and can change over the life of the actor.
- **User preferences** — Claims under the control of the actor, such as time zone, locale, or name.

JSON tag names should follow the [IANA JWT Claims Registry](https://www.iana.org/assignments/jwt/jwt.xhtml#claims) where a registered claim name exists (e.g. `sub`, `iss`, `zoneinfo`, `given_name`, `locale`).

### Token Issuers

Authentication in Microbus uses two token issuer microservices that work in tandem:

- The [bearer token](../structure/coreservices-bearertoken.md) microservice issues long-lived JWTs signed with Ed25519 keys for external actor authentication. These are the tokens returned to end users after login.
- The [access token](../structure/coreservices-accesstoken.md) microservice generates short-lived JWTs signed with ephemeral Ed25519 keys for internal actor propagation. On each incoming request, the HTTP ingress proxy exchanges the bearer token for an access token.

Both token issuers must be included in the main app.

```go
app.Add(
	bearertoken.NewService(),
	accesstoken.NewService(),
)
```

The bearer token microservice signs JWTs using Ed25519 private keys configured via `PrivateKeyPEM`. In `LOCAL` and `TESTING` deployments, a key is auto-generated if none is configured. In `LAB` and `PROD` deployments, a key must be generated and configured in `config.local.yaml`.

```shell
openssl genpkey -algorithm Ed25519 -out private.pem
```

```yaml
bearer.token.core:
  PrivateKeyPEM: |
    -----BEGIN PRIVATE KEY-----
    MC4CAQAwBQYDK2VwBCIEIL...
    -----END PRIVATE KEY-----
```

The access token microservice uses ephemeral in-memory keys that are auto-generated and rotated automatically. No configuration is required.

### Authenticator

An authenticator is a microservice that validates credentials, issues a bearer token JWT, and returns it to the user. Microbus does not include a standard authenticator — each solution creates its own to match its authentication requirements.

The `Mint` endpoint of the bearer token service generates a token from a set of claims. Any claims set in the bearer token are exposed to the end user and cannot be changed without issuing another token. For this reason, it is recommended to include only static claims such as identifiers in the bearer token, and to use a [claims transformer](#claims-transformer) on the access token service to enrich it with sensitive or dynamic claims.

```go
// Create a bearer token following successful authentication
bearertokenapi.NewClient(svc).Mint(ctx, map[string]any{
	"sub": "subject@example.com",
	"uid": 12345,
	"tid": 123,
})
```

The [login example](../structure/examples-login.md) microservice demonstrates an authenticator that accepts a username and password via a web form and returns the JWT in a `Set-Cookie` header. Single-page applications may be better served by a functional endpoint that returns the JWT in JSON form, such as `Authenticate(username string, password string) (signedToken string, httpStatusCode int)`.

### Claims Transformer

A claims transformer is a callback that enriches the access token with dynamic claims during minting — such as security claims or user preferences loaded from a database. The keys of the claims should match the JSON tag names of the `Actor`.

```go
app.Add(
	accesstoken.NewService().Init(func(svc *accesstoken.Service) error {
		err := svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
			userID := int(claims["uid"].(float64))
			tenantID := int(claims["tid"].(float64))
			user, err := usersapi.NewClient(svc).Load(ctx, tenantID, userID)
			if err != nil {
				return errors.Trace(err)
			}
			claims["sub"] = user.Email
			claims["groups"] = user.Groups
			claims["roles"] = user.Roles
			claims["scope"] = user.Scope
			claims["zoneinfo"] = user.TimeZone.String()
			claims["locale"] = user.Locale
			claims["given_name"] = user.GivenName
			claims["family_name"] = user.FamilyName
			return nil
		})
		return err
	}),
)
```

### Middleware

The authorization [middleware](../structure/coreservices-httpingress-middleware.md) of the HTTP ingress proxy looks for a bearer token JWT in the `Authorization: Bearer` header or in a cookie named `Authorization`. It verifies the token's signature against the issuer's JWKS endpoint and exchanges it for a short-lived access token via the [access token](../structure/coreservices-accesstoken.md) service. The access token's claims are then propagated downstream as the actor of the request throughout the call stack.

A custom middleware can be configured when initializing the [HTTP ingress proxy](../structure/coreservices-httpingress.md) to look for the token in different request headers.

### Authorization Requirements

Endpoints can declare required claims as a boolean expression that must be satisfied by the actor's claims for the request to be allowed. Required claims are stipulated using the `sub.RequiredClaims` option of the endpoint's subscription.

> HEY CLAUDE...
>
> Create a functional endpoint `SalesReport(from time.Time, to time.Time) (sales SalesData)` requiring claims `group.sales && (roles.director || roles.manager)`.

When an endpoint's behavior varies depending on the actor, the actor can be obtained from the context and inspected.

```go
func (svc *Service) SalesReport(ctx context.Context, from time.Time, to time.Time) (sales *SalesData, err error) {
	actor, err := act.Of(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if actor.IsDirector() {
		// ...
	} else {
		// ...
	}
	return sales, nil
}
```

To call a downstream microservice with modified claims, mint a new access token with the adjusted claims and attach it via `pub.Token`.

```go
func (svc *Service) BypassRestriction(ctx context.Context) (err error) {
	actor, err := act.Of(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	actor.Roles = append(actor.Roles, "admin") // Elevate the actor
	elevatedToken, err := accesstokenapi.NewClient(svc).Mint(ctx, actor)
	if err != nil {
		return errors.Trace(err)
	}
	err = downstreamapi.NewClient(svc).
		WithOptions(pub.Token(elevatedToken)). // Use the elevated actor
		ActionForAdminsOnly(ctx)
	return errors.Trace(err)
}
```

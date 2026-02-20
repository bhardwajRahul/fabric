# Enabling Authentication

Enabling [authentication and authorization](../blocks/authorization.md) in a `Microbus` application requires an initial setup. Thereafter though, restricting endpoints to only authorized actors is typically a one-liner declaration.

#### Step 1: Actor

An actor represents the user context of a request. Start by creating an `Actor` struct that defines the actor in your application.

```go
package act

import "github.com/microbus-io/fabric/frame"

type Actor struct {
	// Identifiers
	Issuer   string   `json:"iss"`
	Subject  string   `json:"sub"`
	TenantID int      `json:"tenantID"`
	UserID   int      `json:"userID"`
	// Security claims
	Groups   []string `json:"groups"`
	Roles    []string `json:"roles"`
	Scopes   []string `json:"scopes"`
	// User preferences
	TimeZone string   `json:"timezone"`
	Locale   string   `json:"locale"`
	Name     string   `json:"name"`
}

func Of(r any) *Actor {
	var a Actor
	frame.Of(r).ParseActor(&a)
	return &a
}
```

The `Actor` should contain identifiers that uniquely identify the actor in your application, such as user ID, tenant ID, etc. Identifiers are likely constant for the life of the actor.

The `Actor` should also include the security claims to be used as basis for authorization, such as roles or group associations. Security claims are typically assigned to an actor by an administrator and can change over the life of the actor.

The `Actor` may also include preferences that are under the control of the actor, such as time zone, locale, name or email.

#### Step 2: Token Issuer

To be authenticated, requests must include an auth token in the form of a [JWT](https://jwt.io/introduction). On an incoming request, the JWT is validated and converted to an actor that is propagated downstream along the call stack. The [token issuer](../structure/coreservices-tokenissuer.md) core microservice is used to issue and validate JWTs.

Be sure to include the token issuer in the main app.

```go
app.Add(
	tokenissuer.NewService(),
)
```

The token issuer signs JWTs using a 512-bit (64 byte) secret key. Configure your secret in `config.yaml`.

```yaml
tokenissuer.core:
  SecretKey: \K"37WM%h6dj\V£)swfZdebf6b1zXUhd8[`iy>~7[L5BQp3/>z++91s{(PG)6=z/
```

If the core microservice is insufficient to your needs, you may implement your own token issuer that provides the same interface.

#### Step 3: Authenticator

Create a microservice that authenticates a user given their credentials, issues a JWT, and returns it back to them.

Use the `IssueToken` endpoint of the token issuer to generate a token from a set of claims. Be advised that any claims you set in the token are exposed to the end user and cannot be changed without issuing another token. For this reason, it is recommended to include in the JWT only static claims such as identifiers, and to later use a [claims transformer](#step-4-claims-transformer) to enrich it with sensitive or dynamic claims.

```go
// Create a token following successful authentication
tokenissuerapi.NewClient(svc).IssueToken(ctx, jwt.MapClaims{
	"sub":      "subject@example.com",
	"userid":   12345,
	"tenantid": 123,
})
```

The [login example](../structure/examples-login.md) microservice is an example of an authenticator that accepts a username and a password via a web form and returns the JWT via a `Set-Cookie` header. Single-page applications may be better served by a functional endpoint that returns the JWT to the client in JSON form, such as `Authenticate(username string, password string) (signedToken string)`.

#### Step 4: Claims Transformer

Set a claims transformer callback in order to enrich the token with dynamic claims, such as security claims or user preferences. For example, you may want to load certain user settings from a database and include them in the claims. The keys of the claims should match the JSON tag names of the `Actor`.

```go
app.Add(
	tokenissuer.NewService().Init(func(svc *tokenissuer.Service) {
		svc.SetClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) (transformedClaims jwt.MapClaims, err error) {
			// Extend the claims here...

			userID := int(claims["userid"].(float64))
			tenantID := int(claims["tenantid"].(float64))
			user, err := usersapi.NewClient(svc).Load(ctx, tenantID, userID)
			if err != nil {
				return errors.Trace(err)
			}
			claims["sub"] = user.Email
			claims["groups"] = user.Groups
			claims["roles"] = user.Roles
			claims["scopes"] = user.Scopes
			claims["timezone"] = user.TimeZone.String()
			claims["locale"] = user.Locale
			claims["name"] = user.FullName()

			return claims, nil
		})
	}),
)
```

#### Step 5: Middleware

The authorization [middleware](../structure/coreservices-httpingress-middleware.md) looks for a JWT in the `Authorization: Bearer` header or in a cookie named `Authorization`. It contacts the token issuer microservice named in the `iss` claim to validate the token and obtain the claims associated with it. The claims are then propagated downstream to the target microservice and the rest of the call stack thereafter. 

If you want to look for the token in different request headers, you may set a custom middleware when initializing the [HTTP ingress proxy](../structure/coreservices-httpingress.md).

#### Step 6: Authorization Requirements

Instruct the coding agent when creating an endpoint that it requires the actor to provide claims in order to be authorized. Required claims are stipulated using the `sub.RequiredClaim` option of the subscription of the endpoint.

> HEY CLAUDE...
>
> Create a functional endpoint `SalesReport(from time.Time, to time.Time) (sales SalesData)` requiring claims `group.sales && (roles.director || roles.manager)`.

If an endpoint's output is conditional upon the actor, it may obtain the actor from the context via the [frame](../structure/frame.md) and adjust accordingly.

```go
func (svc *Service) SalesReport(ctx context.Context, from time.Time, to time.Time) (sales *SalesData, err error) {
	actor := act.Of(ctx)
	if actor.IsDirector() {
		// ...
	} else {
		// ...
	}
	return sales, nil
}
```

Use the `pub.Actor` publishing option to modify the actor when calling a downstream microservice:

```go
func (svc *Service) BypassRestriction(ctx context.Context) (err error) {
	actor := act.Of(ctx)
	actor.Roles = append(actor.Roles, "admin") // Elevate the actor
	err = downstreamapi.NewClient(svc).
		WithOptions(pub.Actor(actor)). // Use the elevated actor
		ActionForAdminsOnly(ctx)
	return errors.Trace(err)
}
```

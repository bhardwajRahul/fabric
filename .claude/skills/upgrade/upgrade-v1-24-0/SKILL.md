---
name: upgrade-v1-24-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project to v1.24.0.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.24.0:
- [ ] Step 1: Prepare actor
- [ ] Step 2: Token issuers
- [ ] Step 3: Generate bearer token key
- [ ] Step 4: Tracing spans
- [ ] Step 5: Token exchange callback
- [ ] Step 6: Actor impersonation
- [ ] Step 7: Manifests
- [ ] Step 8: Configuration Files
```

#### Step 1: Prepare Actor

Create the `act` directory in the root of the project if one does not exist.

```shell
mkdir -p act
```

Create `act/actor.go` with the content of the template `actor.go` located in the directory of this skill. If the file already exists, do not overwrite it. The existing `Actor` struct may define different claims from the template - that is expected and the struct should not be changed.

If the function `Of` in `act/actor.go` does not return an error, i.e. its signature is `Of(x any) Actor`, extend it to return an error and correct resulting compilation errors. When fixing callers, return the error if the containing function can return an error; otherwise, discard it with `_`.

```go
// Of extracts the actor from the HTTP request or context.
func Of(x any) (act Actor, err error) {
	_, err = frame.Of(x).ParseActor(&act)
	return act, errors.Trace(err)
}
```

#### Step 2: Token Issuers

In the core microservices block in `main/main.go`, replace the deprecated `tokenissuer` microservice with `bearertoken` and `accesstoken` instead.

```go
app.Add(
	// Core microservices
	// ...
	bearertoken.NewService().Init(func(svc *bearertoken.Service) (err error) {
		svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
			// HINT: Enrich the claims of the external bearer token here
			return nil
		})
		return nil
	}),
	accesstoken.NewService().Init(func(svc *accesstoken.Service) (err error) {
		svc.AddClaimsTransformer(func(ctx context.Context, claims jwt.MapClaims) error {
			// HINT: Enrich the claims of the internal access token here
			return nil
		})
		return nil
	}),
)
```

If the deprecated `tokenissuer` included a claims transformer, copy its logic to the `accesstoken` claims transformer. Note that claims are now transformed in-place and there is no need to return them as a result of the transformation.

Add the corresponding imports.

```go
import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/fabric/coreservices/accesstoken"
	"github.com/microbus-io/fabric/coreservices/bearertoken"
)
```

#### Step 3: Generate Bearer Token Key

Generate an Ed25519 key and set it in `config.local.yaml` for the `PrivateKey` config of the `bearer.token.core` microservice.

If the deprecated `tokenissuer.core` had an `AuthTokenTTL` value, copy it to the `TokenTTL` config of `bearer.token.core`. The old `SecretKey` can be discarded - the bearer token service uses a different signing algorithm for which the upgrade generates a new key.

```shell
openssl genpkey -algorithm Ed25519 -out private.pem
```

```yaml
bearer.token.core:
  TokenTTL: 720h
  PrivateKey: MC4CAQAwBQYDK2VwBCIEILioh4C097ydAtppNWBMxO1hkewbzzmbGs1z7n9+OHnp
```

#### Step 4: Tracing Spans

The `Log` method of `trc.Span` was replaced with `LogDebug`, `LogInfo`, `LogWarn` and `LogError`.

Change all `span.Log(severity, ...)` to the corresponding `span.LogDebug(...)`, `span.LogInfo(...)`, `span.LogWarn(...)` or `span.LogError(...)`. If `severity` does not match any of these, use `span.LogInfo(...)`.

#### Step 5: Token Exchange Callback

The signature of the validator function passed to the `Authorization` middleware changed from `tokenValidator func(ctx context.Context, token string) (actor any, valid bool, err error)` to `func(ctx context.Context, bearerToken string) (accessToken string, err error)`. Adjust appropriately if the project is setting up a custom `Authorization` middleware, typically in `main/main.go` during the initialization of the HTTP ingress proxy core microservice.

#### Step 6: Actor Impersonation

`pub.Actor` and `frame.SetActor` were changed to only work during testing and should only be used in `*_test.go` files. In production code, an access token must be minted and set via `pub.Token` or `frame.SetToken` instead.

```go
token, err := accesstokenapi.NewClient(svc).Mint(ctx, actor)
```

#### Step 7: Manifests

Update the frameworkVersion in all manifest files to `1.24.0`.

#### Step 8: Configuration Files

Search for `tokenissuer.core` across all YAML files in the project and remove any references to it.

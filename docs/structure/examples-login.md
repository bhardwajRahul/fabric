# Package `examples/login`

The `login.example` microservice demonstrates the use of [authentication and authorization](../blocks/authorization.md) in `Microbus`.

The `Welcome` endpoint is defined in `service.yaml` to require an actor containing either `a` (admin), `m` (manager) or `u` (user) in its `roles` property. Requests to http://localhost:8080/login.example/welcome that do not satisfy this condition will be met with HTTP error `401 Unauthorized` or `403 Forbidden`.

```yaml
functions:
  - signature: Welcome()
    description: |-
      Welcome renders a page that is shown to the user after a successful login.
      Rendering is adjusted based on the user's roles.
    actor: roles.a || roles.m || roles.u
```

An `ErrorPageRedirect` middleware is added to the [HTTP ingress proxy](../structure/coreservices-httpingress.md) in the `main` app to redirect users to the login page upon a `401 Unauthorized` error.

```go
httpingress.NewService().Init(func(svc *httpingress.Service) {
	svc.Middleware().Append("LoginExample401Redirect",
		middleware.OnRoute(
			func(path string) bool {
				return strings.HasPrefix(path, "/"+login.Hostname+"/")
			},
			middleware.ErrorPageRedirect(http.StatusUnauthorized, "/"+login.Hostname+"/login"),
		),
	)
}),
```

The `Login` endpoint renders a simple HTML login form. Upon a successful login, it calls the [token issuer](../structure/coreservices-tokenissuer.md) core microservice to generate a JWT with the appropriate claims, places it in a `Set-Cookie` header, and redirects the user to the welcome page.

The `Authorization` middleware of the [HTTP ingress proxy](../structure/coreservices-httpingress.md) detects the token in the `Cookie` header and, after validating it with the [token issuer](../structure/coreservices-tokenissuer.md) core microservice, sets its claims as the actor of the request.

The `Welcome` endpoint is able to parse the actor of the request into an `Actor` object knowing that it's been validated on ingress. The `welcome.html` template adjusts the content of the welcome page in accordance with the role of the actor.

```go
var actor Actor
ok, err := frame.Of(r).ParseActor(&actor)
```

The `Logout` endpoint clears the cookie and redirects the user to the login screen.

The `AdminOnly` and `ManagerOnly` endpoint, as their name suggests, are restricted to a single role.

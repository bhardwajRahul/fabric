# Authorization

Multiple components work together to form the authentication and authorization flows in `Microbus`.

<img src="./authorization-1.drawio.svg">

### Flow 1: Authentication

To get started, the client submits credentials to an authenticator microservice. In the simplest form, credentials are a username and a password obtained from the user via a web form. If the credentials are valid, a [JWT](https://jwt.io/introduction) token is issued and returned back to the client.

`Microbus` does not include authenticators out of the box. The [login example](../structure/examples-login.md) microservice demonstrates how such a microservice may look like when the client is a browser. A `Set-Cookie` response header returns the JWT to the browser, which in turn sends it back with all consecutive requests in the `Cookie` request header. The cookie is named `Authorization` to allow the authorization [middleware](../structure/coreservices-httpingress-middleware.md) to easily locate it.

The login example microservice utilizes the [token issuer](../structure/coreservices-tokenissuer.md) core microservice to issue and validate JWTs. It is possible to replace the token issuer core microservice with a [custom token validator](../howto/enabling-auth.md#step-2-token-issuer-and-validator).

### Flow 2: Authorization

Now that the client is in possession of a JWT, it's expected to be included in the header of consecutive requests. The authorization [middleware](../structure/coreservices-httpingress-middleware.md) examines the HTTP request headers for a JWT in the `Authorization: Bearer` header or in a `Cookie` named `Authorization`. It identifies the appropriate token issuer microservice by examining the `validator` claim and validates the token with the issuer to get the actor associated with it. The actor is then propagated downstream to the target microservice and the rest of the call stack thereafter.

```http
Cookie: Authorization=<JWT>
Authorization: Bearer <JWT>
```

The [connector](../structure/connector.md) of each microservice allows a request to continue if the actor associated with the request satisfies a set of requirements expressed as a boolean expression over the properties of the actor. Requirements are specified during the definition of the endpoint, most commonly via [`service.yaml`](../tech/service-yaml.md), or using the `sub.Actor` option when creating [subscriptions](../structure/sub.md) manually.

For example, the `SalesReport` endpoint below is restricted to sales directors or managers only. Requests from actors that do not satisfy these requirements will be denied with a `401 Unauthorized` or `403 Forbidden` error.

```yaml
functions:
  - signature: SalesReport(from time.Time, to time.Time) (sales SalesData)
    actor: groups.sales && (roles.director || roles.manager)
```

### Flow 3: Redirect

By default, `401 Unauthorized` and `403 Forbidden` errors are returned to the client in the form of an HTTP status code accompanied by an error message. The error page redirect [middleware](../structure/coreservices-httpingress-middleware.md) can improve user experience by redirecting to a more user-friendly page. Redirection is contingent upon the `Sec-Fetch-Mode` and `Sec-Fetch-Dest` request headers indicating that the user is using a browser to navigate to a new document, thus avoiding interfering with requests from single-page applications.

```http
Sec-Fetch-Mode: navigate
Sec-Fetch-Dest: document
```

A `401 Unauthorized` error indicates that the user is not logged in. The following redirects to a login page where the user can begin the authentication flow.

```go
httpIngressProxy := httpingress.NewService()
httpIngressProxy.Middleware().Append("401Redirect", middleware.ErrorPageRedirect(http.StatusUnauthorized, "/login"))
```

A `403 Forbidden` error indicates the user is attempting to access a page or API they are not authorized for. The following redirects to a dedicated error page.

```go
httpIngressProxy.Middleware().Append("403Redirect", middleware.ErrorPageRedirect(http.StatusForbidden, "/access-denied"))
```

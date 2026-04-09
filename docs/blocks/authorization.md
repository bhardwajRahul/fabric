# Authorization

Multiple components work together to form the authentication and authorization flows in Microbus. A two-token architecture separates long-lived bearer tokens from short-lived access tokens, providing a clear boundary between external identity and internal authorization.

<img src="./authorization-1.drawio.svg">
<p></p>

### Flow 1: Authentication

To get started, the client submits credentials to an authenticator microservice. In the simplest form, credentials are a username and a password obtained from the user via a web form. If the credentials are valid, a bearer token is issued in the form of a [JWT](https://jwt.io/introduction) and returned back to the client.

Microbus does not include authenticators out of the box. The [login example](../structure/examples-login.md) microservice demonstrates how such a microservice may look like when the client is a browser. A `Set-Cookie` response header returns the bearer token to the browser, which in turn sends it back with all consecutive requests in a `Cookie` request header. The cookie is named `Authorization` to allow the authorization [middleware](../structure/coreservices-httpingress-middleware.md) to easily locate it.

Bearer tokens are long-lived JWTs signed by an external identity provider or by the [bearer token](../structure/coreservices-bearertoken.md) core microservice. The bearer token microservice uses Ed25519 key pairs and exposes a JWKS endpoint that publishes its public keys, allowing any party to verify the token's signature without sharing a secret.

### Flow 2: Token Exchange

Now that the client is in possession of a bearer token, it's expected to be included in the header of consecutive requests. The authorization [middleware](../structure/coreservices-httpingress-middleware.md) examines the HTTP request headers for a bearer token in the `Authorization: Bearer` header or in a `Cookie` named `Authorization`.

```http
Cookie: Authorization=<JWT>
Authorization: Bearer <JWT>
```

When a bearer token is found, the middleware exchanges it for a short-lived access token in a multi-step process:

1. The bearer token is parsed (without verification) to extract the `iss` claim and the `kid` header
2. The `iss` claim identifies the token issuer microservice. The middleware fetches the issuer's public keys from its JWKS endpoint and caches them by `kid`
3. The bearer token's signature is verified against the matching public key
4. The verified claims are forwarded to the [access token](../structure/coreservices-accesstoken.md) core microservice, which mints a short-lived access token. Any registered claims transformers are applied to enrich the claims (e.g. adding roles, tenant ID, or permissions from a user database) before the access token is signed
5. The access token preserves the original identity provider in the `idp` claim and instead sets the `iss` claim to `https://access.token.core`
6. The signed access token is attached to the request and propagated automatically to downstream microservices

Access tokens use ephemeral Ed25519 key pairs that rotate automatically. Like the bearer token service, the access token microservice exposes a JWKS endpoint that aggregates public keys from all replicas, enabling any microservice to verify access tokens independently.

### Flow 3: Authorization

The [connector](../structure/connector.md) of each microservice evaluates the claims in the access token to determine whether the request is allowed to proceed. Requirements are expressed as a boolean expression and specified during the definition of the endpoint, using the `sub.RequiredClaims` option when creating the [subscription](../structure/sub.md).

For example, the option `sub.RequiredClaims("groups.sales && (roles.director || roles.manager)")` indicates that the endpoint is restricted to directors or managers only from the sales group. Requests by actors that do not satisfy these requirements will be denied with a `401 Unauthorized` or `403 Forbidden` error.

### Flow 4: Redirect

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

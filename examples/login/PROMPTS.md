## Login Microservice

Create an example microservice at hostname `login.example` that demonstrates JWT-based authentication and role-based authorization using Microbus's bearer token core service.

## Actor Type

Define an `Actor` struct in `actor.go` (not `service.go`) with fields `Subject string` (JSON tag `sub`) and `Roles []string` (JSON tag `roles`). Add convenience methods: `IsAdmin() bool`, `IsManager() bool`, `IsUser() bool` (each calling `HasRole`), `SetAdmin()`, `SetManager()`, `SetUser()` (each appending the role letter), `HasRole(role string) bool` (using `slices.Contains`), and `String() string` (JSON-encodes to string via `bytes.Buffer`).

Role letters: `"a"` = admin, `"m"` = manager, `"u"` = standard user.

## Hardcoded Users

Three hardcoded users, all with password `"password"`:
- `admin@example.com` — roles `["a"]` (admin only, not a standard user)
- `manager@example.com` — roles `["m", "u"]` (manager is also a standard user)
- `user@example.com` — roles `["u"]`

## Endpoints

Five web handler endpoints:

- `Login` on `ANY /login` — renders `resources/login.html` (template with fields `U`, `P`, `Src`, `Denied`). On POST with valid credentials, calls `bearertokenapi.NewClient(svc).Mint(ctx, claims)` with `sub` and `roles` claims, sets the JWT as an `HttpOnly` cookie named `"Authorization"` (expiry derived from the JWT's `exp` claim via `jwt.Parse`), then redirects. If a `?src=` param was provided and does not contain `"://"`, redirects to `src`; otherwise redirects to the externalized `Welcome` URL via `svc.ExternalizeURL(ctx, loginapi.Welcome.URL())`. On failed login, re-renders the form with `Denied: true`.

- `Logout` on `ANY /logout` — clears the `Authorization` cookie by setting `MaxAge: -1`, then redirects to the externalized `Login` URL.

- `Welcome` on `ANY /welcome`, `requiredClaims: roles.a || roles.m || roles.u` — parses the actor from the request frame using `frame.Of(r).ParseActor(&actor)`, then renders `resources/welcome.html` with the `Actor` struct and the raw actor header (`r.Header.Get(frame.HeaderActor)`). Accessible to any authenticated user.

- `AdminOnly` on `GET /admin-only`, `requiredClaims: roles.a` — renders `resources/admin-only.html`. Accessible only to admins.

- `ManagerOnly` on `GET /manager-only`, `requiredClaims: roles.m` — renders `resources/manager-only.html`. Accessible only to managers.

## Downstream Dependency

Imports `bearertokenapi` for `Mint`. The `bearer.token.core` service must be present in the app.

## Resources

HTML templates in `resources/`: `login.html`, `welcome.html`, `admin-only.html`, `manager-only.html`.

## Non-obvious Details

- Import `github.com/golang-jwt/jwt/v5` to parse the minted JWT and extract the `exp` claim for computing the cookie's `MaxAge`.
- The cookie `Secure` flag is set only if `r.TLS != nil`.
- The cookie `Path` is always `"/"` so it applies to the entire site.
- `frame.HeaderActor` is the header name where Microbus places the decoded actor claims.

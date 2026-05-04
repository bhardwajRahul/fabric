## Browser Microservice

Create an example microservice at hostname `browser.example` that implements a minimal web browser UI demonstrating outbound HTTP requests via the Microbus HTTP egress proxy.

## Endpoint

One web handler endpoint:

- `Browse` on `ANY /browse` — renders an HTML page with an address bar input and a "View Source" button. Reads `?url=` from the query string. If the URL does not contain `"://"`, prepends `"https://"`. When a URL is present, calls `httpegressapi.NewClient(svc).Get(ctx, u)` to fetch the page through the egress proxy, reads the response body with `io.ReadAll`, HTML-escapes it, and wraps it in a `<pre style="white-space:pre-wrap">` block. The entire page is built with `strings.Builder` and returned as `text/html`.

## Downstream Dependency

Imports `httpegressapi` for `Get`. The `http.egress.core` service must be present in the app.

## Non-obvious Details

- All outbound HTTP requests must go through the HTTP egress proxy (`httpegressapi.NewClient(svc).Get`) rather than the standard `http.Client`. This keeps egress traffic observable and testable.
- HTML-escape the fetched content with `html.EscapeString` before writing it into the page to prevent XSS from remote content.

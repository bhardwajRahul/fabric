## HTTP Egress Core Service

Create a core microservice at hostname `http.egress.core` that proxies outbound HTTP requests from the bus to the internet.

Expose one endpoint:

- `MakeRequest` on `POST :444/make-request` — receives a raw HTTP request serialized in binary RFC 7231 wire format in the POST body, executes it against the target URL, decompresses the response, and writes the decompressed response (headers + body) back to the caller.

### MakeRequest Logic

1. Read the proxied request from `r.Body` using `http.ReadRequest(bufio.NewReaderSize(r.Body, 64))`.
2. If the URL has no explicit port, append `:443` for `https` or `:80` for `http`.
3. Clear `req.RequestURI` (required by Go's HTTP client).
4. Set `Accept-Encoding: br;q=1.0,deflate;q=0.8,gzip;q=0.6` to accept all common compression formats.
5. Create a child OpenTelemetry span named after `req.URL.Hostname()`. In `LOCAL` deployment, attach the full request to the span for debugging; in other deployments, only add request attributes on error.
6. Execute the request with a plain `http.Client{}`.
7. Decompress the response body if the `Content-Encoding` header indicates `br` (brotli, via `github.com/andybalholm/brotli`), `deflate`, or `gzip`. After decompression, remove the `Content-Encoding` header and set `Content-Length` to the decompressed size.
8. Copy all response headers and the body to `w`. Write the response status code.
9. On any error, attach the request to the span, record the error, and call `svc.ForceTrace`.

The service has no config properties and no startup/shutdown logic. It is a pure proxy.

### Usage Pattern

Callers use the generated `httpegressapi.NewClient(svc).Do(ctx, req)` helper, which serializes the `*http.Request` into wire format and posts it to `:444/make-request`. Only use this service for outbound internet requests — Microbus-internal calls should use the bus directly.

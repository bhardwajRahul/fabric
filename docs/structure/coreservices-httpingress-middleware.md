# Middleware

A middleware is a function that returns a function that can be added to the [HTTP ingress proxy](../structure/coreservices-httpingress.md) to pre or post process a request.

```go
type Middleware func(next connector.HTTPHandler) connector.HTTPHandler
```

Remember that `Microbus` add an error value to the standard Go web handler.

### Chain

Middlewares are chained together. Each receives the request after it was processed by the preceding (upstream) middleware, passing it along to the `next` (downstream) middleware. And conversely, each receives the response from the next (downstream) middleware, and passes it back to the preceding (upstream) middleware. Both the request and the response may be modified by the middleware.

The HTTP ingress core microservice keeps the chain of middleware in a `middleware.Chain` construct that can be accessed via its `Middleware` method.
Each middleware in the chain is addressable by its name and can be replaced, removed or used as an insertion point. The chain is initialized with reasonable defaults: `CharsetUTF8` -> `ErrorPrinter` -> `BlockedPaths` -> `Logger` -> `Enter` -> `SecureRedirect` -> `CORS` -> `XForward` -> `InternalHeaders` -> `RootPath` -> `Timeout` -> `Authorization` -> `Ready` -> `CacheControl` -> `Compress` -> `DefaultFavIcon`.

`Enter` is a noop marker that indicates that the request was accepted. Middleware after this point typically manipulate the request headers. Similarly, `Ready` is a noop marker that indicates that the request is ready to be processed. Middleware after this point typically manipulate the response headers or body. Both markers can be used as insertion points.

### In the Box

The following middlewares are available in the `middleware` package:

- `Authorization` looks for a token in the "Authorization: Bearer" header or the "Authorization" cookie,
validates it with its issuer, and associates the corresponding actor with the request
- `BlockedPaths` filters incoming requests based on their URL path. The default setting blocks common patterns that are obviously trying to probe the server for vulnerabilities. Supported block patterns: `/exact/path`, `/path/subtree/*`, `*.ext` 
- `CacheControl` sets the Cache-Control header if not otherwise specified
- `CharsetUTF8` appends `; charset=utf-8` to the `Content-Type` header of textual responses
- `Compress` compresses textual responses using brotli, gzip or deflate
- `CORS` responds to the CORS origin OPTION request and blocks requests from disallowed origins
- `DefaultFavIcon` responds to `/favicon.ico`, if no microservice does
- `ErrorPageRedirect` that redirects full-page browser requests that resulted in an error to an error page
- `ErrorPrinter` is the final catcher of errors. It converts the errors to the appropriate HTTP status code, typically `500 Internal Server Error`, and prints an error message to the user
- `Group` chains nested middleware together and is often used in conjunction with the `OnRoute` middleware to apply a group of middleware to a specific route
- `InternalHeaders` filters internal headers from entering or exiting.
- `Logger` logs the incoming requests and error responses.
- `Noop` does nothing
- `OnRoute` applies middleware conditionally based on the path of the URL
- `RootPath` rewrites the root path `/` with one that can be routed to such as `/root`
- `SecureRedirect` redirects request from HTTP port `:80` to HTTPS port `:443`, if appropriate
- `Timeout` applies a timeout to the request
- `XForwarded` sets the `X-Forwarded` headers pertaining to the request, if not already set. These headers are used by downstream microservices to compose absolute URLs when necessary

### Build

To modify the request, a middleware should manipulate the `http.Request` before delegating it to the `next` (downstream) middleware.

```go
func DisableCompression() Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			r.Header.Del("Accept-Encoding")
			return next(w, r)
		}
	}
}
```

Similarly, to modify the header or status code of the response, a middleware should manipulate the `http.ResponseWriter` after delegating the request to the `next` (downstream) middleware.

```go
func RemoveSecretHeader(secretHeaderName string) Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			err = next(w, r)
			r.Header.Del(secretHeaderName)
			return err // No trace
		}
	}
}
```

A middleware that wants to manipulate the body of the response should create a local `http.ResponseWriter` and delegate that downstream instead of the one it received. `httpx.NewResponseRecorder` works in conjunction with `httpx.Copy` to reduce memory allocations.

```go
func Zipper() Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			ww := httpx.NewResponseRecorder()
			err = next(ww, r)
			if err != nil {
				return err // No trace
			}
			res := ww.Result()
			body := res.Body
			isCompress := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && res.Header.Get("Content-Encoding") == ""
			if isCompress {
				res.Body = nil
			}
			err = httpx.Copy(w, res)
			if err != nil {
				return errors.Trace(err)
			}
			if isCompress {
				w.Header().Set("Content-Encoding", "gzip")
				zipper := gzip.NewWriter(w)
				err = io.Copy(zipper, body)
				zipper.Close()
				if err != nil {
					return errors.Trace(err)
				}
			}
			return nil
		}
	}
}
```

Custom middleware is inserted to the chain during initialization of the ingress proxy. The following inserts the `DisableCompression` middleware after the `Enter` marker middleware and sets it to operate on the `/documents/` path.

```go
httpIngress := httpingress.NewService()
httpIngress.Middleware().InsertAfter(httpingress.Enter, middleware.OnRoutePrefix("/documents/", DisableCompression()))  
```

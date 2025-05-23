# Package `coreservices/httpingress`

Think of `Microbus` as a closed garden that requires a special key to access. In order to send and receive messages on `Microbus`, it's necessary to communicate over NATS using a specific protocol. This is basically what the `Connector` facilitates for service-to-service calls.

Practically all solutions require interaction with a source that is outside `Microbus`. The most common scenario is perhaps a request generated from a web browser to a public API endpoint. In this case, something needs to bridge the gap between the incoming real HTTP request and the HTTP messages that travel over `Microbus`. This is where the HTTP ingress proxy comes into play.

<img src="./coreservices-httpingress-1.drawio.svg">
<p></p>

On one end, the HTTP ingress proxy listens on port `:8080` for real HTTP requests; on the other end it is connected to NATS. The ingress proxy converts real requests into requests on the bus; and on the flip side, converts responses from the bus to real responses. Because the bus messages in `Microbus` are formatted themselves as HTTP messages, this conversion is trivial, with minor adjustments:

* The proxy filters out `Microbus-` control headers from coming in or leaking out
* The first segment of the path of the real HTTP request is treated as the hostname of the microservice on the bus. So for example, `POST` request `http://localhost:8080/echo.example/echo` is translated to a bus `POST` request `https://echo.example/echo` which is then mapped to the NATS subject `microbus.443.example.echo.|.POST.echo`.
* Port `:443` is assumed by default when a port is not explicitly specified. Internal ports can be designated in the first segment of the path. For example, `http://localhost:8080/echo.example:1234/echo` is mapped to the bus address `https://echo.example:1234/echo`.
* The empty root path is transformed to `/root`, therefore `http://localhost:8080/` is mapped to `https://root`.

### Configuration

The HTTP ingress proxy supports several configuration properties that can be set in in `config.yaml`:

```yaml
http.ingress.core:
  Ports: 9090
```

`Ports` is a comma-separated list of real HTTP ports on which to listen for requests. The default is to listen on port `:8080`.

`PortMappings` is a comma-separated list of mappings in the form `x:y->z` where `x` is the inbound
HTTP port, `y` is the requested internal port, and `z` is the internal port to serve.
Put differently, an HTTP request `https://ingresshost:x/servicehost:y/path` is mapped to internal NATS
request `https://servicehost:z/path`.
Both `x` and `y` can be `*` to indicate all ports. Setting `z` to `*` indicates to serve the requested
port `y` without change. More specific rules take precedence over `*` rules.

Ports can be used to differentiate between traffic that is coming from trusted and untrusted sources. For example, the default setting `8080:*->*, 443:*->443, 80:*->443` grants port `:8080` access to all internal ports, while ports `:443` and `:80` are restricted to internal port `:443`. The idea is to expose ports `:443` and `:80` to the internet and restrict `:8080` to trusted clients only.

<img src="./coreservices-httpingress-3.drawio.svg">
<p></p>

Four config properties are used to safeguard against long requests:

* `ReadHeaderTimeout` is the timeout to read the request's header
* `ReadTimeout` is the timeout to read the full request, including the header
* `TimeBudget` is the time budget allocated to the downstream microservice to process the request
* `WriteTimeout` is the timeout to write the response back to the client

<img src="./coreservices-httpingress-2.drawio.svg">
<p></p>

`RequestMemoryLimit` is the memory capacity used to hold pending requests, in megabytes.

`AllowedOrigins` is a comma-separated list of CORS origins to allow requests from. The `*` origin can be used to allow CORS request from all origins.

### Respected Headers

The HTTP ingress proxy respects the following incoming headers:

* `Request-Timeout` can be used to override the default time-budget of the request
* `Accept-Encoding` with `br`, `deflate` or `gzip` can be used to compress the response
* `X-Forwarded-Host`, `X-Forwarded-Port`, `X-Forwarded-Proto` and `X-Forwarded-Prefix` are augmented with the ingress proxy's information 
* `Origin` may cause a request to be blocked

### Middleware

Middleware is a function that returns a function that can be added to pre- or post-process a request.

```go
type Middleware func(next connector.HTTPHandler) connector.HTTPHandler
```

Middlewares are chained together. Each receives the request after it was processed by the preceding (upstream) middleware, passing it along to the `next` (downstream) one. And conversely, each receives the response from the next (downstream) middleware, and passes it back to the preceding (upstream) middleware. Both request and response may be modified by the middleware.

The HTTP ingress core microservice keeps the chain of middleware in a `middleware.Chain` construct that can be accessed via its `Middleware` method.
Each middleware in the chain is addressable by name and can be replaced, removed or used as an insertion point.

The chain is initialized with reasonable defaults that perform various functions:
`ErrorPrinter` -> `BlockedPaths` -> `Logger` -> `Enter` -> `SecureRedirect` -> `CORS` -> `XForwarded` -> `InternalHeaders` -> `RootPath` -> `Timeout` -> `Authorization` -> `Ready` -> `CacheControl` -> `Compress` -> `DefaultFavIcon`

The `Enter` middleware is a noop marker that indicates that the request was accepted. Middleware after this point typically manipulate the request headers.
The `Ready` middleware is a noop marker that indicates that the request is ready to be processed. Middleware after this point typically manipulate the response headers or body.

A somewhat contrived example that inserts a middleware named `Contrived` after the `Enter` marker to process requests whose path starts with `/images/`.

```go
httpIngress := NewService()
httpIngress.Middleware().InsertAfter("Enter", "Contrived", middleware.OnRoutePrefix("/images/", func(next connector.HTTPHandler) connector.HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		// Modifying the request should be done before calling next
		r.Header.Del("Accept-Encoding") // Disable compression
		
		// Delegate the request downstream
		ww := httpx.NewResponseRecorder()
		err = next(ww, r)
		if err != nil {
			w.Header().Add("X-Check", "Error")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Oops!"))
			return nil
		}

		// Copy the result from downstream into the original writer
		err = httpx.Copy(w, ww.Result())
		w.Header().Add("X-Check", "OK")
		return errors.Trace(err)
	}
}))
```

Note that the `w` passed to the middleware is an `httpx.ResponseRecorder` whose headers and status code can be modified even after the body had been written. Appending to the body is also allowed. Modifying the body requires casting in order to clear it first.

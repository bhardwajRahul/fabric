# Package `frame`

The `frame` package enables access to the `Microbus-` control headers that are added to the [emulated HTTP](../blocks/unicast.md) messages that travel between microservices.

### Obtaining the Frame

In web endpoints, the frame is obtained `Of` the `*http.Request`.

```go
func (svc *Service) WebEndpoint(w http.ResponseWriter, r *http.Request) (err error) {
	f := frame.Of(r)
	// ...
}
```

Functional endpoints do not have an HTTP request argument, so `Microbus` places the HTTP requests headers in a context's value instead. The same pattern is used to obtain the frame `Of` the `context.Context`

```go
func (svc *Service) FunctionEndpoint(ctx context.Context, x int, y int) (ok bool, err error) {
	f := frame.Of(ctx)
	// ...
}
```

Accessing the frame of an HTTP response is not as common, but can be useful in some situations.

```go
func (svc *Service) FunctionEndpoint(ctx context.Context, x int, y int) (ok bool, err error) {
	ch := controlapi.NewMulticastClient(svc).ForHost("all").Ping(ctx)
	for r := range ch {
		f := frame.Of(r.HTTPResponse)
		// ...
	}
	// ...
}
```

### Accessing the Frame

The frame provides getters that facilitate reading the `http.Header` in a type-safe manner.

```go
func (svc *Service) WebEndpoint(w http.ResponseWriter, r *http.Request) (err error) {
	var actor Actor
	frame.Of(r).ParseActor(&actor)
	baseURL := frame.Of(r).XForwardedBaseURL()
	// ...
}
```

Writing to a frame modifies the `http.Header` held internally by the context. Cloning the context avoids this side effect.

```go
func (svc *Service) FunctionEndpoint(ctx context.Context, x int, y int) (z int, err error) {
	// Call downstream using the original context
	x, err := downstreamapi.NewClient(svc).Standard(ctx)
	// Elevate the context to call a restricted downstream API
	elevatedCtx := frame.CloneContext(ctx)
	elevatedCtx.SetActor(Actor{
		SuperUser: true,
	})
	y, err := downstreamapi.NewClient(svc).Restricted(elevatedCtx)
	// ...
}
```

An empty frame can be added to a standard `context.Context` by means of `frame.ContextWithFrame` and thereafter manipulated with the frame's setter methods.

```go
func TestMyService_Restricted(t *testing.T) {
	elevatedCtx := frame.ContextWithFrame(context.Background())
	elevatedCtx.SetActor(Actor{
		SuperUser: true,
	})
	y, err := myserviceapi.NewClient(svc).Restricted(elevatedCtx)
	if err != nil || y != 12345 {
		t.Fail()
	}
}
```

### Examples

The actor associated with the request is the basis for [authorization](../blocks/authorization.md).

```go
func (svc *Service) WebEndpoint(w http.ResponseWriter, r *http.Request) (err error) {
	// Obtain the actor of the request
	var actor Actor
	frame.Of(r).ParseActor(&actor)
	userName := actor.FullName

	// Impersonate another user when calling downstream
	impersonatedCtx := frame.CloneContext(r.Context())
	impersonatedCtx.SetActor(Actor{
		Subject: "someone_else@example.com",
		Name:    "Someone Else",
		Roles:   "manager",
	})
	err = downstreamapi.NewClient(svc).OnBehalfOf(impersonatedCtx)
	// ...
}
```

The external URLs that originated the request are set by the [HTTP ingress proxy](../structure/coreservices-httpingress.md) in the `X-Forwarded` headers. They can be used to produce fully-qualified URLs that can be returned back to the user.

```go
func (svc *Service) WebEndpoint(w http.ResponseWriter, r *http.Request) (err error) {
	// Obtain the base or full URL of the request
	baseURL := frame.Of(r).XForwardedBaseURL()
	fullURL := frame.Of(r).XForwardedFullURL()
	linkAbsolute := baseURL + "/absolute"
	linkSister := url.JoinPath(fullURL, "../sister")
	// ...
}
```

[Shifting the clock](../blocks/integration-testing.md#shifting-the-clock) allows testing aspects of the application that have a temporal dimension.

```go
func TestMyService_ClockShift(t *testing.T) {
	ctx := frame.ContextWithFrame(context.Background())
	resultNow, err := Svc.TimeSensitiveOperation(ctx)
	// Shift clock 1 hour forward
	frame.Of(ctx).IncrementClockShift(time.Hour)
	resultOneHourLater, err := Svc.TimeSensitiveOperation(ctx)
	// ...
}
```

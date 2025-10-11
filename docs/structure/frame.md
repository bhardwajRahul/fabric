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

Writing to a frame modifies the `http.Header` held internally by the context and should be done with care, if at all.
Cloning the context avoids modifying the original context's `http.Header`.

```go
func (svc *Service) FunctionEndpoint(ctx context.Context, x int, y int) (z int, err error) {
	// Call downstream using the original context
	key, err := downstreamapi.NewClient(svc).GetKey(ctx)
	// ...
	// Clone the incoming context and attach baggage to it
	clonedCtx := frame.CloneContext(ctx)
	clonedCtx.SetBaggage("key", key)
	y, err := downstreamapi.NewClient(svc).ProcessJob(clonedCtx)
	// ...
}
```

Cloning also adds a frame to a standard `context.Context`.

```go
func TestMyService_FunctionEndpoint(t *testing.T) {
	ctx := frame.CloneContext(t.Context())
	clonedCtx.SetBaggage("key", "1234567890")
	y, err := downstreamapi.NewClient(svc).ProcessJob(clonedCtx)
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

	// Impersonate another user using WithOptions (recommended)
	impersonatedActor := Actor{
		Subject: "someone_else@example.com",
		Name:    "Someone Else",
		Roles:   "manager",
	}
	err = downstreamapi.NewClient(svc).
		WithOptions(pub.Actor(impersonatedActor)).
		OnBehalfOf(ctx)
	// ...

	// Impersonate another user using a cloned context
	clonedCtx := frame.CloneContext(r.Context())
	clonedCtx.SetActor(impersonatedActor)
	err = downstreamapi.NewClient(svc).OnBehalfOf(clonedCtx)
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
	ctx := frame.CloneContext(t.Context())
	resultNow, err := Svc.TimeSensitiveOperation(ctx)
	// Shift clock 1 hour forward
	frame.Of(ctx).IncrementClockShift(time.Hour)
	resultOneHourLater, err := Svc.TimeSensitiveOperation(ctx)
	// ...
}
```

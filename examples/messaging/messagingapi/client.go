package messagingapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
)

var (
	_ context.Context
	_ json.Encoder
	_ *http.Request
	_ *errors.TracedError
	_ *httpx.BodyReader
)

// Hostname is the default hostname of the microservice.
const Hostname = "messaging.example"

// Endpoint routes.
const (
	RouteOfHome         = `/home`          // MARKER: Home
	RouteOfNoQueue      = `/no-queue`      // MARKER: NoQueue
	RouteOfDefaultQueue = `/default-queue`  // MARKER: DefaultQueue
	RouteOfCacheLoad    = `/cache-load`     // MARKER: CacheLoad
	RouteOfCacheStore   = `/cache-store`    // MARKER: CacheStore
)

// Endpoint URLs.
var (
	URLOfHome         = httpx.JoinHostAndPath(Hostname, RouteOfHome)         // MARKER: Home
	URLOfNoQueue      = httpx.JoinHostAndPath(Hostname, RouteOfNoQueue)      // MARKER: NoQueue
	URLOfDefaultQueue = httpx.JoinHostAndPath(Hostname, RouteOfDefaultQueue) // MARKER: DefaultQueue
	URLOfCacheLoad    = httpx.JoinHostAndPath(Hostname, RouteOfCacheLoad)    // MARKER: CacheLoad
	URLOfCacheStore   = httpx.JoinHostAndPath(Hostname, RouteOfCacheStore)   // MARKER: CacheStore
)

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c Client) ForHost(host string) Client {
	return Client{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c Client) WithOptions(opts ...pub.Option) Client {
	return Client{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{
		svc:  caller,
		host: Hostname,
	}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{
		svc:  _c.svc,
		host: _c.host,
		opts: append(_c.opts, opts...),
	}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{
		svc:  listener,
		host: Hostname,
	}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{
		svc:  c.svc,
		host: host,
		opts: c.opts,
	}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{
		svc:  c.svc,
		host: c.host,
		opts: append(c.opts, opts...),
	}
}

/*
Home demonstrates making requests using multicast and unicast request/response patterns.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) Home(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: Home
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfHome)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
Home demonstrates making requests using multicast and unicast request/response patterns.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) Home(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: Home
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfHome)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
NoQueue demonstrates how the NoQueue subscription option is used to create
a multicast request/response communication pattern.
All instances of this microservice will respond to each request.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) NoQueue(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: NoQueue
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfNoQueue)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
NoQueue demonstrates how the NoQueue subscription option is used to create
a multicast request/response communication pattern.
All instances of this microservice will respond to each request.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) NoQueue(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: NoQueue
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfNoQueue)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
a unicast request/response communication pattern.
Only one of the instances of this microservice will respond to each request.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) DefaultQueue(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: DefaultQueue
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfDefaultQueue)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
a unicast request/response communication pattern.
Only one of the instances of this microservice will respond to each request.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) DefaultQueue(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: DefaultQueue
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfDefaultQueue)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
CacheLoad looks up an element in the distributed cache of the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) CacheLoad(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: CacheLoad
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfCacheLoad)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
CacheLoad looks up an element in the distributed cache of the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) CacheLoad(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: CacheLoad
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfCacheLoad)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
CacheStore stores an element in the distributed cache of the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) CacheStore(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: CacheStore
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfCacheStore)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
CacheStore stores an element in the distributed cache of the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) CacheStore(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: CacheStore
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfCacheStore)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

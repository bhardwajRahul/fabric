package loginapi

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
const Hostname = "login.example"

// Endpoint routes.
const (
	RouteOfLogin       = `/login`        // MARKER: Login
	RouteOfLogout      = `/logout`       // MARKER: Logout
	RouteOfWelcome     = `/welcome`      // MARKER: Welcome
	RouteOfAdminOnly   = `/admin-only`   // MARKER: AdminOnly
	RouteOfManagerOnly = `/manager-only` // MARKER: ManagerOnly
)

// Endpoint URLs.
var (
	URLOfLogin       = httpx.JoinHostAndPath(Hostname, RouteOfLogin)       // MARKER: Login
	URLOfLogout      = httpx.JoinHostAndPath(Hostname, RouteOfLogout)      // MARKER: Logout
	URLOfWelcome     = httpx.JoinHostAndPath(Hostname, RouteOfWelcome)     // MARKER: Welcome
	URLOfAdminOnly   = httpx.JoinHostAndPath(Hostname, RouteOfAdminOnly)   // MARKER: AdminOnly
	URLOfManagerOnly = httpx.JoinHostAndPath(Hostname, RouteOfManagerOnly) // MARKER: ManagerOnly
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
Login renders a simple login screen that authenticates a user.
Known users are hardcoded as "admin", "manager" and "user".
The password is "password".

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) Login(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: Login
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfLogin)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Login renders a simple login screen that authenticates a user.
Known users are hardcoded as "admin", "manager" and "user".
The password is "password".

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Login(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: Login
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfLogin)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Logout renders a page that logs out the user.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) Logout(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: Logout
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfLogout)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Logout renders a page that logs out the user.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Logout(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: Logout
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfLogout)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Welcome renders a page that is shown to the user after a successful login.
Rendering is adjusted based on the user's roles.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) Welcome(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: Welcome
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWelcome)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Welcome renders a page that is shown to the user after a successful login.
Rendering is adjusted based on the user's roles.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Welcome(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: Welcome
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWelcome)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
AdminOnly is only accessible by admins.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) AdminOnly(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: AdminOnly
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfAdminOnly)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
AdminOnly is only accessible by admins.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) AdminOnly(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: AdminOnly
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfAdminOnly)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
ManagerOnly is only accessible by managers.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c Client) ManagerOnly(ctx context.Context, relativeURL string) (res *http.Response, err error) { // MARKER: ManagerOnly
	return _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfManagerOnly)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

/*
ManagerOnly is only accessible by managers.

If a URL is provided, it is resolved relative to the URL of the endpoint.
*/
func (_c MulticastClient) ManagerOnly(ctx context.Context, relativeURL string) <-chan *pub.Response { // MARKER: ManagerOnly
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfManagerOnly)),
		pub.RelativeURL(relativeURL),
		pub.Options(_c.opts...),
	)
}

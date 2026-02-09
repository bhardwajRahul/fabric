package eventsourceapi

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
const Hostname = "eventsource.example"

// Endpoint routes.
const (
	RouteOfRegister       = `:443/register`        // MARKER: Register
	RouteOfOnAllowRegister = `:417/on-allow-register` // MARKER: OnAllowRegister
	RouteOfOnRegistered   = `:417/on-registered`    // MARKER: OnRegistered
)

// Endpoint URLs.
var (
	URLOfRegister       = httpx.JoinHostAndPath(Hostname, RouteOfRegister)       // MARKER: Register
	URLOfOnAllowRegister = httpx.JoinHostAndPath(Hostname, RouteOfOnAllowRegister) // MARKER: OnAllowRegister
	URLOfOnRegistered   = httpx.JoinHostAndPath(Hostname, RouteOfOnRegistered)   // MARKER: OnRegistered
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

// --- Register function ---

// RegisterIn are the input arguments of Register.
type RegisterIn struct { // MARKER: Register
	Email string `json:"email,omitzero"`
}

// RegisterOut are the output arguments of Register.
type RegisterOut struct { // MARKER: Register
	Allowed bool `json:"allowed,omitzero"`
}

// RegisterResponse is the response to Register.
type RegisterResponse struct { // MARKER: Register
	data         RegisterOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *RegisterResponse) Get() (allowed bool, err error) { // MARKER: Register
	return _res.data.Allowed, _res.err
}

/*
Register attempts to register a new user.
*/
func (_c MulticastClient) Register(ctx context.Context, email string) <-chan *RegisterResponse { // MARKER: Register
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRegister)
	_in := RegisterIn{
		Email: email,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *RegisterResponse, 1)
		_res <- &RegisterResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	_res := make(chan *RegisterResponse, cap(_ch))
	for _i := range _ch {
		var _r RegisterResponse
		_httpRes, _err := _i.Get()
		_r.HTTPResponse = _httpRes
		if _err != nil {
			_r.err = _err // No trace
		} else {
			_err = httpx.ReadOutputPayload(_httpRes, &_r.data)
			if _err != nil {
				_r.err = errors.Trace(_err)
			}
		}
		_res <- &_r
	}
	close(_res)
	return _res
}

/*
Register attempts to register a new user.
*/
func (_c Client) Register(ctx context.Context, email string) (allowed bool, err error) { // MARKER: Register
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRegister)
	_in := RegisterIn{
		Email: email,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		err = _err // No trace
		return
	}
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _out RegisterOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	allowed = _out.Allowed
	return
}

// --- OnAllowRegister event ---

// OnAllowRegisterIn are the input arguments of OnAllowRegister.
type OnAllowRegisterIn struct { // MARKER: OnAllowRegister
	Email string `json:"email,omitzero"`
}

// OnAllowRegisterOut are the output arguments of OnAllowRegister.
type OnAllowRegisterOut struct { // MARKER: OnAllowRegister
	Allow bool `json:"allow,omitzero"`
}

// OnAllowRegisterResponse is the response to OnAllowRegister.
type OnAllowRegisterResponse struct { // MARKER: OnAllowRegister
	data         OnAllowRegisterOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *OnAllowRegisterResponse) Get() (allow bool, err error) { // MARKER: OnAllowRegister
	return _res.data.Allow, _res.err
}

/*
OnAllowRegister is called before a user is allowed to register.
Event sinks are given the opportunity to block the registration.
*/
func (_c MulticastTrigger) OnAllowRegister(ctx context.Context, email string) <-chan *OnAllowRegisterResponse { // MARKER: OnAllowRegister
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfOnAllowRegister)
	_in := OnAllowRegisterIn{
		Email: email,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *OnAllowRegisterResponse, 1)
		_res <- &OnAllowRegisterResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	_res := make(chan *OnAllowRegisterResponse, cap(_ch))
	for _i := range _ch {
		var _r OnAllowRegisterResponse
		_httpRes, _err := _i.Get()
		_r.HTTPResponse = _httpRes
		if _err != nil {
			_r.err = _err // No trace
		} else {
			_err = httpx.ReadOutputPayload(_httpRes, &_r.data)
			if _err != nil {
				_r.err = errors.Trace(_err)
			}
		}
		_res <- &_r
	}
	close(_res)
	return _res
}

/*
OnAllowRegister is called before a user is allowed to register.
Event sinks are given the opportunity to block the registration.
*/
func (c Hook) OnAllowRegister(handler func(ctx context.Context, email string) (allow bool, err error)) (unsub func() (err error), err error) { // MARKER: OnAllowRegister
	doOnAllowRegister := func(w http.ResponseWriter, r *http.Request) error {
		var i OnAllowRegisterIn
		var o OnAllowRegisterOut
		err = httpx.ReadInputPayload(r, RouteOfOnAllowRegister, &i)
		if err != nil {
			return errors.Trace(err)
		}
		o.Allow, err = handler(r.Context(), i.Email)
		if err != nil {
			return err // No trace
		}
		err = httpx.WriteOutputPayload(w, o)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	method := "POST"
	path := httpx.JoinHostAndPath(c.host, RouteOfOnAllowRegister)
	unsub, err = c.svc.Subscribe(method, path, doOnAllowRegister, c.opts...)
	return unsub, errors.Trace(err)
}

// --- OnRegistered event ---

// OnRegisteredIn are the input arguments of OnRegistered.
type OnRegisteredIn struct { // MARKER: OnRegistered
	Email string `json:"email,omitzero"`
}

// OnRegisteredOut are the output arguments of OnRegistered.
type OnRegisteredOut struct { // MARKER: OnRegistered
}

// OnRegisteredResponse is the response to OnRegistered.
type OnRegisteredResponse struct { // MARKER: OnRegistered
	data         OnRegisteredOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *OnRegisteredResponse) Get() (err error) { // MARKER: OnRegistered
	return _res.err
}

/*
OnRegistered is called when a user is successfully registered.
*/
func (_c MulticastTrigger) OnRegistered(ctx context.Context, email string) <-chan *OnRegisteredResponse { // MARKER: OnRegistered
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfOnRegistered)
	_in := OnRegisteredIn{
		Email: email,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *OnRegisteredResponse, 1)
		_res <- &OnRegisteredResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(_method),
		pub.URL(_url),
		pub.Query(_query),
		pub.Body(_body),
		pub.Options(_c.opts...),
	)
	_res := make(chan *OnRegisteredResponse, cap(_ch))
	for _i := range _ch {
		var _r OnRegisteredResponse
		_httpRes, _err := _i.Get()
		_r.HTTPResponse = _httpRes
		if _err != nil {
			_r.err = _err // No trace
		} else {
			_err = httpx.ReadOutputPayload(_httpRes, &_r.data)
			if _err != nil {
				_r.err = errors.Trace(_err)
			}
		}
		_res <- &_r
	}
	close(_res)
	return _res
}

/*
OnRegistered is called when a user is successfully registered.
*/
func (c Hook) OnRegistered(handler func(ctx context.Context, email string) (err error)) (unsub func() (err error), err error) { // MARKER: OnRegistered
	doOnRegistered := func(w http.ResponseWriter, r *http.Request) error {
		var i OnRegisteredIn
		var o OnRegisteredOut
		err = httpx.ReadInputPayload(r, RouteOfOnRegistered, &i)
		if err != nil {
			return errors.Trace(err)
		}
		err = handler(r.Context(), i.Email)
		if err != nil {
			return err // No trace
		}
		err = httpx.WriteOutputPayload(w, o)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	method := "POST"
	path := httpx.JoinHostAndPath(c.host, RouteOfOnRegistered)
	unsub, err = c.svc.Subscribe(method, path, doOnRegistered, c.opts...)
	return unsub, errors.Trace(err)
}

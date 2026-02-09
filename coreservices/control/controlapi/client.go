/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controlapi

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
const Hostname = "control.core"

// Endpoint routes.
const (
	RouteOfPing          = `:888/ping`           // MARKER: Ping
	RouteOfConfigRefresh = `:888/config-refresh`  // MARKER: ConfigRefresh
	RouteOfTrace         = `:888/trace`           // MARKER: Trace
	RouteOfMetrics       = `:888/metrics`         // MARKER: Metrics
	RouteOfOnNewSubs     = `:888/on-new-subs`     // MARKER: OnNewSubs
)

// Endpoint URLs.
var (
	URLOfPing          = httpx.JoinHostAndPath(Hostname, RouteOfPing)          // MARKER: Ping
	URLOfConfigRefresh = httpx.JoinHostAndPath(Hostname, RouteOfConfigRefresh) // MARKER: ConfigRefresh
	URLOfTrace         = httpx.JoinHostAndPath(Hostname, RouteOfTrace)         // MARKER: Trace
	URLOfMetrics       = httpx.JoinHostAndPath(Hostname, RouteOfMetrics)       // MARKER: Metrics
	URLOfOnNewSubs     = httpx.JoinHostAndPath(Hostname, RouteOfOnNewSubs)     // MARKER: OnNewSubs
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

// --- Ping ---

// PingIn are the input arguments of Ping.
type PingIn struct { // MARKER: Ping
}

// PingOut are the output arguments of Ping.
type PingOut struct { // MARKER: Ping
	Pong int `json:"pong,omitzero"`
}

// PingResponse is the response to Ping.
type PingResponse struct { // MARKER: Ping
	data         PingOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *PingResponse) Get() (pong int, err error) { // MARKER: Ping
	return _res.data.Pong, _res.err
}

/*
Ping responds to the message with a pong.
*/
func (_c MulticastClient) Ping(ctx context.Context) <-chan *PingResponse { // MARKER: Ping
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfPing)
	_in := PingIn{}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *PingResponse, 1)
		_res <- &PingResponse{err: _err} // No trace
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
	_res := make(chan *PingResponse, cap(_ch))
	for _i := range _ch {
		var _r PingResponse
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
Ping responds to the message with a pong.
*/
func (_c Client) Ping(ctx context.Context) (pong int, err error) { // MARKER: Ping
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfPing)
	_in := PingIn{}
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
	var _out PingOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.Pong, nil
}

// --- ConfigRefresh ---

// ConfigRefreshIn are the input arguments of ConfigRefresh.
type ConfigRefreshIn struct { // MARKER: ConfigRefresh
}

// ConfigRefreshOut are the output arguments of ConfigRefresh.
type ConfigRefreshOut struct { // MARKER: ConfigRefresh
}

// ConfigRefreshResponse is the response to ConfigRefresh.
type ConfigRefreshResponse struct { // MARKER: ConfigRefresh
	data         ConfigRefreshOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *ConfigRefreshResponse) Get() (err error) { // MARKER: ConfigRefresh
	return _res.err
}

/*
ConfigRefresh pulls the latest config values from the configurator microservice.
*/
func (_c MulticastClient) ConfigRefresh(ctx context.Context) <-chan *ConfigRefreshResponse { // MARKER: ConfigRefresh
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfConfigRefresh)
	_in := ConfigRefreshIn{}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *ConfigRefreshResponse, 1)
		_res <- &ConfigRefreshResponse{err: _err} // No trace
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
	_res := make(chan *ConfigRefreshResponse, cap(_ch))
	for _i := range _ch {
		var _r ConfigRefreshResponse
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
ConfigRefresh pulls the latest config values from the configurator microservice.
*/
func (_c Client) ConfigRefresh(ctx context.Context) (err error) { // MARKER: ConfigRefresh
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfConfigRefresh)
	_in := ConfigRefreshIn{}
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
	var _out ConfigRefreshOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

// --- Trace ---

// TraceIn are the input arguments of Trace.
type TraceIn struct { // MARKER: Trace
	ID string `json:"id,omitzero"`
}

// TraceOut are the output arguments of Trace.
type TraceOut struct { // MARKER: Trace
}

// TraceResponse is the response to Trace.
type TraceResponse struct { // MARKER: Trace
	data         TraceOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *TraceResponse) Get() (err error) { // MARKER: Trace
	return _res.err
}

/*
Trace forces exporting the indicated tracing span.
*/
func (_c MulticastClient) Trace(ctx context.Context, id string) <-chan *TraceResponse { // MARKER: Trace
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfTrace)
	_in := TraceIn{
		ID: id,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *TraceResponse, 1)
		_res <- &TraceResponse{err: _err} // No trace
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
	_res := make(chan *TraceResponse, cap(_ch))
	for _i := range _ch {
		var _r TraceResponse
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
Trace forces exporting the indicated tracing span.
*/
func (_c Client) Trace(ctx context.Context, id string) (err error) { // MARKER: Trace
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfTrace)
	_in := TraceIn{
		ID: id,
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
	var _out TraceOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

// --- Metrics (web endpoint) ---

/*
Metrics returns the Prometheus metrics collected by the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) Metrics(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: Metrics
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfMetrics)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Metrics returns the Prometheus metrics collected by the microservice.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Metrics(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: Metrics
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfMetrics)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

// --- OnNewSubs (outbound event) ---

// OnNewSubsIn are the input arguments of OnNewSubs.
type OnNewSubsIn struct { // MARKER: OnNewSubs
	Hosts []string `json:"hosts,omitzero"`
}

// OnNewSubsOut are the output arguments of OnNewSubs.
type OnNewSubsOut struct { // MARKER: OnNewSubs
}

// OnNewSubsResponse is the response to OnNewSubs.
type OnNewSubsResponse struct { // MARKER: OnNewSubs
	data         OnNewSubsOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *OnNewSubsResponse) Get() (err error) { // MARKER: OnNewSubs
	return _res.err
}

/*
OnNewSubs informs other microservices of new subscriptions, enabling them to update their known responders cache appropriately.
*/
func (_c MulticastTrigger) OnNewSubs(ctx context.Context, hosts []string) <-chan *OnNewSubsResponse { // MARKER: OnNewSubs
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfOnNewSubs)
	_in := OnNewSubsIn{
		Hosts: hosts,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *OnNewSubsResponse, 1)
		_res <- &OnNewSubsResponse{err: _err} // No trace
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
	_res := make(chan *OnNewSubsResponse, cap(_ch))
	for _i := range _ch {
		var _r OnNewSubsResponse
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
OnNewSubs informs other microservices of new subscriptions, enabling them to update their known responders cache appropriately.
*/
func (c Hook) OnNewSubs(handler func(ctx context.Context, hosts []string) (err error)) (unsub func() error, err error) { // MARKER: OnNewSubs
	doOnNewSubs := func(w http.ResponseWriter, r *http.Request) error {
		var i OnNewSubsIn
		var o OnNewSubsOut
		err = httpx.ReadInputPayload(r, RouteOfOnNewSubs, &i)
		if err != nil {
			return errors.Trace(err)
		}
		err = handler(r.Context(), i.Hosts)
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
	path := httpx.JoinHostAndPath(c.host, RouteOfOnNewSubs)
	unsub, err = c.svc.Subscribe(method, path, doOnNewSubs, c.opts...)
	return unsub, errors.Trace(err)
}

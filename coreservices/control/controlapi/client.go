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
	"iter"
	"net/http"
	"reflect"

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
	_ = marshalRequest
	_ = marshalPublish
	_ = marshalFunction
)

// Hostname is the default hostname of the microservice.
const Hostname = "control.core"

// Def defines an endpoint of the microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL to the endpoint.
func (d *Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	Ping          = Def{Method: "ANY", Route: `:888/ping`}           // MARKER: Ping
	ConfigRefresh = Def{Method: "ANY", Route: `:888/config-refresh`} // MARKER: ConfigRefresh
	Trace         = Def{Method: "ANY", Route: `:888/trace`}          // MARKER: Trace
	Metrics       = Def{Method: "ANY", Route: `:888/metrics`}        // MARKER: Metrics
	OnNewSubs     = Def{Method: "POST", Route: `:888/on-new-subs`}   // MARKER: OnNewSubs
)

// multicastResponse packs the response of a functional multicast.
type multicastResponse struct {
	data         any
	HTTPResponse *http.Response
	err          error
}

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{svc: caller, host: Hostname}
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
	return Client{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{svc: caller, host: Hostname}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{svc: listener, host: Hostname}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{svc: c.svc, host: host, opts: c.opts}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{svc: c.svc, host: c.host, opts: append(c.opts, opts...)}
}

// marshalRequest supports functional endpoints.
func marshalRequest(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) (err error) {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return err // No trace
	}
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = httpx.ReadOutputPayload(httpRes, out)
	return errors.Trace(err)
}

// marshalPublish supports multicast functional endpoints.
func marshalPublish(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) iter.Seq[*multicastResponse] {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return func(yield func(*multicastResponse) bool) {
			yield(&multicastResponse{err: err})
		}
	}
	_queue := svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	return func(yield func(*multicastResponse) bool) {
		for qi := range _queue {
			httpResp, err := qi.Get()
			if err == nil {
				reflect.ValueOf(out).Elem().SetZero()
				err = httpx.ReadOutputPayload(httpResp, out)
			}
			if err != nil {
				if !yield(&multicastResponse{err: err, HTTPResponse: httpResp}) {
					return
				}
			} else {
				if !yield(&multicastResponse{data: out, HTTPResponse: httpResp}) {
					return
				}
			}
		}
	}
}

// marshalFunction handled marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// --- Ping ---

// PingIn are the input arguments of Ping.
type PingIn struct { // MARKER: Ping
}

// PingOut are the output arguments of Ping.
type PingOut struct { // MARKER: Ping
	Pong int `json:"pong,omitzero"`
}

// PingResponse packs the response of Ping.
type PingResponse multicastResponse // MARKER: Ping

// Get unpacks the return arguments of Ping.
func (_res *PingResponse) Get() (pong int, err error) { // MARKER: Ping
	_d := _res.data.(*PingOut)
	return _d.Pong, _res.err
}

/*
Ping responds with a pong.
*/
func (_c MulticastClient) Ping(ctx context.Context) iter.Seq[*PingResponse] { // MARKER: Ping
	_in := PingIn{}
	_out := PingOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Ping.Method, Ping.Route, &_in, &_out)
	return func(yield func(*PingResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*PingResponse)(_r)) {
				return
			}
		}
	}
}

/*
Ping responds with a pong.
*/
func (_c Client) Ping(ctx context.Context) (pong int, err error) { // MARKER: Ping
	_in := PingIn{}
	_out := PingOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Ping.Method, Ping.Route, &_in, &_out)
	return _out.Pong, err // No trace
}

// --- ConfigRefresh ---

// ConfigRefreshIn are the input arguments of ConfigRefresh.
type ConfigRefreshIn struct { // MARKER: ConfigRefresh
}

// ConfigRefreshOut are the output arguments of ConfigRefresh.
type ConfigRefreshOut struct { // MARKER: ConfigRefresh
}

// ConfigRefreshResponse packs the response of ConfigRefresh.
type ConfigRefreshResponse multicastResponse // MARKER: ConfigRefresh

// Get unpacks the return arguments of ConfigRefresh.
func (_res *ConfigRefreshResponse) Get() (err error) { // MARKER: ConfigRefresh
	return _res.err
}

/*
ConfigRefresh pulls the latest config values.
*/
func (_c MulticastClient) ConfigRefresh(ctx context.Context) iter.Seq[*ConfigRefreshResponse] { // MARKER: ConfigRefresh
	_in := ConfigRefreshIn{}
	_out := ConfigRefreshOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, ConfigRefresh.Method, ConfigRefresh.Route, &_in, &_out)
	return func(yield func(*ConfigRefreshResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ConfigRefreshResponse)(_r)) {
				return
			}
		}
	}
}

/*
ConfigRefresh pulls the latest config values.
*/
func (_c Client) ConfigRefresh(ctx context.Context) (err error) { // MARKER: ConfigRefresh
	_in := ConfigRefreshIn{}
	_out := ConfigRefreshOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, ConfigRefresh.Method, ConfigRefresh.Route, &_in, &_out)
	return err // No trace
}

// --- Trace ---

// TraceIn are the input arguments of Trace.
type TraceIn struct { // MARKER: Trace
	ID string `json:"id,omitzero"`
}

// TraceOut are the output arguments of Trace.
type TraceOut struct { // MARKER: Trace
}

// TraceResponse packs the response of Trace.
type TraceResponse multicastResponse // MARKER: Trace

// Get unpacks the return arguments of Trace.
func (_res *TraceResponse) Get() (err error) { // MARKER: Trace
	return _res.err
}

/*
Trace forces exporting a tracing span.
*/
func (_c MulticastClient) Trace(ctx context.Context, id string) iter.Seq[*TraceResponse] { // MARKER: Trace
	_in := TraceIn{ID: id}
	_out := TraceOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Trace.Method, Trace.Route, &_in, &_out)
	return func(yield func(*TraceResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*TraceResponse)(_r)) {
				return
			}
		}
	}
}

/*
Trace forces exporting a tracing span.
*/
func (_c Client) Trace(ctx context.Context, id string) (err error) { // MARKER: Trace
	_in := TraceIn{ID: id}
	_out := TraceOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Trace.Method, Trace.Route, &_in, &_out)
	return err // No trace
}

// --- Metrics (web endpoint) ---

/*
Metrics returns Prometheus metrics.

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
		pub.URL(httpx.JoinHostAndPath(_c.host, Metrics.Route)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Metrics returns Prometheus metrics.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Metrics(ctx context.Context, method string, relativeURL string, body any) iter.Seq[*pub.Response] { // MARKER: Metrics
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, Metrics.Route)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

// --- OnNewSubs (outbound event) ---

// OnNewSubsIn are the input arguments of OnNewSubs.
type OnNewSubsIn struct { // MARKER: OnNewSubs
}

// OnNewSubsOut are the output arguments of OnNewSubs.
type OnNewSubsOut struct { // MARKER: OnNewSubs
}

// OnNewSubsResponse packs the response of OnNewSubs.
type OnNewSubsResponse multicastResponse // MARKER: OnNewSubs

// Get unpacks the return arguments of OnNewSubs.
func (_res *OnNewSubsResponse) Get() (err error) { // MARKER: OnNewSubs
	return _res.err
}

/*
OnNewSubs informs of new subscriptions.
*/
func (_c MulticastTrigger) OnNewSubs(ctx context.Context) iter.Seq[*OnNewSubsResponse] { // MARKER: OnNewSubs
	_in := OnNewSubsIn{}
	_out := OnNewSubsOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, OnNewSubs.Method, OnNewSubs.Route, &_in, &_out)
	return func(yield func(*OnNewSubsResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*OnNewSubsResponse)(_r)) {
				return
			}
		}
	}
}

/*
OnNewSubs informs of new subscriptions.
*/
func (c Hook) OnNewSubs(handler func(ctx context.Context) (err error)) (unsub func() error, err error) { // MARKER: OnNewSubs
	doOnNewSubs := func(w http.ResponseWriter, r *http.Request) error {
		var in OnNewSubsIn
		var out OnNewSubsOut
		err = marshalFunction(w, r, OnNewSubs.Route, &in, &out, func(_ any, _ any) error {
			err = handler(r.Context())
			return err
		})
		return err // No trace
	}
	path := httpx.JoinHostAndPath(c.host, OnNewSubs.Route)
	unsub, err = c.svc.Subscribe(OnNewSubs.Method, path, doOnNewSubs, c.opts...)
	return unsub, errors.Trace(err)
}

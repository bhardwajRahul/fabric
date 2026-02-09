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

package configuratorapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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
const Hostname = "configurator.core"

// Endpoint routes.
const (
	RouteOfValues   = `:888/values`    // MARKER: Values
	RouteOfRefresh  = `:444/refresh`   // MARKER: Refresh
	RouteOfSyncRepo = `:888/sync-repo` // MARKER: SyncRepo

	// Deprecated routes
	RouteOfValues443  = `:443/values`  // MARKER: Values443
	RouteOfRefresh443 = `:443/refresh` // MARKER: Refresh443
	RouteOfSync443    = `:443/sync`    // MARKER: Sync443
)

// Endpoint URLs.
var (
	URLOfValues   = httpx.JoinHostAndPath(Hostname, RouteOfValues)   // MARKER: Values
	URLOfRefresh  = httpx.JoinHostAndPath(Hostname, RouteOfRefresh)  // MARKER: Refresh
	URLOfSyncRepo = httpx.JoinHostAndPath(Hostname, RouteOfSyncRepo) // MARKER: SyncRepo

	// Deprecated URLs
	URLOfValues443  = httpx.JoinHostAndPath(Hostname, RouteOfValues443)  // MARKER: Values443
	URLOfRefresh443 = httpx.JoinHostAndPath(Hostname, RouteOfRefresh443) // MARKER: Refresh443
	URLOfSync443    = httpx.JoinHostAndPath(Hostname, RouteOfSync443)    // MARKER: Sync443
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

// ValuesIn are the input arguments of Values.
type ValuesIn struct { // MARKER: Values
	Names []string `json:"names,omitzero"`
}

// ValuesOut are the output arguments of Values.
type ValuesOut struct { // MARKER: Values
	Values map[string]string `json:"values,omitzero"`
}

// ValuesResponse is the response to Values.
type ValuesResponse struct { // MARKER: Values
	data         ValuesOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *ValuesResponse) Get() (values map[string]string, err error) { // MARKER: Values
	return _res.data.Values, _res.err
}

/*
Values returns the values associated with the specified config property names for the caller microservice.
*/
func (_c Client) Values(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValues)
	_in := ValuesIn{
		Names: names,
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
	var _out ValuesOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.Values, nil
}

/*
Values returns the values associated with the specified config property names for the caller microservice.
*/
func (_c MulticastClient) Values(ctx context.Context, names []string) <-chan *ValuesResponse { // MARKER: Values
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValues)
	_in := ValuesIn{
		Names: names,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *ValuesResponse, 1)
		_res <- &ValuesResponse{err: _err} // No trace
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
	_res := make(chan *ValuesResponse, cap(_ch))
	for _i := range _ch {
		var _r ValuesResponse
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

// RefreshIn are the input arguments of Refresh.
type RefreshIn struct { // MARKER: Refresh
}

// RefreshOut are the output arguments of Refresh.
type RefreshOut struct { // MARKER: Refresh
}

// RefreshResponse is the response to Refresh.
type RefreshResponse struct { // MARKER: Refresh
	data         RefreshOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *RefreshResponse) Get() (err error) { // MARKER: Refresh
	return _res.err
}

/*
Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.
*/
func (_c Client) Refresh(ctx context.Context) (err error) { // MARKER: Refresh
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRefresh)
	_in := RefreshIn{}
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
	var _out RefreshOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

/*
Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.
*/
func (_c MulticastClient) Refresh(ctx context.Context) <-chan *RefreshResponse { // MARKER: Refresh
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRefresh)
	_in := RefreshIn{}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *RefreshResponse, 1)
		_res <- &RefreshResponse{err: _err} // No trace
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
	_res := make(chan *RefreshResponse, cap(_ch))
	for _i := range _ch {
		var _r RefreshResponse
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

// SyncRepoIn are the input arguments of SyncRepo.
type SyncRepoIn struct { // MARKER: SyncRepo
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// SyncRepoOut are the output arguments of SyncRepo.
type SyncRepoOut struct { // MARKER: SyncRepo
}

// SyncRepoResponse is the response to SyncRepo.
type SyncRepoResponse struct { // MARKER: SyncRepo
	data         SyncRepoOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *SyncRepoResponse) Get() (err error) { // MARKER: SyncRepo
	return _res.err
}

/*
SyncRepo is used to synchronize values among replica peers of the configurator.
*/
func (_c Client) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: SyncRepo
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSyncRepo)
	_in := SyncRepoIn{
		Timestamp: timestamp,
		Values:    values,
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
	var _out SyncRepoOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

/*
SyncRepo is used to synchronize values among replica peers of the configurator.
*/
func (_c MulticastClient) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) <-chan *SyncRepoResponse { // MARKER: SyncRepo
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSyncRepo)
	_in := SyncRepoIn{
		Timestamp: timestamp,
		Values:    values,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *SyncRepoResponse, 1)
		_res <- &SyncRepoResponse{err: _err} // No trace
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
	_res := make(chan *SyncRepoResponse, cap(_ch))
	for _i := range _ch {
		var _r SyncRepoResponse
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

// Values443In are the input arguments of Values443.
type Values443In struct { // MARKER: Values443
	Names []string `json:"names,omitzero"`
}

// Values443Out are the output arguments of Values443.
type Values443Out struct { // MARKER: Values443
	Values map[string]string `json:"values,omitzero"`
}

// Values443Response is the response to Values443.
type Values443Response struct { // MARKER: Values443
	data         Values443Out
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *Values443Response) Get() (values map[string]string, err error) { // MARKER: Values443
	return _res.data.Values, _res.err
}

/*
Values443 is deprecated.
*/
func (_c Client) Values443(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values443
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValues443)
	_in := Values443In{
		Names: names,
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
	var _out Values443Out
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.Values, nil
}

/*
Values443 is deprecated.
*/
func (_c MulticastClient) Values443(ctx context.Context, names []string) <-chan *Values443Response { // MARKER: Values443
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValues443)
	_in := Values443In{
		Names: names,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *Values443Response, 1)
		_res <- &Values443Response{err: _err} // No trace
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
	_res := make(chan *Values443Response, cap(_ch))
	for _i := range _ch {
		var _r Values443Response
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

// Refresh443In are the input arguments of Refresh443.
type Refresh443In struct { // MARKER: Refresh443
}

// Refresh443Out are the output arguments of Refresh443.
type Refresh443Out struct { // MARKER: Refresh443
}

// Refresh443Response is the response to Refresh443.
type Refresh443Response struct { // MARKER: Refresh443
	data         Refresh443Out
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *Refresh443Response) Get() (err error) { // MARKER: Refresh443
	return _res.err
}

/*
Refresh443 is deprecated.
*/
func (_c Client) Refresh443(ctx context.Context) (err error) { // MARKER: Refresh443
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRefresh443)
	_in := Refresh443In{}
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
	var _out Refresh443Out
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

/*
Refresh443 is deprecated.
*/
func (_c MulticastClient) Refresh443(ctx context.Context) <-chan *Refresh443Response { // MARKER: Refresh443
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfRefresh443)
	_in := Refresh443In{}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *Refresh443Response, 1)
		_res <- &Refresh443Response{err: _err} // No trace
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
	_res := make(chan *Refresh443Response, cap(_ch))
	for _i := range _ch {
		var _r Refresh443Response
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

// Sync443In are the input arguments of Sync443.
type Sync443In struct { // MARKER: Sync443
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// Sync443Out are the output arguments of Sync443.
type Sync443Out struct { // MARKER: Sync443
}

// Sync443Response is the response to Sync443.
type Sync443Response struct { // MARKER: Sync443
	data         Sync443Out
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *Sync443Response) Get() (err error) { // MARKER: Sync443
	return _res.err
}

/*
Sync443 is deprecated.
*/
func (_c Client) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: Sync443
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSync443)
	_in := Sync443In{
		Timestamp: timestamp,
		Values:    values,
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
	var _out Sync443Out
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

/*
Sync443 is deprecated.
*/
func (_c MulticastClient) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) <-chan *Sync443Response { // MARKER: Sync443
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSync443)
	_in := Sync443In{
		Timestamp: timestamp,
		Values:    values,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *Sync443Response, 1)
		_res <- &Sync443Response{err: _err} // No trace
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
	_res := make(chan *Sync443Response, cap(_ch))
	for _i := range _ch {
		var _r Sync443Response
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

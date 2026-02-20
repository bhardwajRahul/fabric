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
	"iter"
	"net/http"
	"reflect"
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
	_ = marshalRequest
	_ = marshalPublish
	_ = marshalFunction
)

// Hostname is the default hostname of the microservice.
const Hostname = "configurator.core"

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
	Values     = Def{Method: "ANY", Route: `:888/values`}    // MARKER: Values
	Refresh    = Def{Method: "ANY", Route: `:444/refresh`}   // MARKER: Refresh
	SyncRepo   = Def{Method: "ANY", Route: `:888/sync-repo`} // MARKER: SyncRepo
	Values443  = Def{Method: "ANY", Route: `:443/values`}    // MARKER: Values443
	Refresh443 = Def{Method: "ANY", Route: `:443/refresh`}   // MARKER: Refresh443
	Sync443    = Def{Method: "ANY", Route: `:443/sync`}      // MARKER: Sync443
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

// ValuesIn are the input arguments of Values.
type ValuesIn struct { // MARKER: Values
	Names []string `json:"names,omitzero"`
}

// ValuesOut are the output arguments of Values.
type ValuesOut struct { // MARKER: Values
	Values map[string]string `json:"values,omitzero"`
}

// ValuesResponse packs the response of Values.
type ValuesResponse multicastResponse // MARKER: Values

// Get unpacks the return arguments of Values.
func (_res *ValuesResponse) Get() (values map[string]string, err error) { // MARKER: Values
	_d := _res.data.(*ValuesOut)
	return _d.Values, _res.err
}

/*
Values returns the values associated with the specified config property names for the caller microservice.
*/
func (_c Client) Values(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values
	_in := ValuesIn{Names: names}
	_out := ValuesOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Values.Method, Values.Route, &_in, &_out)
	return _out.Values, err // No trace
}

/*
Values returns the values associated with the specified config property names for the caller microservice.
*/
func (_c MulticastClient) Values(ctx context.Context, names []string) iter.Seq[*ValuesResponse] { // MARKER: Values
	_in := ValuesIn{Names: names}
	_out := ValuesOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Values.Method, Values.Route, &_in, &_out)
	return func(yield func(*ValuesResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ValuesResponse)(_r)) {
				return
			}
		}
	}
}

// RefreshIn are the input arguments of Refresh.
type RefreshIn struct { // MARKER: Refresh
}

// RefreshOut are the output arguments of Refresh.
type RefreshOut struct { // MARKER: Refresh
}

// RefreshResponse packs the response of Refresh.
type RefreshResponse multicastResponse // MARKER: Refresh

// Get unpacks the return arguments of Refresh.
func (_res *RefreshResponse) Get() (err error) { // MARKER: Refresh
	return _res.err
}

/*
Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.
*/
func (_c Client) Refresh(ctx context.Context) (err error) { // MARKER: Refresh
	_in := RefreshIn{}
	_out := RefreshOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Refresh.Method, Refresh.Route, &_in, &_out)
	return err // No trace
}

/*
Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.
*/
func (_c MulticastClient) Refresh(ctx context.Context) iter.Seq[*RefreshResponse] { // MARKER: Refresh
	_in := RefreshIn{}
	_out := RefreshOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Refresh.Method, Refresh.Route, &_in, &_out)
	return func(yield func(*RefreshResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*RefreshResponse)(_r)) {
				return
			}
		}
	}
}

// SyncRepoIn are the input arguments of SyncRepo.
type SyncRepoIn struct { // MARKER: SyncRepo
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// SyncRepoOut are the output arguments of SyncRepo.
type SyncRepoOut struct { // MARKER: SyncRepo
}

// SyncRepoResponse packs the response of SyncRepo.
type SyncRepoResponse multicastResponse // MARKER: SyncRepo

// Get unpacks the return arguments of SyncRepo.
func (_res *SyncRepoResponse) Get() (err error) { // MARKER: SyncRepo
	return _res.err
}

/*
SyncRepo is used to synchronize values among replica peers of the configurator.
*/
func (_c Client) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: SyncRepo
	_in := SyncRepoIn{Timestamp: timestamp, Values: values}
	_out := SyncRepoOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, SyncRepo.Method, SyncRepo.Route, &_in, &_out)
	return err // No trace
}

/*
SyncRepo is used to synchronize values among replica peers of the configurator.
*/
func (_c MulticastClient) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) iter.Seq[*SyncRepoResponse] { // MARKER: SyncRepo
	_in := SyncRepoIn{Timestamp: timestamp, Values: values}
	_out := SyncRepoOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, SyncRepo.Method, SyncRepo.Route, &_in, &_out)
	return func(yield func(*SyncRepoResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*SyncRepoResponse)(_r)) {
				return
			}
		}
	}
}

// Values443In are the input arguments of Values443.
type Values443In struct { // MARKER: Values443
	Names []string `json:"names,omitzero"`
}

// Values443Out are the output arguments of Values443.
type Values443Out struct { // MARKER: Values443
	Values map[string]string `json:"values,omitzero"`
}

// Values443Response packs the response of Values443.
type Values443Response multicastResponse // MARKER: Values443

// Get unpacks the return arguments of Values443.
func (_res *Values443Response) Get() (values map[string]string, err error) { // MARKER: Values443
	_d := _res.data.(*Values443Out)
	return _d.Values, _res.err
}

/*
Deprecated.
*/
func (_c Client) Values443(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values443
	_in := Values443In{Names: names}
	_out := Values443Out{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Values443.Method, Values443.Route, &_in, &_out)
	return _out.Values, err // No trace
}

/*
Deprecated.
*/
func (_c MulticastClient) Values443(ctx context.Context, names []string) iter.Seq[*Values443Response] { // MARKER: Values443
	_in := Values443In{Names: names}
	_out := Values443Out{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Values443.Method, Values443.Route, &_in, &_out)
	return func(yield func(*Values443Response) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*Values443Response)(_r)) {
				return
			}
		}
	}
}

// Refresh443In are the input arguments of Refresh443.
type Refresh443In struct { // MARKER: Refresh443
}

// Refresh443Out are the output arguments of Refresh443.
type Refresh443Out struct { // MARKER: Refresh443
}

// Refresh443Response packs the response of Refresh443.
type Refresh443Response multicastResponse // MARKER: Refresh443

// Get unpacks the return arguments of Refresh443.
func (_res *Refresh443Response) Get() (err error) { // MARKER: Refresh443
	return _res.err
}

/*
Deprecated.
*/
func (_c Client) Refresh443(ctx context.Context) (err error) { // MARKER: Refresh443
	_in := Refresh443In{}
	_out := Refresh443Out{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Refresh443.Method, Refresh443.Route, &_in, &_out)
	return err // No trace
}

/*
Deprecated.
*/
func (_c MulticastClient) Refresh443(ctx context.Context) iter.Seq[*Refresh443Response] { // MARKER: Refresh443
	_in := Refresh443In{}
	_out := Refresh443Out{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Refresh443.Method, Refresh443.Route, &_in, &_out)
	return func(yield func(*Refresh443Response) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*Refresh443Response)(_r)) {
				return
			}
		}
	}
}

// Sync443In are the input arguments of Sync443.
type Sync443In struct { // MARKER: Sync443
	Timestamp time.Time                    `json:"timestamp,omitzero"`
	Values    map[string]map[string]string `json:"values,omitzero"`
}

// Sync443Out are the output arguments of Sync443.
type Sync443Out struct { // MARKER: Sync443
}

// Sync443Response packs the response of Sync443.
type Sync443Response multicastResponse // MARKER: Sync443

// Get unpacks the return arguments of Sync443.
func (_res *Sync443Response) Get() (err error) { // MARKER: Sync443
	return _res.err
}

/*
Deprecated.
*/
func (_c Client) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: Sync443
	_in := Sync443In{Timestamp: timestamp, Values: values}
	_out := Sync443Out{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Sync443.Method, Sync443.Route, &_in, &_out)
	return err // No trace
}

/*
Deprecated.
*/
func (_c MulticastClient) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) iter.Seq[*Sync443Response] { // MARKER: Sync443
	_in := Sync443In{Timestamp: timestamp, Values: values}
	_out := Sync443Out{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Sync443.Method, Sync443.Route, &_in, &_out)
	return func(yield func(*Sync443Response) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*Sync443Response)(_r)) {
				return
			}
		}
	}
}

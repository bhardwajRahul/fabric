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

package accesstokenapi

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
	"github.com/microbus-io/fabric/workflow"
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
	_ = marshalTask
	_ workflow.Flow
)

// Hostname is the default hostname of the microservice.
const Hostname = "access.token.core"

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
	Mint      = Def{Method: "ANY", Route: ":444/mint"}       // MARKER: Mint
	LocalKeys = Def{Method: "ANY", Route: ":444/local-keys"} // MARKER: LocalKeys
	JWKS      = Def{Method: "ANY", Route: ":888/jwks"}       // MARKER: JWKS
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

// marshalFunction handles marshaling for functional endpoints.
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

// MintIn are the input arguments of Mint.
type MintIn struct { // MARKER: Mint
	Claims any `json:"claims,omitzero"`
}

// MintOut are the output arguments of Mint.
type MintOut struct { // MARKER: Mint
	Token string `json:"token,omitzero"`
}

// MintResponse packs the response of Mint.
type MintResponse multicastResponse // MARKER: Mint

// Get unpacks the return arguments of Mint.
func (_res *MintResponse) Get() (token string, err error) { // MARKER: Mint
	_d := _res.data.(*MintOut)
	return _d.Token, _res.err
}

/*
Mint signs a JWT with the given claims. The token's expiration is set to the time budget of the context.
*/
func (_c MulticastClient) Mint(ctx context.Context, claims any) iter.Seq[*MintResponse] { // MARKER: Mint
	_in := MintIn{Claims: claims}
	_out := MintOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Mint.Method, Mint.Route, &_in, &_out)
	return func(yield func(*MintResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MintResponse)(_r)) {
				return
			}
		}
	}
}

/*
Mint signs a JWT with the given claims. The token's expiration is set to the time budget of the context.
*/
func (_c Client) Mint(ctx context.Context, claims any) (token string, err error) { // MARKER: Mint
	_in := MintIn{Claims: claims}
	_out := MintOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Mint.Method, Mint.Route, &_in, &_out)
	return _out.Token, err // No trace
}

// LocalKeysIn are the input arguments of LocalKeys.
type LocalKeysIn struct { // MARKER: LocalKeys
}

// LocalKeysOut are the output arguments of LocalKeys.
type LocalKeysOut struct { // MARKER: LocalKeys
	Keys []JWK `json:"keys,omitzero"`
}

// LocalKeysResponse packs the response of LocalKeys.
type LocalKeysResponse multicastResponse // MARKER: LocalKeys

// Get unpacks the return arguments of LocalKeys.
func (_res *LocalKeysResponse) Get() (keys []JWK, err error) { // MARKER: LocalKeys
	_d := _res.data.(*LocalKeysOut)
	return _d.Keys, _res.err
}

/*
LocalKeys returns this replica's current and previous public keys.
*/
func (_c MulticastClient) LocalKeys(ctx context.Context) iter.Seq[*LocalKeysResponse] { // MARKER: LocalKeys
	_in := LocalKeysIn{}
	_out := LocalKeysOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, LocalKeys.Method, LocalKeys.Route, &_in, &_out)
	return func(yield func(*LocalKeysResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*LocalKeysResponse)(_r)) {
				return
			}
		}
	}
}

/*
LocalKeys returns this replica's current and previous public keys.
*/
func (_c Client) LocalKeys(ctx context.Context) (keys []JWK, err error) { // MARKER: LocalKeys
	_in := LocalKeysIn{}
	_out := LocalKeysOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, LocalKeys.Method, LocalKeys.Route, &_in, &_out)
	return _out.Keys, err // No trace
}

// JWKSIn are the input arguments of JWKS.
type JWKSIn struct { // MARKER: JWKS
}

// JWKSOut are the output arguments of JWKS.
type JWKSOut struct { // MARKER: JWKS
	Keys []JWK `json:"keys,omitzero"`
}

// JWKSResponse packs the response of JWKS.
type JWKSResponse multicastResponse // MARKER: JWKS

// Get unpacks the return arguments of JWKS.
func (_res *JWKSResponse) Get() (keys []JWK, err error) { // MARKER: JWKS
	_d := _res.data.(*JWKSOut)
	return _d.Keys, _res.err
}

/*
JWKS aggregates public keys from all replicas and returns them as a []JWK.
*/
func (_c MulticastClient) JWKS(ctx context.Context) iter.Seq[*JWKSResponse] { // MARKER: JWKS
	_in := JWKSIn{}
	_out := JWKSOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, JWKS.Method, JWKS.Route, &_in, &_out)
	return func(yield func(*JWKSResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*JWKSResponse)(_r)) {
				return
			}
		}
	}
}

/*
JWKS aggregates public keys from all replicas and returns them as a []JWK.
*/
func (_c Client) JWKS(ctx context.Context) (keys []JWK, err error) { // MARKER: JWKS
	_in := JWKSIn{}
	_out := JWKSOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, JWKS.Method, JWKS.Route, &_in, &_out)
	return _out.Keys, err // No trace
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests. Production code should use
// the foreman Client to create and start flows asynchronously.
type Executor struct {
	svc     service.Publisher
	host    string
	opts    []pub.Option
	inFlow  *workflow.Flow
	outFlow *workflow.Flow
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow}
}

// marshalTask supports task execution via the Executor.
func marshalTask(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any, inFlow *workflow.Flow, outFlow *workflow.Flow) (err error) {
	flow := inFlow
	if flow == nil {
		flow = workflow.NewFlow()
	}
	err = flow.SetState(in)
	if err != nil {
		return errors.Trace(err)
	}
	body, err := json.Marshal(flow)
	if err != nil {
		return errors.Trace(err)
	}
	u := httpx.JoinHostAndPath(host, route)
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Body(body),
		pub.ContentType("application/json"),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = json.NewDecoder(httpRes.Body).Decode(flow)
	if err != nil {
		return errors.Trace(err)
	}
	if outFlow != nil {
		*outFlow = *flow
	}
	if out != nil {
		err = flow.ParseState(out)
		return errors.Trace(err)
	}
	return nil
}

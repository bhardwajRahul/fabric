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

package calculatorapi

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

// ArithmeticResponse packs the response of Arithmetic.
type ArithmeticResponse multicastResponse // MARKER: Arithmetic

// Get unpacks the return arguments of Arithmetic.
func (_res *ArithmeticResponse) Get() (xEcho int, opEcho string, yEcho int, result int, err error) { // MARKER: Arithmetic
	_d := _res.data.(*ArithmeticOut)
	return _d.XEcho, _d.OpEcho, _d.YEcho, _d.Result, _res.err
}

/*
Arithmetic performs an arithmetic operation between two integers x and y given an operator op.
*/
func (_c MulticastClient) Arithmetic(ctx context.Context, x int, op string, y int) iter.Seq[*ArithmeticResponse] { // MARKER: Arithmetic
	_in := ArithmeticIn{X: x, Op: op, Y: y}
	_out := ArithmeticOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Arithmetic.Method, Arithmetic.Route, &_in, &_out)
	return func(yield func(*ArithmeticResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ArithmeticResponse)(_r)) {
				return
			}
		}
	}
}

/*
Arithmetic performs an arithmetic operation between two integers x and y given an operator op.
*/
func (_c Client) Arithmetic(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) { // MARKER: Arithmetic
	_in := ArithmeticIn{X: x, Op: op, Y: y}
	_out := ArithmeticOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Arithmetic.Method, Arithmetic.Route, &_in, &_out)
	return _out.XEcho, _out.OpEcho, _out.YEcho, _out.Result, err // No trace
}

// SquareResponse packs the response of Square.
type SquareResponse multicastResponse // MARKER: Square

// Get unpacks the return arguments of Square.
func (_res *SquareResponse) Get() (xEcho int, result int, err error) { // MARKER: Square
	_d := _res.data.(*SquareOut)
	return _d.XEcho, _d.Result, _res.err
}

/*
Square prints the square of the integer x.
*/
func (_c MulticastClient) Square(ctx context.Context, x int) iter.Seq[*SquareResponse] { // MARKER: Square
	_in := SquareIn{X: x}
	_out := SquareOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Square.Method, Square.Route, &_in, &_out)
	return func(yield func(*SquareResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*SquareResponse)(_r)) {
				return
			}
		}
	}
}

/*
Square prints the square of the integer x.
*/
func (_c Client) Square(ctx context.Context, x int) (xEcho int, result int, err error) { // MARKER: Square
	_in := SquareIn{X: x}
	_out := SquareOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Square.Method, Square.Route, &_in, &_out)
	return _out.XEcho, _out.Result, err // No trace
}

// DistanceResponse packs the response of Distance.
type DistanceResponse multicastResponse // MARKER: Distance

// Get unpacks the return arguments of Distance.
func (_res *DistanceResponse) Get() (d float64, err error) { // MARKER: Distance
	_d := _res.data.(*DistanceOut)
	return _d.D, _res.err
}

/*
Distance calculates the distance between two points. It demonstrates the use of the defined type Point.
*/
func (_c MulticastClient) Distance(ctx context.Context, p1 Point, p2 Point) iter.Seq[*DistanceResponse] { // MARKER: Distance
	_in := DistanceIn{P1: p1, P2: p2}
	_out := DistanceOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Distance.Method, Distance.Route, &_in, &_out)
	return func(yield func(*DistanceResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*DistanceResponse)(_r)) {
				return
			}
		}
	}
}

/*
Distance calculates the distance between two points. It demonstrates the use of the defined type Point.
*/
func (_c Client) Distance(ctx context.Context, p1 Point, p2 Point) (d float64, err error) { // MARKER: Distance
	_in := DistanceIn{P1: p1, P2: p2}
	_out := DistanceOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Distance.Method, Distance.Route, &_in, &_out)
	return _out.D, err // No trace
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

package calculatorapi

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
const Hostname = "calculator.example"

// Endpoint routes.
const (
	RouteOfArithmetic = `:443/arithmetic` // MARKER: Arithmetic
	RouteOfSquare     = `:443/square`     // MARKER: Square
	RouteOfDistance    = `:443/distance`   // MARKER: Distance
)

// Endpoint URLs.
var (
	URLOfArithmetic = httpx.JoinHostAndPath(Hostname, RouteOfArithmetic) // MARKER: Arithmetic
	URLOfSquare     = httpx.JoinHostAndPath(Hostname, RouteOfSquare)     // MARKER: Square
	URLOfDistance   = httpx.JoinHostAndPath(Hostname, RouteOfDistance)   // MARKER: Distance
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

// ArithmeticIn are the input arguments of Arithmetic.
type ArithmeticIn struct { // MARKER: Arithmetic
	X  int    `json:"x,omitzero"`
	Op string `json:"op,omitzero"`
	Y  int    `json:"y,omitzero"`
}

// ArithmeticOut are the output arguments of Arithmetic.
type ArithmeticOut struct { // MARKER: Arithmetic
	XEcho  int    `json:"xEcho,omitzero"`
	OpEcho string `json:"opEcho,omitzero"`
	YEcho  int    `json:"yEcho,omitzero"`
	Result int    `json:"result,omitzero"`
}

// ArithmeticResponse is the response to Arithmetic.
type ArithmeticResponse struct { // MARKER: Arithmetic
	data         ArithmeticOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *ArithmeticResponse) Get() (xEcho int, opEcho string, yEcho int, result int, err error) { // MARKER: Arithmetic
	return _res.data.XEcho, _res.data.OpEcho, _res.data.YEcho, _res.data.Result, _res.err
}

/*
Arithmetic performs an arithmetic operation between two integers x and y given an operator op.
*/
func (_c MulticastClient) Arithmetic(ctx context.Context, x int, op string, y int) <-chan *ArithmeticResponse { // MARKER: Arithmetic
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfArithmetic)
	_in := ArithmeticIn{
		X:  x,
		Op: op,
		Y:  y,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *ArithmeticResponse, 1)
		_res <- &ArithmeticResponse{err: _err} // No trace
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
	_res := make(chan *ArithmeticResponse, cap(_ch))
	for _i := range _ch {
		var _r ArithmeticResponse
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
Arithmetic performs an arithmetic operation between two integers x and y given an operator op.
*/
func (_c Client) Arithmetic(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) { // MARKER: Arithmetic
	var _err error
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfArithmetic)
	_in := ArithmeticIn{
		X:  x,
		Op: op,
		Y:  y,
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
	var _out ArithmeticOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.XEcho, _out.OpEcho, _out.YEcho, _out.Result, nil
}

// SquareIn are the input arguments of Square.
type SquareIn struct { // MARKER: Square
	X int `json:"x,omitzero"`
}

// SquareOut are the output arguments of Square.
type SquareOut struct { // MARKER: Square
	XEcho  int `json:"xEcho,omitzero"`
	Result int `json:"result,omitzero"`
}

// SquareResponse is the response to Square.
type SquareResponse struct { // MARKER: Square
	data         SquareOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *SquareResponse) Get() (xEcho int, result int, err error) { // MARKER: Square
	return _res.data.XEcho, _res.data.Result, _res.err
}

/*
Square prints the square of the integer x.
*/
func (_c MulticastClient) Square(ctx context.Context, x int) <-chan *SquareResponse { // MARKER: Square
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSquare)
	_in := SquareIn{
		X: x,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *SquareResponse, 1)
		_res <- &SquareResponse{err: _err} // No trace
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
	_res := make(chan *SquareResponse, cap(_ch))
	for _i := range _ch {
		var _r SquareResponse
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
Square prints the square of the integer x.
*/
func (_c Client) Square(ctx context.Context, x int) (xEcho int, result int, err error) { // MARKER: Square
	var _err error
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfSquare)
	_in := SquareIn{
		X: x,
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
	var _out SquareOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.XEcho, _out.Result, nil
}

// DistanceIn are the input arguments of Distance.
type DistanceIn struct { // MARKER: Distance
	P1 Point `json:"p1,omitzero"`
	P2 Point `json:"p2,omitzero"`
}

// DistanceOut are the output arguments of Distance.
type DistanceOut struct { // MARKER: Distance
	D float64 `json:"d,omitzero"`
}

// DistanceResponse is the response to Distance.
type DistanceResponse struct { // MARKER: Distance
	data         DistanceOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *DistanceResponse) Get() (d float64, err error) { // MARKER: Distance
	return _res.data.D, _res.err
}

/*
Distance calculates the distance between two points.
It demonstrates the use of the defined type Point.
*/
func (_c MulticastClient) Distance(ctx context.Context, p1 Point, p2 Point) <-chan *DistanceResponse { // MARKER: Distance
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfDistance)
	_in := DistanceIn{
		P1: p1,
		P2: p2,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *DistanceResponse, 1)
		_res <- &DistanceResponse{err: _err} // No trace
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
	_res := make(chan *DistanceResponse, cap(_ch))
	for _i := range _ch {
		var _r DistanceResponse
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
Distance calculates the distance between two points.
It demonstrates the use of the defined type Point.
*/
func (_c Client) Distance(ctx context.Context, p1 Point, p2 Point) (d float64, err error) { // MARKER: Distance
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfDistance)
	_in := DistanceIn{
		P1: p1,
		P2: p2,
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
	var _out DistanceOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.D, nil
}

package tokenissuerapi

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
const Hostname = "tokenissuer.core"

// Endpoint routes.
const (
	RouteOfIssueToken    = `:444/issue-token`    // MARKER: IssueToken
	RouteOfValidateToken = `:444/validate-token` // MARKER: ValidateToken
)

// Endpoint URLs.
var (
	URLOfIssueToken    = httpx.JoinHostAndPath(Hostname, RouteOfIssueToken)    // MARKER: IssueToken
	URLOfValidateToken = httpx.JoinHostAndPath(Hostname, RouteOfValidateToken) // MARKER: ValidateToken
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

// IssueTokenIn are the input arguments of IssueToken.
type IssueTokenIn struct { // MARKER: IssueToken
	Claims MapClaims `json:"claims,omitzero"`
}

// IssueTokenOut are the output arguments of IssueToken.
type IssueTokenOut struct { // MARKER: IssueToken
	SignedToken string `json:"signedToken,omitzero"`
}

// IssueTokenResponse is the response to IssueToken.
type IssueTokenResponse struct { // MARKER: IssueToken
	data         IssueTokenOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *IssueTokenResponse) Get() (signedToken string, err error) { // MARKER: IssueToken
	return _res.data.SignedToken, _res.err
}

/*
IssueToken generates a new JWT with a set of claims.
The claims must be provided as a jwt.MapClaims or an object that can be JSON encoded.
See https://www.iana.org/assignments/jwt/jwt.xhtml for a list of the common claim names.
*/
func (_c MulticastClient) IssueToken(ctx context.Context, claims MapClaims) <-chan *IssueTokenResponse { // MARKER: IssueToken
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfIssueToken)
	_in := IssueTokenIn{
		Claims: claims,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *IssueTokenResponse, 1)
		_res <- &IssueTokenResponse{err: _err} // No trace
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
	_res := make(chan *IssueTokenResponse, cap(_ch))
	for _i := range _ch {
		var _r IssueTokenResponse
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
IssueToken generates a new JWT with a set of claims.
The claims must be provided as a jwt.MapClaims or an object that can be JSON encoded.
See https://www.iana.org/assignments/jwt/jwt.xhtml for a list of the common claim names.
*/
func (_c Client) IssueToken(ctx context.Context, claims MapClaims) (signedToken string, err error) { // MARKER: IssueToken
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfIssueToken)
	_in := IssueTokenIn{
		Claims: claims,
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
	var _out IssueTokenOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.SignedToken, nil
}

// ValidateTokenIn are the input arguments of ValidateToken.
type ValidateTokenIn struct { // MARKER: ValidateToken
	SignedToken string `json:"signedToken,omitzero"`
}

// ValidateTokenOut are the output arguments of ValidateToken.
type ValidateTokenOut struct { // MARKER: ValidateToken
	Claims MapClaims `json:"claims,omitzero"`
	Valid  bool      `json:"valid,omitzero"`
}

// ValidateTokenResponse is the response to ValidateToken.
type ValidateTokenResponse struct { // MARKER: ValidateToken
	data         ValidateTokenOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *ValidateTokenResponse) Get() (claims MapClaims, valid bool, err error) { // MARKER: ValidateToken
	return _res.data.Claims, _res.data.Valid, _res.err
}

/*
ValidateToken validates a JWT previously generated by this issuer and returns the claims associated with it.
*/
func (_c MulticastClient) ValidateToken(ctx context.Context, signedToken string) <-chan *ValidateTokenResponse { // MARKER: ValidateToken
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValidateToken)
	_in := ValidateTokenIn{
		SignedToken: signedToken,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *ValidateTokenResponse, 1)
		_res <- &ValidateTokenResponse{err: _err} // No trace
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
	_res := make(chan *ValidateTokenResponse, cap(_ch))
	for _i := range _ch {
		var _r ValidateTokenResponse
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
ValidateToken validates a JWT previously generated by this issuer and returns the claims associated with it.
*/
func (_c Client) ValidateToken(ctx context.Context, signedToken string) (claims MapClaims, valid bool, err error) { // MARKER: ValidateToken
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfValidateToken)
	_in := ValidateTokenIn{
		SignedToken: signedToken,
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
	var _out ValidateTokenOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.Claims, _out.Valid, nil
}

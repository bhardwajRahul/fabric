package directoryapi

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
const Hostname = "directory.example"

// Endpoint routes.
const (
	RouteOfCreate      = `/persons`              // MARKER: Create
	RouteOfLoad        = `/persons/key/{key}`     // MARKER: Load
	RouteOfDelete      = `/persons/key/{key}`     // MARKER: Delete
	RouteOfUpdate      = `/persons/key/{key}`     // MARKER: Update
	RouteOfLoadByEmail = `/persons/email/{email}` // MARKER: LoadByEmail
	RouteOfList        = `/persons`               // MARKER: List
	RouteOfWebUI       = `/web-ui`                // MARKER: WebUI
)

// Endpoint URLs.
var (
	URLOfCreate      = httpx.JoinHostAndPath(Hostname, RouteOfCreate)      // MARKER: Create
	URLOfLoad        = httpx.JoinHostAndPath(Hostname, RouteOfLoad)        // MARKER: Load
	URLOfDelete      = httpx.JoinHostAndPath(Hostname, RouteOfDelete)      // MARKER: Delete
	URLOfUpdate      = httpx.JoinHostAndPath(Hostname, RouteOfUpdate)      // MARKER: Update
	URLOfLoadByEmail = httpx.JoinHostAndPath(Hostname, RouteOfLoadByEmail) // MARKER: LoadByEmail
	URLOfList        = httpx.JoinHostAndPath(Hostname, RouteOfList)        // MARKER: List
	URLOfWebUI       = httpx.JoinHostAndPath(Hostname, RouteOfWebUI)       // MARKER: WebUI
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

// --- Create ---

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	HTTPRequestBody Person `json:"-"`
}

// CreateOut are the return values of Create.
type CreateOut struct { // MARKER: Create
	Key PersonKey `json:"key,omitzero"`
}

// CreateResponse is the response to Create.
type CreateResponse struct { // MARKER: Create
	data         CreateOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *CreateResponse) Get() (key PersonKey, err error) { // MARKER: Create
	return _res.data.Key, _res.err
}

/*
Create registers the person in the directory.
*/
func (_c MulticastClient) Create(ctx context.Context, httpRequestBody Person) <-chan *CreateResponse { // MARKER: Create
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfCreate)
	_in := CreateIn{
		HTTPRequestBody: httpRequestBody,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *CreateResponse, 1)
		_res <- &CreateResponse{err: _err} // No trace
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
	_res := make(chan *CreateResponse, cap(_ch))
	for _i := range _ch {
		var _r CreateResponse
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
Create registers the person in the directory.
*/
func (_c Client) Create(ctx context.Context, httpRequestBody Person) (key PersonKey, err error) { // MARKER: Create
	var _err error
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfCreate)
	_in := CreateIn{
		HTTPRequestBody: httpRequestBody,
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
	var _out CreateOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.Key, nil
}

// --- Load ---

// LoadIn are the input arguments of Load.
type LoadIn struct { // MARKER: Load
	Key PersonKey `json:"key,omitzero"`
}

// LoadOut are the return values of Load.
type LoadOut struct { // MARKER: Load
	HTTPResponseBody Person `json:"httpResponseBody,omitzero"`
}

// LoadResponse is the response to Load.
type LoadResponse struct { // MARKER: Load
	data         LoadOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *LoadResponse) Get() (httpResponseBody Person, err error) { // MARKER: Load
	return _res.data.HTTPResponseBody, _res.err
}

/*
Load looks up a person in the directory.
*/
func (_c MulticastClient) Load(ctx context.Context, key PersonKey) <-chan *LoadResponse { // MARKER: Load
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfLoad)
	_in := LoadIn{
		Key: key,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *LoadResponse, 1)
		_res <- &LoadResponse{err: _err} // No trace
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
	_res := make(chan *LoadResponse, cap(_ch))
	for _i := range _ch {
		var _r LoadResponse
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
Load looks up a person in the directory.
*/
func (_c Client) Load(ctx context.Context, key PersonKey) (httpResponseBody Person, err error) { // MARKER: Load
	var _err error
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfLoad)
	_in := LoadIn{
		Key: key,
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
	var _out LoadOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.HTTPResponseBody, nil
}

// --- Delete ---

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	Key PersonKey `json:"key,omitzero"`
}

// DeleteOut are the return values of Delete.
type DeleteOut struct { // MARKER: Delete
}

// DeleteResponse is the response to Delete.
type DeleteResponse struct { // MARKER: Delete
	data         DeleteOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *DeleteResponse) Get() (err error) { // MARKER: Delete
	return _res.err
}

/*
Delete removes a person from the directory.
*/
func (_c MulticastClient) Delete(ctx context.Context, key PersonKey) <-chan *DeleteResponse { // MARKER: Delete
	_method := "DELETE"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfDelete)
	_in := DeleteIn{
		Key: key,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *DeleteResponse, 1)
		_res <- &DeleteResponse{err: _err} // No trace
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
	_res := make(chan *DeleteResponse, cap(_ch))
	for _i := range _ch {
		var _r DeleteResponse
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
Delete removes a person from the directory.
*/
func (_c Client) Delete(ctx context.Context, key PersonKey) (err error) { // MARKER: Delete
	var _err error
	_method := "DELETE"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfDelete)
	_in := DeleteIn{
		Key: key,
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
	var _out DeleteOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return nil
}

// --- Update ---

// UpdateIn are the input arguments of Update.
type UpdateIn struct { // MARKER: Update
	Key             PersonKey `json:"key,omitzero"`
	HTTPRequestBody Person    `json:"-"`
}

// UpdateOut are the return values of Update.
type UpdateOut struct { // MARKER: Update
}

// UpdateResponse is the response to Update.
type UpdateResponse struct { // MARKER: Update
	data         UpdateOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *UpdateResponse) Get() (err error) { // MARKER: Update
	return _res.err
}

/*
Update updates the person's data in the directory.
*/
func (_c MulticastClient) Update(ctx context.Context, key PersonKey, httpRequestBody Person) <-chan *UpdateResponse { // MARKER: Update
	_method := "PUT"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfUpdate)
	_in := UpdateIn{
		Key:             key,
		HTTPRequestBody: httpRequestBody,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *UpdateResponse, 1)
		_res <- &UpdateResponse{err: _err} // No trace
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
	_res := make(chan *UpdateResponse, cap(_ch))
	for _i := range _ch {
		var _r UpdateResponse
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
Update updates the person's data in the directory.
*/
func (_c Client) Update(ctx context.Context, key PersonKey, httpRequestBody Person) (err error) { // MARKER: Update
	var _err error
	_method := "PUT"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfUpdate)
	_in := UpdateIn{
		Key:             key,
		HTTPRequestBody: httpRequestBody,
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
	var _out UpdateOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return nil
}

// --- LoadByEmail ---

// LoadByEmailIn are the input arguments of LoadByEmail.
type LoadByEmailIn struct { // MARKER: LoadByEmail
	Email string `json:"email,omitzero"`
}

// LoadByEmailOut are the return values of LoadByEmail.
type LoadByEmailOut struct { // MARKER: LoadByEmail
	HTTPResponseBody Person `json:"httpResponseBody,omitzero"`
}

// LoadByEmailResponse is the response to LoadByEmail.
type LoadByEmailResponse struct { // MARKER: LoadByEmail
	data         LoadByEmailOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *LoadByEmailResponse) Get() (httpResponseBody Person, err error) { // MARKER: LoadByEmail
	return _res.data.HTTPResponseBody, _res.err
}

/*
LoadByEmail looks up a person in the directory by their email.
*/
func (_c MulticastClient) LoadByEmail(ctx context.Context, email string) <-chan *LoadByEmailResponse { // MARKER: LoadByEmail
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfLoadByEmail)
	_in := LoadByEmailIn{
		Email: email,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *LoadByEmailResponse, 1)
		_res <- &LoadByEmailResponse{err: _err} // No trace
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
	_res := make(chan *LoadByEmailResponse, cap(_ch))
	for _i := range _ch {
		var _r LoadByEmailResponse
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
LoadByEmail looks up a person in the directory by their email.
*/
func (_c Client) LoadByEmail(ctx context.Context, email string) (httpResponseBody Person, err error) { // MARKER: LoadByEmail
	var _err error
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfLoadByEmail)
	_in := LoadByEmailIn{
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
	var _out LoadByEmailOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.HTTPResponseBody, nil
}

// --- List ---

// ListIn are the input arguments of List.
type ListIn struct { // MARKER: List
}

// ListOut are the return values of List.
type ListOut struct { // MARKER: List
	HTTPResponseBody []PersonKey `json:"httpResponseBody,omitzero"`
}

// ListResponse is the response to List.
type ListResponse struct { // MARKER: List
	data         ListOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *ListResponse) Get() (httpResponseBody []PersonKey, err error) { // MARKER: List
	return _res.data.HTTPResponseBody, _res.err
}

/*
List returns the keys of all the persons in the directory.
*/
func (_c MulticastClient) List(ctx context.Context) <-chan *ListResponse { // MARKER: List
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfList)
	_in := ListIn{}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *ListResponse, 1)
		_res <- &ListResponse{err: _err} // No trace
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
	_res := make(chan *ListResponse, cap(_ch))
	for _i := range _ch {
		var _r ListResponse
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
List returns the keys of all the persons in the directory.
*/
func (_c Client) List(ctx context.Context) (httpResponseBody []PersonKey, err error) { // MARKER: List
	var _err error
	_method := "GET"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfList)
	_in := ListIn{}
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
	var _out ListOut
	_err = httpx.ReadOutputPayload(_httpRes, &_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return _out.HTTPResponseBody, nil
}

// --- WebUI ---

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) WebUI(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: WebUI
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWebUI)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) WebUI(ctx context.Context, method string, relativeURL string, body any) <-chan *pub.Response { // MARKER: WebUI
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWebUI)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

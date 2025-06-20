/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

// Code generated by Microbus. DO NOT EDIT.

/*
Package directoryapi implements the public API of the directory.example microservice,
including clients and data structures.

The directory microservice exposes a RESTful API for persisting personal records in a SQL database.
*/
package directoryapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
)

var (
	_ context.Context
	_ *json.Decoder
	_ io.Reader
	_ *http.Request
	_ *url.URL
	_ strings.Reader
	_ time.Duration
	_ *errors.TracedError
	_ *httpx.BodyReader
	_ pub.Option
)

// Hostname is the default hostname of the microservice: directory.example.
const Hostname = "directory.example"

// Fully-qualified URLs of the microservice's endpoints.
var (
	URLOfCreate = httpx.JoinHostAndPath(Hostname, `:443/persons`)
	URLOfLoad = httpx.JoinHostAndPath(Hostname, `:443/persons/key/{key}`)
	URLOfDelete = httpx.JoinHostAndPath(Hostname, `:443/persons/key/{key}`)
	URLOfUpdate = httpx.JoinHostAndPath(Hostname, `:443/persons/key/{key}`)
	URLOfLoadByEmail = httpx.JoinHostAndPath(Hostname, `:443/persons/email/{email}`)
	URLOfList = httpx.JoinHostAndPath(Hostname, `:443/persons`)
	URLOfWebUI = httpx.JoinHostAndPath(Hostname, `:443/web-ui`)
)

// Client is an interface to calling the endpoints of the directory.example microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client to the directory.example microservice.
func NewClient(caller service.Publisher) *Client {
	return &Client{
		svc:  caller,
		host: "directory.example",
	}
}

// ForHost replaces the default hostname of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// WithOptions applies options to requests made by this client.
func (_c *Client) WithOptions(opts ...pub.Option) *Client {
	_c.opts = append(_c.opts, opts...)
	return _c
}

// MulticastClient is an interface to calling the endpoints of the directory.example microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client to the directory.example microservice.
func NewMulticastClient(caller service.Publisher) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "directory.example",
	}
}

// ForHost replaces the default hostname of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

// WithOptions applies options to requests made by this client.
func (_c *MulticastClient) WithOptions(opts ...pub.Option) *MulticastClient {
	_c.opts = append(_c.opts, opts...)
	return _c
}

// errChan returns a response channel with a single error response.
func (_c *MulticastClient) errChan(err error) <-chan *pub.Response {
	ch := make(chan *pub.Response, 1)
	ch <- pub.NewErrorResponse(err)
	close(ch)
	return ch
}

/*
WebUI_Get performs a GET request to the WebUI endpoint.

WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func (_c *Client) WebUI_Get(ctx context.Context, url string) (res *http.Response, err error) {
	url, err = httpx.ResolveURL(URLOfWebUI, url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(
		ctx,
		pub.Method("GET"),
		pub.URL(url),
		pub.Options(_c.opts...),
	)
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
WebUI_Get performs a GET request to the WebUI endpoint.

WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func (_c *MulticastClient) WebUI_Get(ctx context.Context, url string) <-chan *pub.Response {
	var err error
	url, err = httpx.ResolveURL(URLOfWebUI, url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(
		ctx,
		pub.Method("GET"),
		pub.URL(url),
		pub.Options(_c.opts...),
	)
}

/*
WebUI_Post performs a POST request to the WebUI endpoint.

WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func (_c *Client) WebUI_Post(ctx context.Context, url string, contentType string, body any) (res *http.Response, err error) {
	url, err = httpx.ResolveURL(URLOfWebUI, url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(
		ctx,
		pub.Method("POST"),
		pub.URL(url),
		pub.ContentType(contentType),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
WebUI_Post performs a POST request to the WebUI endpoint.

WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func (_c *MulticastClient) WebUI_Post(ctx context.Context, url string, contentType string, body any) <-chan *pub.Response {
	var err error
	url, err = httpx.ResolveURL(URLOfWebUI, url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(
		ctx,
		pub.Method("POST"),
		pub.URL(url),
		pub.ContentType(contentType),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a request is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func (_c *Client) WebUI(r *http.Request) (res *http.Response, err error) {
	if r == nil {
		r, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	url, err := httpx.ResolveURL(URLOfWebUI, r.URL.String())
	if err != nil {
		return nil, errors.Trace(err)
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(
		r.Context(),
		pub.Method(r.Method),
		pub.URL(url),
		pub.CopyHeaders(r.Header),
		pub.Body(r.Body),
		pub.Options(_c.opts...),
	)
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a request is not provided, it defaults to the URL of the endpoint. Otherwise, it is resolved relative to the URL of the endpoint.
*/
func (_c *MulticastClient) WebUI(ctx context.Context, r *http.Request) <-chan *pub.Response {
	var err error
	if r == nil {
		r, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			return _c.errChan(errors.Trace(err))
		}
	}
	url, err := httpx.ResolveURL(URLOfWebUI, r.URL.String())
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	url, err = httpx.FillPathArguments(url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(r.Method),
		pub.URL(url),
		pub.CopyHeaders(r.Header),
		pub.Body(r.Body),
		pub.Options(_c.opts...),
	)
}

// CreateIn are the input arguments of Create.
type CreateIn struct {
	HTTPRequestBody *Person `json:"-"`
}

// CreateOut are the return values of Create.
type CreateOut struct {
	Key PersonKey `json:"key"`
}

// CreateResponse is the response to Create.
type CreateResponse struct {
	data CreateOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *CreateResponse) Get() (key PersonKey, err error) {
	key = _out.data.Key
	err = _out.err
	return
}

/*
Create registers the person in the directory.
*/
func (_c *MulticastClient) Create(ctx context.Context, httpRequestBody *Person) <-chan *CreateResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
	})
	_in := CreateIn{
		httpRequestBody,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *CreateResponse, 1)
		_res <- &CreateResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_body := httpRequestBody
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`POST`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data))
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
func (_c *Client) Create(ctx context.Context, httpRequestBody *Person) (key PersonKey, err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
	})
	_in := CreateIn{
		httpRequestBody,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	_body := httpRequestBody
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`POST`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	key = _out.Key
	return
}

// LoadIn are the input arguments of Load.
type LoadIn struct {
	Key PersonKey `json:"key"`
}

// LoadOut are the return values of Load.
type LoadOut struct {
	HTTPResponseBody *Person `json:"httpResponseBody"`
}

// LoadResponse is the response to Load.
type LoadResponse struct {
	data LoadOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *LoadResponse) Get() (httpResponseBody *Person, err error) {
	httpResponseBody = _out.data.HTTPResponseBody
	err = _out.err
	return
}

/*
Load looks up a person in the directory.
*/
func (_c *MulticastClient) Load(ctx context.Context, key PersonKey) <-chan *LoadResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := LoadIn{
		key,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *LoadResponse, 1)
		_res <- &LoadResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	var _body any
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`GET`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data.HTTPResponseBody))
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
func (_c *Client) Load(ctx context.Context, key PersonKey) (httpResponseBody *Person, err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := LoadIn{
		key,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _body any
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`GET`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out.HTTPResponseBody)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	httpResponseBody = _out.HTTPResponseBody
	return
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct {
	Key PersonKey `json:"key"`
}

// DeleteOut are the return values of Delete.
type DeleteOut struct {
}

// DeleteResponse is the response to Delete.
type DeleteResponse struct {
	data DeleteOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *DeleteResponse) Get() (err error) {
	err = _out.err
	return
}

/*
Delete removes a person from the directory.
*/
func (_c *MulticastClient) Delete(ctx context.Context, key PersonKey) <-chan *DeleteResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := DeleteIn{
		key,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *DeleteResponse, 1)
		_res <- &DeleteResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	var _body any
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`DELETE`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data))
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
func (_c *Client) Delete(ctx context.Context, key PersonKey) (err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := DeleteIn{
		key,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _body any
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`DELETE`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

// UpdateIn are the input arguments of Update.
type UpdateIn struct {
	Key PersonKey `json:"key"`
	HTTPRequestBody *Person `json:"-"`
}

// UpdateOut are the return values of Update.
type UpdateOut struct {
}

// UpdateResponse is the response to Update.
type UpdateResponse struct {
	data UpdateOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *UpdateResponse) Get() (err error) {
	err = _out.err
	return
}

/*
Update updates the person's data in the directory.
*/
func (_c *MulticastClient) Update(ctx context.Context, key PersonKey, httpRequestBody *Person) <-chan *UpdateResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := UpdateIn{
		key,
		httpRequestBody,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *UpdateResponse, 1)
		_res <- &UpdateResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	_body := httpRequestBody
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`PUT`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data))
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
func (_c *Client) Update(ctx context.Context, key PersonKey, httpRequestBody *Person) (err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/key/{key}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`key`: key,
	})
	_in := UpdateIn{
		key,
		httpRequestBody,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	_body := httpRequestBody
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`PUT`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

// LoadByEmailIn are the input arguments of LoadByEmail.
type LoadByEmailIn struct {
	Email string `json:"email"`
}

// LoadByEmailOut are the return values of LoadByEmail.
type LoadByEmailOut struct {
	HTTPResponseBody *Person `json:"httpResponseBody"`
}

// LoadByEmailResponse is the response to LoadByEmail.
type LoadByEmailResponse struct {
	data LoadByEmailOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *LoadByEmailResponse) Get() (httpResponseBody *Person, err error) {
	httpResponseBody = _out.data.HTTPResponseBody
	err = _out.err
	return
}

/*
LoadByEmail looks up a person in the directory by their email.
*/
func (_c *MulticastClient) LoadByEmail(ctx context.Context, email string) <-chan *LoadByEmailResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/email/{email}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`email`: email,
	})
	_in := LoadByEmailIn{
		email,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *LoadByEmailResponse, 1)
		_res <- &LoadByEmailResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	var _body any
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`GET`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data.HTTPResponseBody))
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
func (_c *Client) LoadByEmail(ctx context.Context, email string) (httpResponseBody *Person, err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons/email/{email}`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
		`email`: email,
	})
	_in := LoadByEmailIn{
		email,
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _body any
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`GET`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out.HTTPResponseBody)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	httpResponseBody = _out.HTTPResponseBody
	return
}

// ListIn are the input arguments of List.
type ListIn struct {
}

// ListOut are the return values of List.
type ListOut struct {
	HTTPResponseBody []PersonKey `json:"httpResponseBody"`
}

// ListResponse is the response to List.
type ListResponse struct {
	data ListOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *ListResponse) Get() (httpResponseBody []PersonKey, err error) {
	httpResponseBody = _out.data.HTTPResponseBody
	err = _out.err
	return
}

/*
List returns the keys of all the persons in the directory.
*/
func (_c *MulticastClient) List(ctx context.Context) <-chan *ListResponse {
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
	})
	_in := ListIn{
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		_res := make(chan *ListResponse, 1)
		_res <- &ListResponse{err: _err} // No trace
		close(_res)
		return _res
	}
	var _body any
	_ch := _c.svc.Publish(
		ctx,
		pub.Method(`GET`),
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
			_err = json.NewDecoder(_httpRes.Body).Decode(&(_r.data.HTTPResponseBody))
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
func (_c *Client) List(ctx context.Context) (httpResponseBody []PersonKey, err error) {
	var _err error
	_url := httpx.JoinHostAndPath(_c.host, `:443/persons`)
	_url = httpx.InsertPathArguments(_url, httpx.QArgs{
	})
	_in := ListIn{
	}
	_query, _err := httpx.EncodeDeepObject(_in)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _body any
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(`GET`),
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
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out.HTTPResponseBody)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	httpResponseBody = _out.HTTPResponseBody
	return
}

/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package eventsinkapi implements the public API of the eventsink.example microservice,
including clients and data structures.

The event sink microservice handles events that are fired by the event source microservice.
*/
package eventsinkapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
)

var (
	_ context.Context
	_ *json.Decoder
	_ *http.Request
	_ strings.Reader
	_ time.Duration
	_ *errors.TracedError
	_ *httpx.BodyReader
	_ pub.Option
	_ sub.Option
)

// HostName is the default host name of the microservice: eventsink.example.
const HostName = "eventsink.example"

// Fully-qualified URLs of the microservice's endpoints.
var (
	URLOfRegistered = httpx.JoinHostAndPath(HostName, ":443/registered")
)

// Service is an interface abstraction of a microservice used by the client.
// The connector implements this interface.
type Service interface {
	Request(ctx context.Context, options ...pub.Option) (*http.Response, error)
	Publish(ctx context.Context, options ...pub.Option) <-chan *pub.Response
	Subscribe(method string, path string, handler sub.HTTPHandler, options ...sub.Option) error
	Unsubscribe(method string, path string) error
}

// Client is an interface to calling the endpoints of the eventsink.example microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  Service
	host string
}

// NewClient creates a new unicast client to the eventsink.example microservice.
func NewClient(caller Service) *Client {
	return &Client{
		svc:  caller,
		host: "eventsink.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// MulticastClient is an interface to calling the endpoints of the eventsink.example microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  Service
	host string
}

// NewMulticastClient creates a new multicast client to the eventsink.example microservice.
func NewMulticastClient(caller Service) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "eventsink.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

// RegisteredIn are the input arguments of Registered.
type RegisteredIn struct {
}

// RegisteredOut are the return values of Registered.
type RegisteredOut struct {
	Emails []string `json:"emails"`
}

// RegisteredResponse is the response to Registered.
type RegisteredResponse struct {
	data RegisteredOut
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *RegisteredResponse) Get() (emails []string, err error) {
	emails = _out.data.Emails
	err = _out.err
	return
}

/*
Registered returns the list of registered users.
*/
func (_c *MulticastClient) Registered(ctx context.Context, _options ...pub.Option) <-chan *RegisteredResponse {
	method := `*`
	if method == "*" {
		method = "POST"
	}
	_in := RegisteredIn{
	}
	_opts := []pub.Option{
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/registered`)),
		pub.Body(_in),
	}
	_opts = append(_opts, _options...)
	_ch := _c.svc.Publish(ctx, _opts...)

	_res := make(chan *RegisteredResponse, cap(_ch))
	go func() {
		for _i := range _ch {
			var _r RegisteredResponse
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
	}()
	return _res
}

/*
Registered returns the list of registered users.
*/
func (_c *Client) Registered(ctx context.Context) (emails []string, err error) {
	method := `*`
	if method == "" || method == "*" {
		method = "POST"
	}
	_in := RegisteredIn{
	}
	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/registered`)),
		pub.Body(_in),
	)
	if _err != nil {
		err = _err // No trace
		return
	}
	var _out RegisteredOut
	_err = json.NewDecoder(_httpRes.Body).Decode(&_out)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	emails = _out.Emails
	return
}

/*
Copyright 2023 Microbus LLC and various contributors

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
Package messagingapi implements the public API of the messaging.example microservice,
including clients and data structures.

The Messaging microservice demonstrates service-to-service communication patterns.
*/
package messagingapi

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

// The default host name addressed by the clients is messaging.example.
const HostName = "messaging.example"

// Service is an interface abstraction of a microservice used by the client.
// The connector implements this interface.
type Service interface {
	Request(ctx context.Context, options ...pub.Option) (*http.Response, error)
	Publish(ctx context.Context, options ...pub.Option) <-chan *pub.Response
	Subscribe(path string, handler sub.HTTPHandler, options ...sub.Option) error
	Unsubscribe(path string) error
}

// Client is an interface to calling the endpoints of the messaging.example microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  Service
	host string
}

// NewClient creates a new unicast client to the messaging.example microservice.
func NewClient(caller Service) *Client {
	return &Client{
		svc:  caller,
		host: "messaging.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// MulticastClient is an interface to calling the endpoints of the messaging.example microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  Service
	host string
}

// NewMulticastClient creates a new multicast client to the messaging.example microservice.
func NewMulticastClient(caller Service) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "messaging.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

/*
Home demonstrates making requests using multicast and unicast request/response patterns.
*/
func (_c *Client) Home(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/home`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return res, err
}

/*
Home demonstrates making requests using multicast and unicast request/response patterns.
*/
func (_c *MulticastClient) Home(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/home`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}

/*
NoQueue demonstrates how the NoQueue subscription option is used to create
a multicast request/response communication pattern.
All instances of this microservice will respond to each request.
*/
func (_c *Client) NoQueue(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/no-queue`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return res, err
}

/*
NoQueue demonstrates how the NoQueue subscription option is used to create
a multicast request/response communication pattern.
All instances of this microservice will respond to each request.
*/
func (_c *MulticastClient) NoQueue(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/no-queue`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}

/*
DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
a unicast request/response communication pattern.
Only one of the instances of this microservice will respond to each request.
*/
func (_c *Client) DefaultQueue(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/default-queue`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return res, err
}

/*
DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
a unicast request/response communication pattern.
Only one of the instances of this microservice will respond to each request.
*/
func (_c *MulticastClient) DefaultQueue(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/default-queue`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}

/*
CacheLoad looks up an element in the distributed cache of the microservice.
*/
func (_c *Client) CacheLoad(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/cache-load`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return res, err
}

/*
CacheLoad looks up an element in the distributed cache of the microservice.
*/
func (_c *MulticastClient) CacheLoad(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/cache-load`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}

/*
CacheStore stores an element in the distributed cache of the microservice.
*/
func (_c *Client) CacheStore(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/cache-store`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return res, err
}

/*
CacheStore stores an element in the distributed cache of the microservice.
*/
func (_c *MulticastClient) CacheStore(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/cache-store`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}
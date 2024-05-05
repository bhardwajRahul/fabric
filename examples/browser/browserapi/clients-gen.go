/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package browserapi implements the public API of the browser.example microservice,
including clients and data structures.

The browser microservice implements a simple web browser that utilizes the egress proxy.
*/
package browserapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
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

// HostName is the default host name of the microservice: browser.example.
const HostName = "browser.example"

// Fully-qualified URLs of the microservice's endpoints.
var (
	URLOfBrowse = httpx.JoinHostAndPath(HostName, ":443/browse")
)

// Client is an interface to calling the endpoints of the browser.example microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  service.PublisherSubscriber
	host string
}

// NewClient creates a new unicast client to the browser.example microservice.
func NewClient(caller service.PublisherSubscriber) *Client {
	return &Client{
		svc:  caller,
		host: "browser.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// MulticastClient is an interface to calling the endpoints of the browser.example microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  service.PublisherSubscriber
	host string
}

// NewMulticastClient creates a new multicast client to the browser.example microservice.
func NewMulticastClient(caller service.PublisherSubscriber) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "browser.example",
	}
}

// ForHost replaces the default host name of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

/*
Browser shows a simple address bar and the source code of a URL.
*/
func (_c *Client) Browse(ctx context.Context, options ...pub.Option) (res *http.Response, err error) {
	method := `*`
	if method == "*" {
		method = "GET"
	}
	opts := []pub.Option{
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/browse`)),
	}
	opts = append(opts, options...)
	res, err = _c.svc.Request(ctx, opts...)
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
Browser shows a simple address bar and the source code of a URL.
*/
func (_c *MulticastClient) Browse(ctx context.Context, options ...pub.Option) <-chan *pub.Response {
	method := `*`
	if method == "*" {
		method = "GET"
	}
	opts := []pub.Option{
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, `:443/browse`)),
	}
	opts = append(opts, options...)
	return _c.svc.Publish(ctx, opts...)
}

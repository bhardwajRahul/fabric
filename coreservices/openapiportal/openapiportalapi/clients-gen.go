/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package openapiportalapi implements the public API of the openapiportal.sys microservice,
including clients and data structures.

The OpenAPI microservice lists links to the OpenAPI endpoint of all microservices that provide one
on the requested port.
*/
package openapiportalapi

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
	"github.com/microbus-io/fabric/sub"
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
	_ sub.Option
)

// HostName is the default host name of the microservice: openapiportal.sys.
const HostName = "openapiportal.sys"

// Fully-qualified URLs of the microservice's endpoints.
var (
	URLOfList = httpx.JoinHostAndPath(HostName, `//openapi:*`)
)

// Client is an interface to calling the endpoints of the openapiportal.sys microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  service.Publisher
	host string
}

// NewClient creates a new unicast client to the openapiportal.sys microservice.
func NewClient(caller service.Publisher) *Client {
	return &Client{
		svc:  caller,
		host: "openapiportal.sys",
	}
}

// ForHost replaces the default host name of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// MulticastClient is an interface to calling the endpoints of the openapiportal.sys microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  service.Publisher
	host string
}

// NewMulticastClient creates a new multicast client to the openapiportal.sys microservice.
func NewMulticastClient(caller service.Publisher) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "openapiportal.sys",
	}
}

// ForHost replaces the default host name of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

// resolveURL resolves a URL in relation to the endpoint's base path.
func (_c *Client) resolveURL(base string, relative string) (resolved string, err error) {
	if relative == "" {
		return base, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", errors.Trace(err)
	}
	relativeURL, err := url.Parse(relative)
	if err != nil {
		return "", errors.Trace(err)
	}
	resolvedURL := baseURL.ResolveReference(relativeURL)
	return resolvedURL.String(), nil
}

// resolveURL resolves a URL in relation to the endpoint's base path.
func (_c *MulticastClient) resolveURL(base string, relative string) (resolved string, err error) {
	if relative == "" {
		return base, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", errors.Trace(err)
	}
	relativeURL, err := url.Parse(relative)
	if err != nil {
		return "", errors.Trace(err)
	}
	resolvedURL := baseURL.ResolveReference(relativeURL)
	return resolvedURL.String(), nil
}

// errChan returns a response channel with a single error response.
func (_c *MulticastClient) errChan(err error) <-chan *pub.Response {
	ch := make(chan *pub.Response, 1)
	ch <- pub.NewErrorResponse(err)
	close(ch)
	return ch
}

/*
ListGet performs a GET request to the List endpoint.

List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a URL is not provided, it defaults to //openapi:* .
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
*/
func (_c *Client) ListGet(ctx context.Context, url string) (res *http.Response, err error) {
	url, err = _c.resolveURL(URLOfList, url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(ctx, pub.Method("GET"), pub.URL(url))
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
ListGet performs a GET request to the List endpoint.

List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a URL is not provided, it defaults to //openapi:* .
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
*/
func (_c *MulticastClient) ListGet(ctx context.Context, url string) <-chan *pub.Response {
	var err error
	url, err = _c.resolveURL(URLOfList, url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(ctx, pub.Method("GET"), pub.URL(url))
}

/*
ListPost performs a POST request to the List endpoint.

List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a URL is not provided, it defaults to //openapi:* .
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func (_c *Client) ListPost(ctx context.Context, url string, contentType string, body any) (res *http.Response, err error) {
	url, err = _c.resolveURL(URLOfList, url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(ctx, pub.Method("POST"), pub.URL(url), pub.ContentType(contentType), pub.Body(body))
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
ListPost performs a POST request to the List endpoint.

List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a URL is not provided, it defaults to //openapi:* .
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
If the body if of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
If a content type is not explicitly provided, an attempt will be made to derive it from the body.
*/
func (_c *MulticastClient) ListPost(ctx context.Context, url string, contentType string, body any) <-chan *pub.Response {
	var err error
	url, err = _c.resolveURL(URLOfList, url)
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(ctx, pub.Method("POST"), pub.URL(url), pub.ContentType(contentType), pub.Body(body))
}

/*
List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a request is not provided, it defaults to GET //openapi:*
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
*/
func (_c *Client) List(ctx context.Context, httpReq *http.Request) (res *http.Response, err error) {
	if httpReq == nil {
		httpReq, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	url, err := _c.resolveURL(URLOfList, httpReq.URL.String())
	if err != nil {
		return nil, errors.Trace(err)
	}
	res, err = _c.svc.Request(ctx, pub.Method(httpReq.Method), pub.URL(url), pub.CopyHeaders(httpReq), pub.Body(httpReq.Body))
	if err != nil {
		return nil, err // No trace
	}
	return res, err
}

/*
List displays links to the OpenAPI endpoint of all microservices that provide one on the request's port.

If a request is not provided, it defaults to GET //openapi:*
Otherwise, the request's URL is resolved relative to the URL of the endpoint.
*/
func (_c *MulticastClient) List(ctx context.Context, httpReq *http.Request) <-chan *pub.Response {
	var err error
	if httpReq == nil {
		httpReq, err = http.NewRequest(`GET`, "", nil)
		if err != nil {
			return _c.errChan(errors.Trace(err))
		}
	}
	url, err := _c.resolveURL(URLOfList, httpReq.URL.String())
	if err != nil {
		return _c.errChan(errors.Trace(err))
	}
	return _c.svc.Publish(ctx, pub.Method(httpReq.Method), pub.URL(url), pub.CopyHeaders(httpReq), pub.Body(httpReq.Body))
}

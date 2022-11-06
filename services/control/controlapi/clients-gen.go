// Code generated by Microbus. DO NOT EDIT.

package controlapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
)

var (
	_ context.Context
	_ json.Decoder
	_ http.Request
	_ strings.Reader
	_ time.Duration

	_ errors.TracedError
	_ pub.Request
	_ sub.Subscription
)

const ServiceName = "control.sys"

// Service is an interface abstraction of a microservice used by the client.
// The connector implements this interface.
type Service interface {
	Request(ctx context.Context, options ...pub.Option) (*http.Response, error)
	Publish(ctx context.Context, options ...pub.Option) <-chan *pub.Response
}

// Client provides type-safe access to the endpoints of the control.sys microservice.
// This simple version is for unicast calls.
type Client struct {
	svc  Service
	host string
}

// NewClient creates a new unicast client to the control.sys microservice.
func NewClient(caller Service) *Client {
	return &Client{
		svc:  caller,
		host: "control.sys",
	}
}

// ForHost replaces the default host name of this client.
func (_c *Client) ForHost(host string) *Client {
	_c.host = host
	return _c
}

// MulticastClient provides type-safe access to the endpoints of the control.sys microservice.
// This advanced version is for multicast calls.
type MulticastClient struct {
	svc  Service
	host string
}

// NewMulticastClient creates a new multicast client to the control.sys microservice.
func NewMulticastClient(caller Service) *MulticastClient {
	return &MulticastClient{
		svc:  caller,
		host: "control.sys",
	}
}

// ForHost replaces the default host name of this client.
func (_c *MulticastClient) ForHost(host string) *MulticastClient {
	_c.host = host
	return _c
}

// PingIn are the incoming arguments to Ping.
type PingIn struct {
}

// PingOut are the return values of Ping.
type PingOut struct {
	data struct {
		Pong int `json:"pong"`
	}
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *PingOut) Get() (pong int, err error) {
	pong = _out.data.Pong
	err = _out.err
	return
}

/*
Ping responds to the message with a pong.
*/
func (_c *Client) Ping(ctx context.Context) (pong int, err error) {
	_in := PingIn{
	}
	_body, _err := json.Marshal(_in)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}

	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method("POST"),
		pub.URL(sub.JoinHostAndPath(_c.host, `:888/ping`)),
		pub.Body(_body),
		pub.Header("Content-Type", "application/json"),
	)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	var _out PingOut
	_err = json.NewDecoder(_httpRes.Body).Decode(&(_out.data))
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	pong = _out.data.Pong
	return
}

/*
Ping responds to the message with a pong.
*/
func (_c *MulticastClient) Ping(ctx context.Context, _options ...pub.Option) <-chan *PingOut {
	_in := PingIn{
	}
	_body, _err := json.Marshal(_in)
	if _err != nil {
		_res := make(chan *PingOut, 1)
		_res <- &PingOut{err: errors.Trace(_err)}
		close(_res)
		return _res
	}

	_opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(sub.JoinHostAndPath(_c.host, `:888/ping`)),
		pub.Body(_body),
		pub.Header("Content-Type", "application/json"),
	}
	_opts = append(_opts, _options...)
	_ch := _c.svc.Publish(ctx, _opts...)

	_res := make(chan *PingOut, cap(_ch))
	go func() {
		for _i := range _ch {
			var _r PingOut
			_httpRes, _err := _i.Get()
			_r.HTTPResponse = _httpRes
			if _err != nil {
				_r.err = errors.Trace(_err)
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

// ConfigRefreshIn are the incoming arguments to ConfigRefresh.
type ConfigRefreshIn struct {
}

// ConfigRefreshOut are the return values of ConfigRefresh.
type ConfigRefreshOut struct {
	data struct {
	}
	HTTPResponse *http.Response
	err error
}

// Get retrieves the return values.
func (_out *ConfigRefreshOut) Get() (err error) {
	err = _out.err
	return
}

/*
ConfigRefresh pulls the latest config values from the configurator service.
*/
func (_c *Client) ConfigRefresh(ctx context.Context) (err error) {
	_in := ConfigRefreshIn{
	}
	_body, _err := json.Marshal(_in)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}

	_httpRes, _err := _c.svc.Request(
		ctx,
		pub.Method("POST"),
		pub.URL(sub.JoinHostAndPath(_c.host, `:888/config-refresh`)),
		pub.Body(_body),
		pub.Header("Content-Type", "application/json"),
	)
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	var _out ConfigRefreshOut
	_err = json.NewDecoder(_httpRes.Body).Decode(&(_out.data))
	if _err != nil {
		err = errors.Trace(_err)
		return
	}
	return
}

/*
ConfigRefresh pulls the latest config values from the configurator service.
*/
func (_c *MulticastClient) ConfigRefresh(ctx context.Context, _options ...pub.Option) <-chan *ConfigRefreshOut {
	_in := ConfigRefreshIn{
	}
	_body, _err := json.Marshal(_in)
	if _err != nil {
		_res := make(chan *ConfigRefreshOut, 1)
		_res <- &ConfigRefreshOut{err: errors.Trace(_err)}
		close(_res)
		return _res
	}

	_opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(sub.JoinHostAndPath(_c.host, `:888/config-refresh`)),
		pub.Body(_body),
		pub.Header("Content-Type", "application/json"),
	}
	_opts = append(_opts, _options...)
	_ch := _c.svc.Publish(ctx, _opts...)

	_res := make(chan *ConfigRefreshOut, cap(_ch))
	go func() {
		for _i := range _ch {
			var _r ConfigRefreshOut
			_httpRes, _err := _i.Get()
			_r.HTTPResponse = _httpRes
			if _err != nil {
				_r.err = errors.Trace(_err)
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

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

package smtpingressapi

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
const Hostname = "smtp.ingress.core"

// Endpoint routes.
const (
	RouteOfOnIncomingEmail = `:417/on-incoming-email` // MARKER: OnIncomingEmail
)

// Endpoint URLs.
var (
	URLOfOnIncomingEmail = httpx.JoinHostAndPath(Hostname, RouteOfOnIncomingEmail) // MARKER: OnIncomingEmail
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

// OnIncomingEmailIn are the input arguments of OnIncomingEmail.
type OnIncomingEmailIn struct { // MARKER: OnIncomingEmail
	MailMessage *Email `json:"mailMessage,omitzero"`
}

// OnIncomingEmailOut are the output arguments of OnIncomingEmail.
type OnIncomingEmailOut struct { // MARKER: OnIncomingEmail
}

// OnIncomingEmailResponse is the response to OnIncomingEmail.
type OnIncomingEmailResponse struct { // MARKER: OnIncomingEmail
	data         OnIncomingEmailOut
	HTTPResponse *http.Response
	err          error
}

// Get retrieves the return values.
func (_res *OnIncomingEmailResponse) Get() (err error) { // MARKER: OnIncomingEmail
	return _res.err
}

/*
OnIncomingEmail is triggered when a new email message is received.
*/
func (_c MulticastTrigger) OnIncomingEmail(ctx context.Context, mailMessage *Email) <-chan *OnIncomingEmailResponse { // MARKER: OnIncomingEmail
	_method := "POST"
	_url := httpx.JoinHostAndPath(_c.host, RouteOfOnIncomingEmail)
	_in := OnIncomingEmailIn{
		MailMessage: mailMessage,
	}
	_query, _body, _err := httpx.WriteInputPayload(_method, _in)
	if _err != nil {
		_res := make(chan *OnIncomingEmailResponse, 1)
		_res <- &OnIncomingEmailResponse{err: _err} // No trace
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
	_res := make(chan *OnIncomingEmailResponse, cap(_ch))
	for _i := range _ch {
		var _r OnIncomingEmailResponse
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
OnIncomingEmail is triggered when a new email message is received.
*/
func (c Hook) OnIncomingEmail(handler func(ctx context.Context, mailMessage *Email) (err error)) (unsub func() error, err error) { // MARKER: OnIncomingEmail
	doOnIncomingEmail := func(w http.ResponseWriter, r *http.Request) error {
		var i OnIncomingEmailIn
		var o OnIncomingEmailOut
		err = httpx.ReadInputPayload(r, RouteOfOnIncomingEmail, &i)
		if err != nil {
			return errors.Trace(err)
		}
		err = handler(
			r.Context(),
			i.MailMessage,
		)
		if err != nil {
			return err // No trace
		}
		err = httpx.WriteOutputPayload(w, o)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	method := "POST"
	path := httpx.JoinHostAndPath(c.host, RouteOfOnIncomingEmail)
	unsub, err = c.svc.Subscribe(method, path, doOnIncomingEmail, c.opts...)
	return unsub, errors.Trace(err)
}

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

package pub

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Request is used to construct an HTTP request that can be sent over the bus.
// Although technically public, it is used internally and should not be constructed by microservices directly.
type Request struct {
	Method        string
	URL           string
	Header        http.Header
	Body          io.Reader
	Multicast     bool
	ContentLength int
	Timeout       time.Duration

	queryArgs string
}

// NewRequest constructs a new request from the provided options.
func NewRequest(options ...Option) (*Request, error) {
	req := &Request{
		Method:    "POST",
		Header:    make(http.Header),
		Multicast: true,
	}
	err := req.Apply(options...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return req, nil
}

// Apply the provided options to the request, in order.
func (req *Request) Apply(options ...Option) error {
	for _, opt := range options {
		err := opt(req)
		if err != nil {
			return errors.Trace(err)
		}
	}
	filled, err := httpx.FillPathArguments(req.URL)
	if err != nil {
		return errors.Trace(err)
	}
	req.URL = filled
	return nil
}

// Canonical returns the fully-qualified canonical path of the request, without the query arguments.
func (req *Request) Canonical() string {
	qm := strings.Index(req.URL, "?")
	if qm >= 0 {
		return req.URL[:qm]
	}
	return req.URL
}

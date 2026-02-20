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

package control

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/coreservices/control/controlapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ controlapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockPing          func(ctx context.Context) (pong int, err error)         // MARKER: Ping
	mockConfigRefresh func(ctx context.Context) (err error)                    // MARKER: ConfigRefresh
	mockTrace         func(ctx context.Context, id string) (err error)         // MARKER: Trace
	mockMetrics       func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Metrics
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockPing sets up a mock handler for Ping.
func (svc *Mock) MockPing(handler func(ctx context.Context) (pong int, err error)) *Mock { // MARKER: Ping
	svc.mockPing = handler
	return svc
}

// Ping executes the mock handler.
func (svc *Mock) Ping(ctx context.Context) (pong int, err error) { // MARKER: Ping
	if svc.mockPing == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	pong, err = svc.mockPing(ctx)
	return pong, errors.Trace(err)
}

// MockConfigRefresh sets up a mock handler for ConfigRefresh.
func (svc *Mock) MockConfigRefresh(handler func(ctx context.Context) (err error)) *Mock { // MARKER: ConfigRefresh
	svc.mockConfigRefresh = handler
	return svc
}

// ConfigRefresh executes the mock handler.
func (svc *Mock) ConfigRefresh(ctx context.Context) (err error) { // MARKER: ConfigRefresh
	if svc.mockConfigRefresh == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockConfigRefresh(ctx)
	return errors.Trace(err)
}

// MockTrace sets up a mock handler for Trace.
func (svc *Mock) MockTrace(handler func(ctx context.Context, id string) (err error)) *Mock { // MARKER: Trace
	svc.mockTrace = handler
	return svc
}

// Trace executes the mock handler.
func (svc *Mock) Trace(ctx context.Context, id string) (err error) { // MARKER: Trace
	if svc.mockTrace == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockTrace(ctx, id)
	return errors.Trace(err)
}

// MockMetrics sets up a mock handler for Metrics.
func (svc *Mock) MockMetrics(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Metrics
	svc.mockMetrics = handler
	return svc
}

// Metrics executes the mock handler.
func (svc *Mock) Metrics(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Metrics
	if svc.mockMetrics == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockMetrics(w, r)
	return errors.Trace(err)
}

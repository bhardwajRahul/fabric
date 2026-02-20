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

package configurator

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/coreservices/configurator/configuratorapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ configuratorapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockValues     func(ctx context.Context, names []string) (values map[string]string, err error)                   // MARKER: Values
	mockRefresh    func(ctx context.Context) (err error)                                                              // MARKER: Refresh
	mockSyncRepo   func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)   // MARKER: SyncRepo
	mockValues443  func(ctx context.Context, names []string) (values map[string]string, err error)                    // MARKER: Values443
	mockRefresh443 func(ctx context.Context) (err error)                                                              // MARKER: Refresh443
	mockSync443    func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)   // MARKER: Sync443
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

// MockValues sets up a mock handler for Values.
func (svc *Mock) MockValues(handler func(ctx context.Context, names []string) (values map[string]string, err error)) *Mock { // MARKER: Values
	svc.mockValues = handler
	return svc
}

// Values executes the mock handler.
func (svc *Mock) Values(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values
	if svc.mockValues == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	values, err = svc.mockValues(ctx, names)
	return values, errors.Trace(err)
}

// MockRefresh sets up a mock handler for Refresh.
func (svc *Mock) MockRefresh(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Refresh
	svc.mockRefresh = handler
	return svc
}

// Refresh executes the mock handler.
func (svc *Mock) Refresh(ctx context.Context) (err error) { // MARKER: Refresh
	if svc.mockRefresh == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	return errors.Trace(svc.mockRefresh(ctx))
}

// MockSyncRepo sets up a mock handler for SyncRepo.
func (svc *Mock) MockSyncRepo(handler func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)) *Mock { // MARKER: SyncRepo
	svc.mockSyncRepo = handler
	return svc
}

// SyncRepo executes the mock handler.
func (svc *Mock) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: SyncRepo
	if svc.mockSyncRepo == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	return errors.Trace(svc.mockSyncRepo(ctx, timestamp, values))
}

// MockValues443 sets up a mock handler for Values443.
func (svc *Mock) MockValues443(handler func(ctx context.Context, names []string) (values map[string]string, err error)) *Mock { // MARKER: Values443
	svc.mockValues443 = handler
	return svc
}

// Values443 executes the mock handler.
func (svc *Mock) Values443(ctx context.Context, names []string) (values map[string]string, err error) { // MARKER: Values443
	if svc.mockValues443 == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	values, err = svc.mockValues443(ctx, names)
	return values, errors.Trace(err)
}

// MockRefresh443 sets up a mock handler for Refresh443.
func (svc *Mock) MockRefresh443(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Refresh443
	svc.mockRefresh443 = handler
	return svc
}

// Refresh443 executes the mock handler.
func (svc *Mock) Refresh443(ctx context.Context) (err error) { // MARKER: Refresh443
	if svc.mockRefresh443 == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	return errors.Trace(svc.mockRefresh443(ctx))
}

// MockSync443 sets up a mock handler for Sync443.
func (svc *Mock) MockSync443(handler func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)) *Mock { // MARKER: Sync443
	svc.mockSync443 = handler
	return svc
}

// Sync443 executes the mock handler.
func (svc *Mock) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) { // MARKER: Sync443
	if svc.mockSync443 == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	return errors.Trace(svc.mockSync443(ctx, timestamp, values))
}

// PeriodicRefresh is a no op in the mock.
func (svc *Mock) PeriodicRefresh(ctx context.Context) (err error) {
	return nil
}

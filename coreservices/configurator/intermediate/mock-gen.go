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

package intermediate

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"

	"github.com/microbus-io/fabric/coreservices/configurator/configuratorapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ configuratorapi.Client
)

// Mock is a mockable version of the configurator.core microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockValues func(ctx context.Context, names []string) (values map[string]string, err error)
	mockRefresh func(ctx context.Context) (err error)
	mockSyncRepo func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)
	mockValues443 func(ctx context.Context, names []string) (values map[string]string, err error)
	mockRefresh443 func(ctx context.Context) (err error)
	mockSync443 func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	m := &Mock{}
	m.Intermediate = NewService(m, 7357) // Stands for TEST
	return m
}

// OnStartup makes sure that the mock is not executed in a non-dev environment.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in '%s' deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is a no op.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockValues sets up a mock handler for the Values endpoint.
func (svc *Mock) MockValues(handler func(ctx context.Context, names []string) (values map[string]string, err error)) *Mock {
	svc.mockValues = handler
	return svc
}

// Values runs the mock handler set by MockValues.
func (svc *Mock) Values(ctx context.Context, names []string) (values map[string]string, err error) {
	if svc.mockValues == nil {
		err = errors.New("mocked endpoint 'Values' not implemented")
		return
	}
	return svc.mockValues(ctx, names)
}

// MockRefresh sets up a mock handler for the Refresh endpoint.
func (svc *Mock) MockRefresh(handler func(ctx context.Context) (err error)) *Mock {
	svc.mockRefresh = handler
	return svc
}

// Refresh runs the mock handler set by MockRefresh.
func (svc *Mock) Refresh(ctx context.Context) (err error) {
	if svc.mockRefresh == nil {
		err = errors.New("mocked endpoint 'Refresh' not implemented")
		return
	}
	return svc.mockRefresh(ctx)
}

// MockSyncRepo sets up a mock handler for the SyncRepo endpoint.
func (svc *Mock) MockSyncRepo(handler func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)) *Mock {
	svc.mockSyncRepo = handler
	return svc
}

// SyncRepo runs the mock handler set by MockSyncRepo.
func (svc *Mock) SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) {
	if svc.mockSyncRepo == nil {
		err = errors.New("mocked endpoint 'SyncRepo' not implemented")
		return
	}
	return svc.mockSyncRepo(ctx, timestamp, values)
}

// MockValues443 sets up a mock handler for the Values443 endpoint.
func (svc *Mock) MockValues443(handler func(ctx context.Context, names []string) (values map[string]string, err error)) *Mock {
	svc.mockValues443 = handler
	return svc
}

// Values443 runs the mock handler set by MockValues443.
func (svc *Mock) Values443(ctx context.Context, names []string) (values map[string]string, err error) {
	if svc.mockValues443 == nil {
		err = errors.New("mocked endpoint 'Values443' not implemented")
		return
	}
	return svc.mockValues443(ctx, names)
}

// MockRefresh443 sets up a mock handler for the Refresh443 endpoint.
func (svc *Mock) MockRefresh443(handler func(ctx context.Context) (err error)) *Mock {
	svc.mockRefresh443 = handler
	return svc
}

// Refresh443 runs the mock handler set by MockRefresh443.
func (svc *Mock) Refresh443(ctx context.Context) (err error) {
	if svc.mockRefresh443 == nil {
		err = errors.New("mocked endpoint 'Refresh443' not implemented")
		return
	}
	return svc.mockRefresh443(ctx)
}

// MockSync443 sets up a mock handler for the Sync443 endpoint.
func (svc *Mock) MockSync443(handler func(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)) *Mock {
	svc.mockSync443 = handler
	return svc
}

// Sync443 runs the mock handler set by MockSync443.
func (svc *Mock) Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) {
	if svc.mockSync443 == nil {
		err = errors.New("mocked endpoint 'Sync443' not implemented")
		return
	}
	return svc.mockSync443(ctx, timestamp, values)
}

// PeriodicRefresh is a no op.
func (svc *Mock) PeriodicRefresh(ctx context.Context) (err error) {
	return nil
}

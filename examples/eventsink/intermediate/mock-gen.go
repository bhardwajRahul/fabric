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

	"github.com/microbus-io/fabric/examples/eventsink/eventsinkapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ eventsinkapi.Client
)

// Mock is a mockable version of the eventsink.example microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockRegistered func(ctx context.Context) (emails []string, err error)
	mockOnAllowRegister func(ctx context.Context, email string) (allow bool, err error)
	mockOnRegistered func(ctx context.Context, email string) (err error)
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

// MockRegistered sets up a mock handler for the Registered endpoint.
func (svc *Mock) MockRegistered(handler func(ctx context.Context) (emails []string, err error)) *Mock {
	svc.mockRegistered = handler
	return svc
}

// Registered runs the mock handler set by MockRegistered.
func (svc *Mock) Registered(ctx context.Context) (emails []string, err error) {
	if svc.mockRegistered == nil {
		err = errors.New("mocked endpoint 'Registered' not implemented")
		return
	}
	return svc.mockRegistered(ctx)
}

// MockOnAllowRegister sets up a mock handler for the OnAllowRegister endpoint.
func (svc *Mock) MockOnAllowRegister(handler func(ctx context.Context, email string) (allow bool, err error)) *Mock {
	svc.mockOnAllowRegister = handler
	return svc
}

// OnAllowRegister runs the mock handler set by MockOnAllowRegister.
func (svc *Mock) OnAllowRegister(ctx context.Context, email string) (allow bool, err error) {
	if svc.mockOnAllowRegister == nil {
		err = errors.New("mocked endpoint 'OnAllowRegister' not implemented")
		return
	}
	return svc.mockOnAllowRegister(ctx, email)
}

// MockOnRegistered sets up a mock handler for the OnRegistered endpoint.
func (svc *Mock) MockOnRegistered(handler func(ctx context.Context, email string) (err error)) *Mock {
	svc.mockOnRegistered = handler
	return svc
}

// OnRegistered runs the mock handler set by MockOnRegistered.
func (svc *Mock) OnRegistered(ctx context.Context, email string) (err error) {
	if svc.mockOnRegistered == nil {
		err = errors.New("mocked endpoint 'OnRegistered' not implemented")
		return
	}
	return svc.mockOnRegistered(ctx, email)
}

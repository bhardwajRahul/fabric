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

package eventsink

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/eventsink/eventsinkapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ eventsinkapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockRegistered      func(ctx context.Context) (emails []string, err error)                 // MARKER: Registered
	mockOnAllowRegister func(ctx context.Context, email string) (allow bool, err error)        // MARKER: OnAllowRegister
	mockOnRegistered    func(ctx context.Context, email string) (err error)                    // MARKER: OnRegistered
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

// MockRegistered sets up a mock handler for Registered.
func (svc *Mock) MockRegistered(handler func(ctx context.Context) (emails []string, err error)) *Mock { // MARKER: Registered
	svc.mockRegistered = handler
	return svc
}

// Registered executes the mock handler.
func (svc *Mock) Registered(ctx context.Context) (emails []string, err error) { // MARKER: Registered
	if svc.mockRegistered == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	emails, err = svc.mockRegistered(ctx)
	return emails, errors.Trace(err)
}

// MockOnAllowRegister sets up a mock handler for OnAllowRegister.
func (svc *Mock) MockOnAllowRegister(handler func(ctx context.Context, email string) (allow bool, err error)) *Mock { // MARKER: OnAllowRegister
	svc.mockOnAllowRegister = handler
	return svc
}

// OnAllowRegister executes the mock handler.
func (svc *Mock) OnAllowRegister(ctx context.Context, email string) (allow bool, err error) { // MARKER: OnAllowRegister
	if svc.mockOnAllowRegister == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	allow, err = svc.mockOnAllowRegister(ctx, email)
	return allow, errors.Trace(err)
}

// MockOnRegistered sets up a mock handler for OnRegistered.
func (svc *Mock) MockOnRegistered(handler func(ctx context.Context, email string) (err error)) *Mock { // MARKER: OnRegistered
	svc.mockOnRegistered = handler
	return svc
}

// OnRegistered executes the mock handler.
func (svc *Mock) OnRegistered(ctx context.Context, email string) (err error) { // MARKER: OnRegistered
	if svc.mockOnRegistered == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnRegistered(ctx, email)
	return errors.Trace(err)
}

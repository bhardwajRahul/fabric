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

package smtpingress

import (
	"context"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
)

// Mock is a mockable version of the smtp.ingress.core microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockOnChangedPort       func(ctx context.Context) (err error) // MARKER: Port
	mockOnChangedEnabled    func(ctx context.Context) (err error) // MARKER: Enabled
	mockOnChangedMaxSize    func(ctx context.Context) (err error) // MARKER: MaxSize
	mockOnChangedMaxClients func(ctx context.Context) (err error) // MARKER: MaxClients
	mockOnChangedWorkers    func(ctx context.Context) (err error) // MARKER: Workers
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
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

// MockOnChangedPort sets up a mock handler for OnChangedPort.
func (svc *Mock) MockOnChangedPort(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Port
	svc.mockOnChangedPort = handler
	return svc
}

// OnChangedPort executes the mock handler.
func (svc *Mock) OnChangedPort(ctx context.Context) (err error) { // MARKER: Port
	if svc.mockOnChangedPort == nil {
		return nil
	}
	err = svc.mockOnChangedPort(ctx)
	return errors.Trace(err)
}

// MockOnChangedEnabled sets up a mock handler for OnChangedEnabled.
func (svc *Mock) MockOnChangedEnabled(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Enabled
	svc.mockOnChangedEnabled = handler
	return svc
}

// OnChangedEnabled executes the mock handler.
func (svc *Mock) OnChangedEnabled(ctx context.Context) (err error) { // MARKER: Enabled
	if svc.mockOnChangedEnabled == nil {
		return nil
	}
	err = svc.mockOnChangedEnabled(ctx)
	return errors.Trace(err)
}

// MockOnChangedMaxSize sets up a mock handler for OnChangedMaxSize.
func (svc *Mock) MockOnChangedMaxSize(handler func(ctx context.Context) (err error)) *Mock { // MARKER: MaxSize
	svc.mockOnChangedMaxSize = handler
	return svc
}

// OnChangedMaxSize executes the mock handler.
func (svc *Mock) OnChangedMaxSize(ctx context.Context) (err error) { // MARKER: MaxSize
	if svc.mockOnChangedMaxSize == nil {
		return nil
	}
	err = svc.mockOnChangedMaxSize(ctx)
	return errors.Trace(err)
}

// MockOnChangedMaxClients sets up a mock handler for OnChangedMaxClients.
func (svc *Mock) MockOnChangedMaxClients(handler func(ctx context.Context) (err error)) *Mock { // MARKER: MaxClients
	svc.mockOnChangedMaxClients = handler
	return svc
}

// OnChangedMaxClients executes the mock handler.
func (svc *Mock) OnChangedMaxClients(ctx context.Context) (err error) { // MARKER: MaxClients
	if svc.mockOnChangedMaxClients == nil {
		return nil
	}
	err = svc.mockOnChangedMaxClients(ctx)
	return errors.Trace(err)
}

// MockOnChangedWorkers sets up a mock handler for OnChangedWorkers.
func (svc *Mock) MockOnChangedWorkers(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Workers
	svc.mockOnChangedWorkers = handler
	return svc
}

// OnChangedWorkers executes the mock handler.
func (svc *Mock) OnChangedWorkers(ctx context.Context) (err error) { // MARKER: Workers
	if svc.mockOnChangedWorkers == nil {
		return nil
	}
	err = svc.mockOnChangedWorkers(ctx)
	return errors.Trace(err)
}

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

package httpingress

import (
	"context"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
)

// Mock is a mockable version of the http.ingress.core microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockOnChangedPorts             func(ctx context.Context) (err error) // MARKER: Ports
	mockOnChangedAllowedOrigins    func(ctx context.Context) (err error) // MARKER: AllowedOrigins
	mockOnChangedPortMappings      func(ctx context.Context) (err error) // MARKER: PortMappings
	mockOnChangedReadTimeout       func(ctx context.Context) (err error) // MARKER: ReadTimeout
	mockOnChangedWriteTimeout      func(ctx context.Context) (err error) // MARKER: WriteTimeout
	mockOnChangedReadHeaderTimeout func(ctx context.Context) (err error) // MARKER: ReadHeaderTimeout
	mockOnChangedBlockedPaths      func(ctx context.Context) (err error) // MARKER: BlockedPaths
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

// MockOnChangedPorts sets up a mock handler for OnChangedPorts.
func (svc *Mock) MockOnChangedPorts(handler func(ctx context.Context) (err error)) *Mock { // MARKER: Ports
	svc.mockOnChangedPorts = handler
	return svc
}

// OnChangedPorts executes the mock handler.
func (svc *Mock) OnChangedPorts(ctx context.Context) (err error) { // MARKER: Ports
	if svc.mockOnChangedPorts == nil {
		return nil
	}
	err = svc.mockOnChangedPorts(ctx)
	return errors.Trace(err)
}

// MockOnChangedAllowedOrigins sets up a mock handler for OnChangedAllowedOrigins.
func (svc *Mock) MockOnChangedAllowedOrigins(handler func(ctx context.Context) (err error)) *Mock { // MARKER: AllowedOrigins
	svc.mockOnChangedAllowedOrigins = handler
	return svc
}

// OnChangedAllowedOrigins executes the mock handler.
func (svc *Mock) OnChangedAllowedOrigins(ctx context.Context) (err error) { // MARKER: AllowedOrigins
	if svc.mockOnChangedAllowedOrigins == nil {
		return nil
	}
	err = svc.mockOnChangedAllowedOrigins(ctx)
	return errors.Trace(err)
}

// MockOnChangedPortMappings sets up a mock handler for OnChangedPortMappings.
func (svc *Mock) MockOnChangedPortMappings(handler func(ctx context.Context) (err error)) *Mock { // MARKER: PortMappings
	svc.mockOnChangedPortMappings = handler
	return svc
}

// OnChangedPortMappings executes the mock handler.
func (svc *Mock) OnChangedPortMappings(ctx context.Context) (err error) { // MARKER: PortMappings
	if svc.mockOnChangedPortMappings == nil {
		return nil
	}
	err = svc.mockOnChangedPortMappings(ctx)
	return errors.Trace(err)
}

// MockOnChangedReadTimeout sets up a mock handler for OnChangedReadTimeout.
func (svc *Mock) MockOnChangedReadTimeout(handler func(ctx context.Context) (err error)) *Mock { // MARKER: ReadTimeout
	svc.mockOnChangedReadTimeout = handler
	return svc
}

// OnChangedReadTimeout executes the mock handler.
func (svc *Mock) OnChangedReadTimeout(ctx context.Context) (err error) { // MARKER: ReadTimeout
	if svc.mockOnChangedReadTimeout == nil {
		return nil
	}
	err = svc.mockOnChangedReadTimeout(ctx)
	return errors.Trace(err)
}

// MockOnChangedWriteTimeout sets up a mock handler for OnChangedWriteTimeout.
func (svc *Mock) MockOnChangedWriteTimeout(handler func(ctx context.Context) (err error)) *Mock { // MARKER: WriteTimeout
	svc.mockOnChangedWriteTimeout = handler
	return svc
}

// OnChangedWriteTimeout executes the mock handler.
func (svc *Mock) OnChangedWriteTimeout(ctx context.Context) (err error) { // MARKER: WriteTimeout
	if svc.mockOnChangedWriteTimeout == nil {
		return nil
	}
	err = svc.mockOnChangedWriteTimeout(ctx)
	return errors.Trace(err)
}

// MockOnChangedReadHeaderTimeout sets up a mock handler for OnChangedReadHeaderTimeout.
func (svc *Mock) MockOnChangedReadHeaderTimeout(handler func(ctx context.Context) (err error)) *Mock { // MARKER: ReadHeaderTimeout
	svc.mockOnChangedReadHeaderTimeout = handler
	return svc
}

// OnChangedReadHeaderTimeout executes the mock handler.
func (svc *Mock) OnChangedReadHeaderTimeout(ctx context.Context) (err error) { // MARKER: ReadHeaderTimeout
	if svc.mockOnChangedReadHeaderTimeout == nil {
		return nil
	}
	err = svc.mockOnChangedReadHeaderTimeout(ctx)
	return errors.Trace(err)
}

// MockOnChangedBlockedPaths sets up a mock handler for OnChangedBlockedPaths.
func (svc *Mock) MockOnChangedBlockedPaths(handler func(ctx context.Context) (err error)) *Mock { // MARKER: BlockedPaths
	svc.mockOnChangedBlockedPaths = handler
	return svc
}

// OnChangedBlockedPaths executes the mock handler.
func (svc *Mock) OnChangedBlockedPaths(ctx context.Context) (err error) { // MARKER: BlockedPaths
	if svc.mockOnChangedBlockedPaths == nil {
		return nil
	}
	err = svc.mockOnChangedBlockedPaths(ctx)
	return errors.Trace(err)
}

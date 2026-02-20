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

package hello

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/hello/helloapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ helloapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockHello        func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Hello
	mockEcho         func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Echo
	mockPing         func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Ping
	mockCalculator   func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Calculator
	mockBusPNG       func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: BusPNG
	mockLocalization func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Localization
	mockRoot         func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Root
	mockTickTock     func(ctx context.Context) (err error)                    // MARKER: TickTock
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

// MockHello sets up a mock handler for Hello.
func (svc *Mock) MockHello(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Hello
	svc.mockHello = handler
	return svc
}

// Hello executes the mock handler.
func (svc *Mock) Hello(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Hello
	if svc.mockHello == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockHello(w, r)
	return errors.Trace(err)
}

// MockEcho sets up a mock handler for Echo.
func (svc *Mock) MockEcho(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Echo
	svc.mockEcho = handler
	return svc
}

// Echo executes the mock handler.
func (svc *Mock) Echo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Echo
	if svc.mockEcho == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockEcho(w, r)
	return errors.Trace(err)
}

// MockPing sets up a mock handler for Ping.
func (svc *Mock) MockPing(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Ping
	svc.mockPing = handler
	return svc
}

// Ping executes the mock handler.
func (svc *Mock) Ping(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Ping
	if svc.mockPing == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockPing(w, r)
	return errors.Trace(err)
}

// MockCalculator sets up a mock handler for Calculator.
func (svc *Mock) MockCalculator(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Calculator
	svc.mockCalculator = handler
	return svc
}

// Calculator executes the mock handler.
func (svc *Mock) Calculator(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Calculator
	if svc.mockCalculator == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockCalculator(w, r)
	return errors.Trace(err)
}

// MockBusPNG sets up a mock handler for BusPNG.
func (svc *Mock) MockBusPNG(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: BusPNG
	svc.mockBusPNG = handler
	return svc
}

// BusPNG executes the mock handler.
func (svc *Mock) BusPNG(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BusPNG
	if svc.mockBusPNG == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockBusPNG(w, r)
	return errors.Trace(err)
}

// MockLocalization sets up a mock handler for Localization.
func (svc *Mock) MockLocalization(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Localization
	svc.mockLocalization = handler
	return svc
}

// Localization executes the mock handler.
func (svc *Mock) Localization(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Localization
	if svc.mockLocalization == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockLocalization(w, r)
	return errors.Trace(err)
}

// MockRoot sets up a mock handler for Root.
func (svc *Mock) MockRoot(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Root
	svc.mockRoot = handler
	return svc
}

// Root executes the mock handler.
func (svc *Mock) Root(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Root
	if svc.mockRoot == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockRoot(w, r)
	return errors.Trace(err)
}

// MockTickTock sets up a mock handler for TickTock.
func (svc *Mock) MockTickTock(handler func(ctx context.Context) (err error)) *Mock { // MARKER: TickTock
	svc.mockTickTock = handler
	return svc
}

// TickTock executes the mock handler.
func (svc *Mock) TickTock(ctx context.Context) (err error) { // MARKER: TickTock
	if svc.mockTickTock == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockTickTock(ctx)
	return errors.Trace(err)
}

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

package calculator

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ calculatorapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockArithmetic func(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) // MARKER: Arithmetic
	mockSquare     func(ctx context.Context, x int) (xEcho int, result int, err error)                                             // MARKER: Square
	mockDistance               func(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error)                // MARKER: Distance
	mockOnObserveSumOperations func(ctx context.Context) (err error)                                                                           // MARKER: SumOperations
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

// MockArithmetic sets up a mock handler for Arithmetic.
func (svc *Mock) MockArithmetic(handler func(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error)) *Mock { // MARKER: Arithmetic
	svc.mockArithmetic = handler
	return svc
}

// Arithmetic executes the mock handler.
func (svc *Mock) Arithmetic(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) { // MARKER: Arithmetic
	if svc.mockArithmetic == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	xEcho, opEcho, yEcho, result, err = svc.mockArithmetic(ctx, x, op, y)
	return xEcho, opEcho, yEcho, result, errors.Trace(err)
}

// MockSquare sets up a mock handler for Square.
func (svc *Mock) MockSquare(handler func(ctx context.Context, x int) (xEcho int, result int, err error)) *Mock { // MARKER: Square
	svc.mockSquare = handler
	return svc
}

// Square executes the mock handler.
func (svc *Mock) Square(ctx context.Context, x int) (xEcho int, result int, err error) { // MARKER: Square
	if svc.mockSquare == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	xEcho, result, err = svc.mockSquare(ctx, x)
	return xEcho, result, errors.Trace(err)
}

// MockDistance sets up a mock handler for Distance.
func (svc *Mock) MockDistance(handler func(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error)) *Mock { // MARKER: Distance
	svc.mockDistance = handler
	return svc
}

// Distance executes the mock handler.
func (svc *Mock) Distance(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error) { // MARKER: Distance
	if svc.mockDistance == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	d, err = svc.mockDistance(ctx, p1, p2)
	return d, errors.Trace(err)
}

// MockOnObserveSumOperations sets up a mock handler for OnObserveSumOperations.
func (svc *Mock) MockOnObserveSumOperations(handler func(ctx context.Context) (err error)) *Mock { // MARKER: SumOperations
	svc.mockOnObserveSumOperations = handler
	return svc
}

// OnObserveSumOperations executes the mock handler.
func (svc *Mock) OnObserveSumOperations(ctx context.Context) (err error) { // MARKER: SumOperations
	if svc.mockOnObserveSumOperations == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnObserveSumOperations(ctx)
	return errors.Trace(err)
}

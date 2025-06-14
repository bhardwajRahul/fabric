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

package calculator

import (
	"context"
	"math"
	"net/http"
	"sync/atomic"

	"github.com/microbus-io/fabric/errors"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
	"github.com/microbus-io/fabric/examples/calculator/intermediate"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ calculatorapi.Client
)

/*
Service implements the calculator.example microservice.

The Calculator microservice performs simple mathematical operations.
*/
type Service struct {
	*intermediate.Intermediate // DO NOT REMOVE

	sumAdd      atomic.Int64
	sumSubtract atomic.Int64
	sumMultiply atomic.Int64
	sumDivide   atomic.Int64
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
Arithmetic perform an arithmetic operation between two integers x and y given an operator op.
*/
func (svc *Service) Arithmetic(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) {
	if op == " " {
		op = "+" // + is interpreted as a space in URLs
	}
	// Perform the arithmetic operation
	switch op {
	case "+":
		result = x + y
		svc.sumAdd.Add(int64(result))
	case "-":
		result = x - y
		svc.sumSubtract.Add(int64(result))
	case "*":
		result = x * y
		svc.sumMultiply.Add(int64(result))
	case "/":
		result = x / y
		svc.sumDivide.Add(int64(result))
	default:
		return x, op, y, result, errors.New("invalid operator '%s'", op)
	}
	svc.AddUsedOperators(ctx, 1, op)
	return x, op, y, result, nil
}

/*
Square prints the square of the integer x.
*/
func (svc *Service) Square(ctx context.Context, x int) (xEcho int, result int, err error) {
	svc.AddUsedOperators(ctx, 1, "^2")
	return x, x * x, nil
}

/*
Distance calculates the distance between two points.
It demonstrates the use of the defined type Point.
*/
func (svc *Service) Distance(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error) {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return math.Sqrt(dx*dx + dy*dy), nil
}

/*
OnObserveSumOperations observes the value of the SumOperations gauge metric.
SumOperations tracks the total sum of the results of all operators.
*/
func (svc *Service) OnObserveSumOperations(ctx context.Context) (err error) {
	svc.RecordSumOperations(ctx, int(svc.sumAdd.Load()), "+")
	svc.RecordSumOperations(ctx, int(svc.sumSubtract.Load()), "-")
	svc.RecordSumOperations(ctx, int(svc.sumMultiply.Load()), "*")
	svc.RecordSumOperations(ctx, int(svc.sumDivide.Load()), "/")
	return nil
}

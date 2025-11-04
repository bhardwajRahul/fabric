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
	"sync"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
)

func TestCalculator_Arithmetic(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("calculator.arithmetic.tester")
	client := calculatorapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Table-driven tests for various arithmetic operations
	tests := []struct {
		name           string
		x              int
		op             string
		y              int
		expectedXEcho  int
		expectedOpEcho string
		expectedYEcho  int
		expectedResult int
		expectError    bool
		errorContains  string
	}{
		// Addition tests
		{name: "addition positive", x: 5, op: "+", y: 3, expectedXEcho: 5, expectedOpEcho: "+", expectedYEcho: 3, expectedResult: 8},
		{name: "addition negative", x: -9, op: "+", y: 9, expectedXEcho: -9, expectedOpEcho: "+", expectedYEcho: 9, expectedResult: 0},
		{name: "addition both negative", x: -5, op: "+", y: -3, expectedXEcho: -5, expectedOpEcho: "+", expectedYEcho: -3, expectedResult: -8},
		{name: "addition zero", x: 0, op: "+", y: 0, expectedXEcho: 0, expectedOpEcho: "+", expectedYEcho: 0, expectedResult: 0},
		{name: "addition with space operator", x: -9, op: " ", y: 9, expectedXEcho: -9, expectedOpEcho: "+", expectedYEcho: 9, expectedResult: 0},
		{name: "addition large numbers", x: 1000000, op: "+", y: 2000000, expectedXEcho: 1000000, expectedOpEcho: "+", expectedYEcho: 2000000, expectedResult: 3000000},

		// Subtraction tests
		{name: "subtraction positive", x: 3, op: "-", y: 8, expectedXEcho: 3, expectedOpEcho: "-", expectedYEcho: 8, expectedResult: -5},
		{name: "subtraction negative result", x: 10, op: "-", y: 15, expectedXEcho: 10, expectedOpEcho: "-", expectedYEcho: 15, expectedResult: -5},
		{name: "subtraction same numbers", x: 7, op: "-", y: 7, expectedXEcho: 7, expectedOpEcho: "-", expectedYEcho: 7, expectedResult: 0},
		{name: "subtraction zero", x: 5, op: "-", y: 0, expectedXEcho: 5, expectedOpEcho: "-", expectedYEcho: 0, expectedResult: 5},

		// Multiplication tests
		{name: "multiplication positive", x: 5, op: "*", y: 5, expectedXEcho: 5, expectedOpEcho: "*", expectedYEcho: 5, expectedResult: 25},
		{name: "multiplication negative", x: 5, op: "*", y: -6, expectedXEcho: 5, expectedOpEcho: "*", expectedYEcho: -6, expectedResult: -30},
		{name: "multiplication by zero", x: 100, op: "*", y: 0, expectedXEcho: 100, expectedOpEcho: "*", expectedYEcho: 0, expectedResult: 0},
		{name: "multiplication by one", x: 42, op: "*", y: 1, expectedXEcho: 42, expectedOpEcho: "*", expectedYEcho: 1, expectedResult: 42},
		{name: "multiplication both negative", x: -3, op: "*", y: -4, expectedXEcho: -3, expectedOpEcho: "*", expectedYEcho: -4, expectedResult: 12},

		// Division tests
		{name: "division positive", x: 15, op: "/", y: 5, expectedXEcho: 15, expectedOpEcho: "/", expectedYEcho: 5, expectedResult: 3},
		{name: "division negative", x: -20, op: "/", y: 4, expectedXEcho: -20, expectedOpEcho: "/", expectedYEcho: 4, expectedResult: -5},
		{name: "division by one", x: 10, op: "/", y: 1, expectedXEcho: 10, expectedOpEcho: "/", expectedYEcho: 1, expectedResult: 10},
		{name: "division with remainder", x: 10, op: "/", y: 3, expectedXEcho: 10, expectedOpEcho: "/", expectedYEcho: 3, expectedResult: 3},
		{name: "division by zero", x: 15, op: "/", y: 0, expectedXEcho: 15, expectedOpEcho: "/", expectedYEcho: 0, expectError: true, errorContains: "zero"},

		// Error cases
		{name: "invalid operator", x: 15, op: "z", y: 5, expectedXEcho: 15, expectedOpEcho: "z", expectedYEcho: 5, expectError: true, errorContains: "operator"},
		{name: "modulo not supported", x: 10, op: "%", y: 3, expectedXEcho: 10, expectedOpEcho: "%", expectedYEcho: 3, expectError: true, errorContains: "operator"},
		{name: "power not supported", x: 2, op: "^", y: 3, expectedXEcho: 2, expectedOpEcho: "^", expectedYEcho: 3, expectError: true, errorContains: "operator"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			xEcho, opEcho, yEcho, result, err := client.Arithmetic(ctx, tc.x, tc.op, tc.y)

			if tc.expectError {
				assert.Contains(err, tc.errorContains)
				// When an error occurs, service returns zero values for echo params
			} else {
				assert.Expect(
					xEcho, tc.expectedXEcho,
					opEcho, tc.expectedOpEcho,
					yEcho, tc.expectedYEcho,
					result, tc.expectedResult,
					err, nil,
				)
			}
		})
	}
}

func TestCalculator_Square(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("calculator.square.tester")
	client := calculatorapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Table-driven tests for square operation
	tests := []struct {
		name           string
		x              int
		expectedXEcho  int
		expectedResult int
	}{
		{name: "square of zero", x: 0, expectedXEcho: 0, expectedResult: 0},
		{name: "square of one", x: 1, expectedXEcho: 1, expectedResult: 1},
		{name: "square of positive", x: 5, expectedXEcho: 5, expectedResult: 25},
		{name: "square of negative", x: -8, expectedXEcho: -8, expectedResult: 64},
		{name: "square of larger number", x: 10, expectedXEcho: 10, expectedResult: 100},
		{name: "square of large negative", x: -15, expectedXEcho: -15, expectedResult: 225},
		{name: "square of two", x: 2, expectedXEcho: 2, expectedResult: 4},
		{name: "square of hundred", x: 100, expectedXEcho: 100, expectedResult: 10000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			xEcho, result, err := client.Square(ctx, tc.x)
			assert.Expect(
				xEcho, tc.expectedXEcho,
				result, tc.expectedResult,
				err, nil,
			)
		})
	}
}

func TestCalculator_Distance(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("calculator.distance.tester")
	client := calculatorapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Table-driven tests for distance calculation
	tests := []struct {
		name             string
		p1               calculatorapi.Point
		p2               calculatorapi.Point
		expectedDistance float64
	}{
		{
			name:             "3-4-5 triangle from origin",
			p1:               calculatorapi.Point{X: 0, Y: 0},
			p2:               calculatorapi.Point{X: 3, Y: 4},
			expectedDistance: 5.0,
		},
		{
			name:             "horizontal line negative coords",
			p1:               calculatorapi.Point{X: -5, Y: -8},
			p2:               calculatorapi.Point{X: 5, Y: -8},
			expectedDistance: 10.0,
		},
		{
			name:             "same point zero distance",
			p1:               calculatorapi.Point{X: 0, Y: 0},
			p2:               calculatorapi.Point{X: 0, Y: 0},
			expectedDistance: 0.0,
		},
		{
			name:             "vertical line",
			p1:               calculatorapi.Point{X: 3, Y: 0},
			p2:               calculatorapi.Point{X: 3, Y: 5},
			expectedDistance: 5.0,
		},
		{
			name:             "horizontal line",
			p1:               calculatorapi.Point{X: 0, Y: 3},
			p2:               calculatorapi.Point{X: 5, Y: 3},
			expectedDistance: 5.0,
		},
		{
			name:             "diagonal with negative coordinates",
			p1:               calculatorapi.Point{X: -3, Y: -4},
			p2:               calculatorapi.Point{X: 0, Y: 0},
			expectedDistance: 5.0,
		},
		{
			name:             "unit distance",
			p1:               calculatorapi.Point{X: 0, Y: 0},
			p2:               calculatorapi.Point{X: 1, Y: 0},
			expectedDistance: 1.0,
		},
		{
			name:             "5-12-13 triangle",
			p1:               calculatorapi.Point{X: 0, Y: 0},
			p2:               calculatorapi.Point{X: 5, Y: 12},
			expectedDistance: 13.0,
		},
		{
			name:             "diagonal same coordinates",
			p1:               calculatorapi.Point{X: 1, Y: 1},
			p2:               calculatorapi.Point{X: 1, Y: 1},
			expectedDistance: 0.0,
		},
		{
			name:             "quadrant 1 to quadrant 3",
			p1:               calculatorapi.Point{X: 3, Y: 3},
			p2:               calculatorapi.Point{X: -3, Y: -3},
			expectedDistance: 8.48528137423857, // sqrt(72)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			d, err := client.Distance(ctx, tc.p1, tc.p2)
			assert.Expect(d, tc.expectedDistance, err, nil)
		})
	}
}

func TestCalculator_OnObserveSumOperations(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("calculator.observesumops.tester")
	client := calculatorapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Test initial state - should succeed without any operations performed
	err := svc.OnObserveSumOperations(ctx)
	assert.NoError(err)

	// Perform some arithmetic operations to update the sums
	_, _, _, _, err = client.Arithmetic(ctx, 10, "+", 5) // sumAdd = 15
	assert.NoError(err)
	_, _, _, _, err = client.Arithmetic(ctx, 20, "+", 10) // sumAdd = 45
	assert.NoError(err)

	_, _, _, _, err = client.Arithmetic(ctx, 10, "-", 3) // sumSubtract = 7
	assert.NoError(err)
	_, _, _, _, err = client.Arithmetic(ctx, 5, "-", 8) // sumSubtract = 4
	assert.NoError(err)

	_, _, _, _, err = client.Arithmetic(ctx, 3, "*", 4) // sumMultiply = 12
	assert.NoError(err)
	_, _, _, _, err = client.Arithmetic(ctx, 5, "*", 2) // sumMultiply = 22
	assert.NoError(err)

	_, _, _, _, err = client.Arithmetic(ctx, 20, "/", 4) // sumDivide = 5
	assert.NoError(err)
	_, _, _, _, err = client.Arithmetic(ctx, 15, "/", 3) // sumDivide = 10
	assert.NoError(err)

	// Test after operations - should successfully observe the accumulated sums
	err = svc.OnObserveSumOperations(ctx)
	assert.NoError(err)

	// Perform more operations
	_, _, _, _, err = client.Arithmetic(ctx, 100, "+", 200) // sumAdd = 345
	assert.NoError(err)

	// Test again to ensure repeated calls work correctly
	err = svc.OnObserveSumOperations(ctx)
	assert.NoError(err)
}

// TestCalculator_ConcurrentOperations tests thread safety of arithmetic operations
func TestCalculator_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("calculator.concurrent.tester")
	client := calculatorapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	// Perform multiple operations concurrently to test thread safety
	operations := []struct {
		x  int
		op string
		y  int
	}{
		{5, "+", 3},
		{10, "-", 4},
		{7, "*", 6},
		{20, "/", 4},
		{15, "+", 25},
		{100, "-", 50},
	}

	var wg sync.WaitGroup
	for _, op := range operations {
		wg.Add(1)
		go func(x int, operator string, y int) {
			defer wg.Done()
			_, _, _, _, err := client.Arithmetic(ctx, x, operator, y)
			assert.NoError(err)
		}(op.x, op.op, op.y)
	}
	wg.Wait()

	// Verify the observer still works after concurrent operations
	err := svc.OnObserveSumOperations(ctx)
	assert.NoError(err)
}

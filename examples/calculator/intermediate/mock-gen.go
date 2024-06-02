/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

package intermediate

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
)

var (
	_ context.Context
	_ *json.Decoder
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ *httpx.ResponseRecorder
	_ sub.Option
	_ calculatorapi.Client
)

// Mock is a mockable version of the calculator.example microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*connector.Connector
	mockArithmetic func(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error)
	mockSquare func(ctx context.Context, x int) (xEcho int, result int, err error)
	mockDistance func(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error)
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{
		Connector: connector.New("calculator.example"),
	}
	svc.SetVersion(7357) // Stands for TEST
	svc.SetDescription(`The Calculator microservice performs simple mathematical operations.`)
	svc.SetOnStartup(svc.doOnStartup)

	// Functions
	svc.Subscribe(`GET`, `:443/arithmetic`, svc.doArithmetic)
	svc.Subscribe(`GET`, `:443/square`, svc.doSquare)
	svc.Subscribe(`ANY`, `:443/distance`, svc.doDistance)

	return svc
}

// doOnStartup makes sure that the mock is not executed in a non-dev environment.
func (svc *Mock) doOnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.Newf("mocking disallowed in '%s' deployment", svc.Deployment())
	}
	return nil
}

// doArithmetic handles marshaling for the Arithmetic function.
func (svc *Mock) doArithmetic(w http.ResponseWriter, r *http.Request) error {
	if svc.mockArithmetic == nil {
		return errors.New("mocked endpoint 'Arithmetic' not implemented")
	}
	var i calculatorapi.ArithmeticIn
	var o calculatorapi.ArithmeticOut
	err := httpx.ParseRequestData(r, &i)
	if err!=nil {
		return errors.Trace(err)
	}
	o.XEcho, o.OpEcho, o.YEcho, o.Result, err = svc.mockArithmetic(
		r.Context(),
		i.X,
		i.Op,
		i.Y,
	)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// MockArithmetic sets up a mock handler for the Arithmetic function.
func (svc *Mock) MockArithmetic(handler func(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error)) *Mock {
	svc.mockArithmetic = handler
	return svc
}

// doSquare handles marshaling for the Square function.
func (svc *Mock) doSquare(w http.ResponseWriter, r *http.Request) error {
	if svc.mockSquare == nil {
		return errors.New("mocked endpoint 'Square' not implemented")
	}
	var i calculatorapi.SquareIn
	var o calculatorapi.SquareOut
	err := httpx.ParseRequestData(r, &i)
	if err!=nil {
		return errors.Trace(err)
	}
	o.XEcho, o.Result, err = svc.mockSquare(
		r.Context(),
		i.X,
	)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// MockSquare sets up a mock handler for the Square function.
func (svc *Mock) MockSquare(handler func(ctx context.Context, x int) (xEcho int, result int, err error)) *Mock {
	svc.mockSquare = handler
	return svc
}

// doDistance handles marshaling for the Distance function.
func (svc *Mock) doDistance(w http.ResponseWriter, r *http.Request) error {
	if svc.mockDistance == nil {
		return errors.New("mocked endpoint 'Distance' not implemented")
	}
	var i calculatorapi.DistanceIn
	var o calculatorapi.DistanceOut
	err := httpx.ParseRequestData(r, &i)
	if err!=nil {
		return errors.Trace(err)
	}
	o.D, err = svc.mockDistance(
		r.Context(),
		i.P1,
		i.P2,
	)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// MockDistance sets up a mock handler for the Distance function.
func (svc *Mock) MockDistance(handler func(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error)) *Mock {
	svc.mockDistance = handler
	return svc
}

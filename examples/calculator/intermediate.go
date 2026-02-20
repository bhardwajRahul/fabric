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
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/examples/calculator/calculatorapi"
	"github.com/microbus-io/fabric/examples/calculator/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ calculatorapi.Client
)

const (
	Hostname = calculatorapi.Hostname
	Version  = 352
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	OnObserveSumOperations(ctx context.Context) (err error)                                                               // MARKER: SumOperations
	Arithmetic(ctx context.Context, x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int, err error) // MARKER: Arithmetic
	Square(ctx context.Context, x int) (xEcho int, result int, err error)                                                 // MARKER: Square
	Distance(ctx context.Context, p1 calculatorapi.Point, p2 calculatorapi.Point) (d float64, err error)                  // MARKER: Distance
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`The Calculator microservice performs simple mathematical operations.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(calculatorapi.Arithmetic.Method, calculatorapi.Arithmetic.Route, svc.doArithmetic) // MARKER: Arithmetic
	svc.Subscribe(calculatorapi.Square.Method, calculatorapi.Square.Route, svc.doSquare)             // MARKER: Square
	svc.Subscribe(calculatorapi.Distance.Method, calculatorapi.Distance.Route, svc.doDistance)       // MARKER: Distance

	// HINT: Add web endpoints here

	// HINT: Add metrics here
	svc.DescribeCounter("used_operators", "UsedOperators tracks the types of the arithmetic operators used.")  // MARKER: UsedOperators
	svc.DescribeGauge("sum_operations", "SumOperations tracks the total sum of the results of all operators.") // MARKER: SumOperations

	// HINT: Add tickers here

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

	_ = marshalFunction
	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
		{ // MARKER: Arithmetic
			Type:        "function",
			Name:        "Arithmetic",
			Method:      calculatorapi.Arithmetic.Method,
			Route:       calculatorapi.Arithmetic.Route,
			Summary:     "Arithmetic(x int, op string, y int) (xEcho int, opEcho string, yEcho int, result int)",
			Description: `Arithmetic performs an arithmetic operation between two integers x and y given an operator op.`,
			InputArgs:   calculatorapi.ArithmeticIn{},
			OutputArgs:  calculatorapi.ArithmeticOut{},
		},
		{ // MARKER: Square
			Type:        "function",
			Name:        "Square",
			Method:      calculatorapi.Square.Method,
			Route:       calculatorapi.Square.Route,
			Summary:     "Square(x int) (xEcho int, result int)",
			Description: `Square prints the square of the integer x.`,
			InputArgs:   calculatorapi.SquareIn{},
			OutputArgs:  calculatorapi.SquareOut{},
		},
		{ // MARKER: Distance
			Type:        "function",
			Name:        "Distance",
			Method:      calculatorapi.Distance.Method,
			Route:       calculatorapi.Distance.Route,
			Summary:     "Distance(p1 Point, p2 Point) (d float64)",
			Description: `Distance calculates the distance between two points. It demonstrates the use of the defined type Point.`,
			InputArgs:   calculatorapi.DistanceIn{},
			OutputArgs:  calculatorapi.DistanceOut{},
		},
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
		// HINT: Call JIT observers to record the metric here
		func() (err error) { return svc.OnObserveSumOperations(ctx) }, // MARKER: SumOperations
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	return nil
}

/*
IncrementUsedOperators counts the types of the arithmetic operators used.
*/
func (svc *Intermediate) IncrementUsedOperators(ctx context.Context, num int, op string) (err error) { // MARKER: UsedOperators
	return svc.IncrementCounter(ctx, "used_operators", float64(num),
		"op", utils.AnyToString(op),
	)
}

/*
RecordSumOperations records the total sum of the results of all operators.
*/
func (svc *Intermediate) RecordSumOperations(ctx context.Context, sum int, op string) (err error) { // MARKER: SumOperations
	return svc.RecordGauge(ctx, "sum_operations", float64(sum),
		"op", utils.AnyToString(op),
	)
}

// doArithmetic handles marshaling for Arithmetic.
func (svc *Intermediate) doArithmetic(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Arithmetic
	var in calculatorapi.ArithmeticIn
	var out calculatorapi.ArithmeticOut
	err = marshalFunction(w, r, calculatorapi.Arithmetic.Route, &in, &out, func(_ any, _ any) error {
		out.XEcho, out.OpEcho, out.YEcho, out.Result, err = svc.Arithmetic(r.Context(), in.X, in.Op, in.Y)
		return err
	})
	return err // No trace
}

// doSquare handles marshaling for Square.
func (svc *Intermediate) doSquare(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Square
	var in calculatorapi.SquareIn
	var out calculatorapi.SquareOut
	err = marshalFunction(w, r, calculatorapi.Square.Route, &in, &out, func(_ any, _ any) error {
		out.XEcho, out.Result, err = svc.Square(r.Context(), in.X)
		return err
	})
	return err // No trace
}

// doDistance handles marshaling for Distance.
func (svc *Intermediate) doDistance(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Distance
	var in calculatorapi.DistanceIn
	var out calculatorapi.DistanceOut
	err = marshalFunction(w, r, calculatorapi.Distance.Route, &in, &out, func(_ any, _ any) error {
		out.D, err = svc.Distance(r.Context(), in.P1, in.P2)
		return err
	})
	return err // No trace
}

// marshalFunction handled marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

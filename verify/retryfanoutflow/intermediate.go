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

package retryfanoutflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/retryfanoutflow/retryfanoutflowapi"
	"github.com/microbus-io/fabric/verify/retryfanoutflow/resources"
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
	_ retryfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = retryfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Enter(ctx context.Context, flow *workflow.Flow, elements []int) (elementsOut []int, err error)     // MARKER: Enter
	Increment(ctx context.Context, flow *workflow.Flow, element int) (resultsOut []int, err error)     // MARKER: Increment
	Join(ctx context.Context, flow *workflow.Flow, results []int) (resultsOut []int, err error)        // MARKER: Join
	RetryFanOut(ctx context.Context) (graph *workflow.Graph, err error)                                // MARKER: RetryFanOut
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
	svc.SetDescription(`retryfanoutflow.verify exercises ordered list fan-in across a forEach fan-out whose per-element task retries infinitely on a random 10% failure.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: Enter
		"Enter", svc.doEnter,
		sub.At(retryfanoutflowapi.Enter.Method, retryfanoutflowapi.Enter.Route),
		sub.Description(`Enter is the forEach source.`),
		sub.Task(retryfanoutflowapi.EnterIn{}, retryfanoutflowapi.EnterOut{}),
	)
	svc.Subscribe( // MARKER: Increment
		"Increment", svc.doIncrement,
		sub.At(retryfanoutflowapi.Increment.Method, retryfanoutflowapi.Increment.Route),
		sub.Description(`Increment runs once per element, retrying infinitely on a random 10% failure.`),
		sub.Task(retryfanoutflowapi.IncrementIn{}, retryfanoutflowapi.IncrementOut{}),
	)
	svc.Subscribe( // MARKER: Join
		"Join", svc.doJoin,
		sub.At(retryfanoutflowapi.Join.Method, retryfanoutflowapi.Join.Route),
		sub.Description(`Join is the fan-in target.`),
		sub.Task(retryfanoutflowapi.JoinIn{}, retryfanoutflowapi.JoinOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: RetryFanOut
		"RetryFanOut", svc.doRetryFanOut,
		sub.At(retryfanoutflowapi.RetryFanOut.Method, retryfanoutflowapi.RetryFanOut.Route),
		sub.Description(`RetryFanOut defines the graph: Enter -> forEach(elements) -> Increment -> Join.`),
		sub.Workflow(retryfanoutflowapi.RetryFanOutIn{}, retryfanoutflowapi.RetryFanOutOut{}),
	)

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

// marshalFunction handles marshaling for functional endpoints.
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

// doEnter handles marshaling for Enter.
func (svc *Intermediate) doEnter(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Enter
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryfanoutflowapi.EnterIn
	flow.ParseState(&in)
	var out retryfanoutflowapi.EnterOut
	out.ElementsOut, err = svc.Enter(r.Context(), &flow, in.Elements)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doIncrement handles marshaling for Increment.
func (svc *Intermediate) doIncrement(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Increment
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryfanoutflowapi.IncrementIn
	flow.ParseState(&in)
	var out retryfanoutflowapi.IncrementOut
	out.ResultsOut, err = svc.Increment(r.Context(), &flow, in.Element)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doJoin handles marshaling for Join.
func (svc *Intermediate) doJoin(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Join
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryfanoutflowapi.JoinIn
	flow.ParseState(&in)
	var out retryfanoutflowapi.JoinOut
	out.ResultsOut, err = svc.Join(r.Context(), &flow, in.Results)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doRetryFanOut handles marshaling for RetryFanOut.
func (svc *Intermediate) doRetryFanOut(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RetryFanOut
	graph, err := svc.RetryFanOut(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

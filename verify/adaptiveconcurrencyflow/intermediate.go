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

package adaptiveconcurrencyflow

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

	"github.com/microbus-io/fabric/verify/adaptiveconcurrencyflow/adaptiveconcurrencyflowapi"
	"github.com/microbus-io/fabric/verify/adaptiveconcurrencyflow/resources"
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
	_ adaptiveconcurrencyflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = adaptiveconcurrencyflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Adaptive(ctx context.Context, flow *workflow.Flow, tag string) (tallied bool, err error) // MARKER: Adaptive
	AdaptiveConcurrency(ctx context.Context) (graph *workflow.Graph, err error)              // MARKER: AdaptiveConcurrency
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
	svc.SetDescription(`adaptiveconcurrencyflow.verify exercises the foreman's CUBIC recovery curve via a runtime-mutable task cap.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: Adaptive
		"Adaptive", svc.doAdaptive,
		sub.At(adaptiveconcurrencyflowapi.Adaptive.Method, adaptiveconcurrencyflowapi.Adaptive.Route),
		sub.Description(`Adaptive admits up to the current cap concurrent in-flight executions and returns 429 above that.`),
		sub.Task(adaptiveconcurrencyflowapi.AdaptiveIn{}, adaptiveconcurrencyflowapi.AdaptiveOut{}),
	)

	svc.Subscribe( // MARKER: AdaptiveConcurrency
		"AdaptiveConcurrency", svc.doAdaptiveConcurrency,
		sub.At(adaptiveconcurrencyflowapi.AdaptiveConcurrency.Method, adaptiveconcurrencyflowapi.AdaptiveConcurrency.Route),
		sub.Description(`AdaptiveConcurrency routes through the cap-mutable Adaptive task; exercises the CUBIC recovery curve.`),
		sub.Workflow(adaptiveconcurrencyflowapi.AdaptiveConcurrencyIn{}, adaptiveconcurrencyflowapi.AdaptiveConcurrencyOut{}),
	)

	_ = marshalFunction
	return svc
}

func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doAdaptive handles marshaling for Adaptive.
func (svc *Intermediate) doAdaptive(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Adaptive
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in adaptiveconcurrencyflowapi.AdaptiveIn
	flow.ParseState(&in)
	var out adaptiveconcurrencyflowapi.AdaptiveOut
	out.Tallied, err = svc.Adaptive(r.Context(), &flow, in.Tag)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doAdaptiveConcurrency handles marshaling for AdaptiveConcurrency.
func (svc *Intermediate) doAdaptiveConcurrency(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AdaptiveConcurrency
	graph, err := svc.AdaptiveConcurrency(r.Context())
	if err != nil {
		return err
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

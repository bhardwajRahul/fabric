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

package continueflow

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

	"github.com/microbus-io/fabric/verify/continueflow/continueflowapi"
	"github.com/microbus-io/fabric/verify/continueflow/resources"
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
	_ continueflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = continueflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Increment(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) // MARKER: Increment
	Counting(ctx context.Context) (graph *workflow.Graph, err error)                              // MARKER: Counting
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
	svc.SetDescription(`continueflow.verify exercises multi-turn flows via Continue.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: Increment
		"Increment", svc.doIncrement,
		sub.At(continueflowapi.Increment.Method, continueflowapi.Increment.Route),
		sub.Description(`Increment reads counter and writes counter+1.`),
		sub.Task(continueflowapi.IncrementIn{}, continueflowapi.IncrementOut{}),
	)

	svc.Subscribe( // MARKER: Counting
		"Counting", svc.doCounting,
		sub.At(continueflowapi.Counting.Method, continueflowapi.Counting.Route),
		sub.Description(`Counting defines a single-task workflow that persists counter across turns.`),
		sub.Workflow(continueflowapi.CountingIn{}, continueflowapi.CountingOut{}),
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

// doIncrement handles marshaling for Increment.
func (svc *Intermediate) doIncrement(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Increment
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in continueflowapi.IncrementIn
	flow.ParseState(&in)
	var out continueflowapi.IncrementOut
	out.CounterOut, err = svc.Increment(r.Context(), &flow, in.Counter)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doCounting handles marshaling for Counting.
func (svc *Intermediate) doCounting(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Counting
	graph, err := svc.Counting(r.Context())
	if err != nil {
		return err
	}
	if err = graph.Validate(); err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

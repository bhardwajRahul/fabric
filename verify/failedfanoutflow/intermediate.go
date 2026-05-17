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

package failedfanoutflow

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

	"github.com/microbus-io/fabric/verify/failedfanoutflow/failedfanoutflowapi"
	"github.com/microbus-io/fabric/verify/failedfanoutflow/resources"
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
	_ failedfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = failedfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Src(ctx context.Context, flow *workflow.Flow) (started bool, err error)                             // MARKER: Src
	A(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error)                         // MARKER: A
	B(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error)                         // MARKER: B
	C(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error)                         // MARKER: C
	J(ctx context.Context, flow *workflow.Flow, sumExecuted int) (totalExecuted int, err error)         // MARKER: J
	FailedFanOut(ctx context.Context) (graph *workflow.Graph, err error)                                // MARKER: FailedFanOut
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
	svc.SetDescription(`failedfanoutflow.verify exercises a fan-out branch that hard-fails with no OnError transition, cascading the whole flow to failed.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: Src
		"Src", svc.doSrc,
		sub.At(failedfanoutflowapi.Src.Method, failedfanoutflowapi.Src.Route),
		sub.Description(`Src is the static fan-out source.`),
		sub.Task(failedfanoutflowapi.SrcIn{}, failedfanoutflowapi.SrcOut{}),
	)
	svc.Subscribe( // MARKER: A
		"A", svc.doA,
		sub.At(failedfanoutflowapi.A.Method, failedfanoutflowapi.A.Route),
		sub.Description(`A is a normal fan-out branch.`),
		sub.Task(failedfanoutflowapi.AIn{}, failedfanoutflowapi.AOut{}),
	)
	svc.Subscribe( // MARKER: B
		"B", svc.doB,
		sub.At(failedfanoutflowapi.B.Method, failedfanoutflowapi.B.Route),
		sub.Description(`B is the failing fan-out branch.`),
		sub.Task(failedfanoutflowapi.BIn{}, failedfanoutflowapi.BOut{}),
	)
	svc.Subscribe( // MARKER: C
		"C", svc.doC,
		sub.At(failedfanoutflowapi.C.Method, failedfanoutflowapi.C.Route),
		sub.Description(`C is a normal fan-out branch.`),
		sub.Task(failedfanoutflowapi.CIn{}, failedfanoutflowapi.COut{}),
	)
	svc.Subscribe( // MARKER: J
		"J", svc.doJ,
		sub.At(failedfanoutflowapi.J.Method, failedfanoutflowapi.J.Route),
		sub.Description(`J is the fan-in target.`),
		sub.Task(failedfanoutflowapi.JIn{}, failedfanoutflowapi.JOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: FailedFanOut
		"FailedFanOut", svc.doFailedFanOut,
		sub.At(failedfanoutflowapi.FailedFanOut.Method, failedfanoutflowapi.FailedFanOut.Route),
		sub.Description(`FailedFanOut defines the graph: Src -> {A, B, C} -> J.`),
		sub.Workflow(failedfanoutflowapi.FailedFanOutIn{}, failedfanoutflowapi.FailedFanOutOut{}),
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

// doSrc handles marshaling for Src.
func (svc *Intermediate) doSrc(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Src
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in failedfanoutflowapi.SrcIn
	flow.ParseState(&in)
	var out failedfanoutflowapi.SrcOut
	out.Started, err = svc.Src(r.Context(), &flow)
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

// doA handles marshaling for A.
func (svc *Intermediate) doA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: A
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in failedfanoutflowapi.AIn
	flow.ParseState(&in)
	var out failedfanoutflowapi.AOut
	out.SumExecutedOut, err = svc.A(r.Context(), &flow)
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

// doB handles marshaling for B.
func (svc *Intermediate) doB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: B
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in failedfanoutflowapi.BIn
	flow.ParseState(&in)
	var out failedfanoutflowapi.BOut
	out.SumExecutedOut, err = svc.B(r.Context(), &flow)
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

// doC handles marshaling for C.
func (svc *Intermediate) doC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: C
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in failedfanoutflowapi.CIn
	flow.ParseState(&in)
	var out failedfanoutflowapi.COut
	out.SumExecutedOut, err = svc.C(r.Context(), &flow)
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

// doJ handles marshaling for J.
func (svc *Intermediate) doJ(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: J
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in failedfanoutflowapi.JIn
	flow.ParseState(&in)
	var out failedfanoutflowapi.JOut
	out.TotalExecuted, err = svc.J(r.Context(), &flow, in.SumExecuted)
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

// doFailedFanOut handles marshaling for FailedFanOut.
func (svc *Intermediate) doFailedFanOut(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: FailedFanOut
	graph, err := svc.FailedFanOut(r.Context())
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

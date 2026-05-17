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

package subgraphfanoutflow

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

	"github.com/microbus-io/fabric/verify/subgraphfanoutflow/resources"
	"github.com/microbus-io/fabric/verify/subgraphfanoutflow/subgraphfanoutflowapi"
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
	_ subgraphfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = subgraphfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                          // MARKER: TaskA
	NormalB(ctx context.Context, flow *workflow.Flow) (resultB string, err error)                                                      // MARKER: NormalB
	TaskX(ctx context.Context, flow *workflow.Flow) (xPassed bool, err error)                                                          // MARKER: TaskX
	TaskY(ctx context.Context, flow *workflow.Flow, xPassed bool) (subResult string, err error)                                        // MARKER: TaskY
	NormalD(ctx context.Context, flow *workflow.Flow) (resultD string, err error)                                                      // MARKER: NormalD
	TaskE(ctx context.Context, flow *workflow.Flow, resultB, subResult, resultD string) (finalResult string, err error)                // MARKER: TaskE
	Sub(ctx context.Context) (graph *workflow.Graph, err error)                                                                         // MARKER: Sub
	SubFanOut(ctx context.Context) (graph *workflow.Graph, err error)                                                                   // MARKER: SubFanOut
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
	svc.SetDescription(`subgraphfanoutflow.verify exercises a subgraph as one sibling of an outer fan-out.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(subgraphfanoutflowapi.TaskA.Method, subgraphfanoutflowapi.TaskA.Route),
		sub.Description(`TaskA is the fan-out source.`),
		sub.Task(subgraphfanoutflowapi.TaskAIn{}, subgraphfanoutflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: NormalB
		"NormalB", svc.doNormalB,
		sub.At(subgraphfanoutflowapi.NormalB.Method, subgraphfanoutflowapi.NormalB.Route),
		sub.Description(`NormalB produces resultB.`),
		sub.Task(subgraphfanoutflowapi.NormalBIn{}, subgraphfanoutflowapi.NormalBOut{}),
	)
	svc.Subscribe( // MARKER: TaskX
		"TaskX", svc.doTaskX,
		sub.At(subgraphfanoutflowapi.TaskX.Method, subgraphfanoutflowapi.TaskX.Route),
		sub.Description(`TaskX is the subgraph entry.`),
		sub.Task(subgraphfanoutflowapi.TaskXIn{}, subgraphfanoutflowapi.TaskXOut{}),
	)
	svc.Subscribe( // MARKER: TaskY
		"TaskY", svc.doTaskY,
		sub.At(subgraphfanoutflowapi.TaskY.Method, subgraphfanoutflowapi.TaskY.Route),
		sub.Description(`TaskY runs after TaskX in the subgraph.`),
		sub.Task(subgraphfanoutflowapi.TaskYIn{}, subgraphfanoutflowapi.TaskYOut{}),
	)
	svc.Subscribe( // MARKER: NormalD
		"NormalD", svc.doNormalD,
		sub.At(subgraphfanoutflowapi.NormalD.Method, subgraphfanoutflowapi.NormalD.Route),
		sub.Description(`NormalD produces resultD.`),
		sub.Task(subgraphfanoutflowapi.NormalDIn{}, subgraphfanoutflowapi.NormalDOut{}),
	)
	svc.Subscribe( // MARKER: TaskE
		"TaskE", svc.doTaskE,
		sub.At(subgraphfanoutflowapi.TaskE.Method, subgraphfanoutflowapi.TaskE.Route),
		sub.Description(`TaskE is the outer fan-in.`),
		sub.Task(subgraphfanoutflowapi.TaskEIn{}, subgraphfanoutflowapi.TaskEOut{}),
	)

	svc.Subscribe( // MARKER: Sub
		"Sub", svc.doSub,
		sub.At(subgraphfanoutflowapi.Sub.Method, subgraphfanoutflowapi.Sub.Route),
		sub.Description(`Sub defines the sequential subgraph X -> Y.`),
		sub.Workflow(subgraphfanoutflowapi.SubIn{}, subgraphfanoutflowapi.SubOut{}),
	)
	svc.Subscribe( // MARKER: SubFanOut
		"SubFanOut", svc.doSubFanOut,
		sub.At(subgraphfanoutflowapi.SubFanOut.Method, subgraphfanoutflowapi.SubFanOut.Route),
		sub.Description(`SubFanOut defines A -> {NormalB, Sub, NormalD} -> E.`),
		sub.Workflow(subgraphfanoutflowapi.SubFanOutIn{}, subgraphfanoutflowapi.SubFanOutOut{}),
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

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.TaskAIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.TaskAOut
	out.Started, err = svc.TaskA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doNormalB handles marshaling for NormalB.
func (svc *Intermediate) doNormalB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NormalB
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.NormalBIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.NormalBOut
	out.ResultB, err = svc.NormalB(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskX handles marshaling for TaskX.
func (svc *Intermediate) doTaskX(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskX
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.TaskXIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.TaskXOut
	out.XPassed, err = svc.TaskX(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskY handles marshaling for TaskY.
func (svc *Intermediate) doTaskY(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskY
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.TaskYIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.TaskYOut
	out.SubResult, err = svc.TaskY(r.Context(), &flow, in.XPassed)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doNormalD handles marshaling for NormalD.
func (svc *Intermediate) doNormalD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NormalD
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.NormalDIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.NormalDOut
	out.ResultD, err = svc.NormalD(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskE handles marshaling for TaskE.
func (svc *Intermediate) doTaskE(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskE
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphfanoutflowapi.TaskEIn
	flow.ParseState(&in)
	var out subgraphfanoutflowapi.TaskEOut
	out.FinalResult, err = svc.TaskE(r.Context(), &flow, in.ResultB, in.SubResult, in.ResultD)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doSub handles marshaling for Sub.
func (svc *Intermediate) doSub(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Sub
	graph, err := svc.Sub(r.Context())
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

// doSubFanOut handles marshaling for SubFanOut.
func (svc *Intermediate) doSubFanOut(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SubFanOut
	graph, err := svc.SubFanOut(r.Context())
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

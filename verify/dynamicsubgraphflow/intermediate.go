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

package dynamicsubgraphflow

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

	"github.com/microbus-io/fabric/verify/dynamicsubgraphflow/dynamicsubgraphflowapi"
	"github.com/microbus-io/fabric/verify/dynamicsubgraphflow/resources"
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
	_ dynamicsubgraphflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = dynamicsubgraphflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Parent(ctx context.Context, flow *workflow.Flow, value int, innerDone bool, innerResult int) (parentResult string, err error) // MARKER: Parent
	InnerA(ctx context.Context, flow *workflow.Flow, value int) (innerStage int, err error)                                       // MARKER: InnerA
	InnerB(ctx context.Context, flow *workflow.Flow, innerStage int) (innerResult int, innerDone bool, err error)                 // MARKER: InnerB
	Inner(ctx context.Context) (graph *workflow.Graph, err error)                                                                  // MARKER: Inner
	DynamicSubgraph(ctx context.Context) (graph *workflow.Graph, err error)                                                        // MARKER: DynamicSubgraph
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
	svc.SetDescription(`dynamicsubgraphflow.verify exercises the flow.Subgraph control signal.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: Parent
		"Parent", svc.doParent,
		sub.At(dynamicsubgraphflowapi.Parent.Method, dynamicsubgraphflowapi.Parent.Route),
		sub.Description(`Parent calls flow.Subgraph on first invocation, then re-runs with merged state.`),
		sub.Task(dynamicsubgraphflowapi.ParentIn{}, dynamicsubgraphflowapi.ParentOut{}),
	)
	svc.Subscribe( // MARKER: InnerA
		"InnerA", svc.doInnerA,
		sub.At(dynamicsubgraphflowapi.InnerA.Method, dynamicsubgraphflowapi.InnerA.Route),
		sub.Description(`InnerA doubles value.`),
		sub.Task(dynamicsubgraphflowapi.InnerAIn{}, dynamicsubgraphflowapi.InnerAOut{}),
	)
	svc.Subscribe( // MARKER: InnerB
		"InnerB", svc.doInnerB,
		sub.At(dynamicsubgraphflowapi.InnerB.Method, dynamicsubgraphflowapi.InnerB.Route),
		sub.Description(`InnerB adds 3 and marks the subgraph done.`),
		sub.Task(dynamicsubgraphflowapi.InnerBIn{}, dynamicsubgraphflowapi.InnerBOut{}),
	)

	svc.Subscribe( // MARKER: Inner
		"Inner", svc.doInner,
		sub.At(dynamicsubgraphflowapi.Inner.Method, dynamicsubgraphflowapi.Inner.Route),
		sub.Description(`Inner defines the child subgraph InnerA -> InnerB.`),
		sub.Workflow(dynamicsubgraphflowapi.InnerIn{}, dynamicsubgraphflowapi.InnerOut{}),
	)
	svc.Subscribe( // MARKER: DynamicSubgraph
		"DynamicSubgraph", svc.doDynamicSubgraph,
		sub.At(dynamicsubgraphflowapi.DynamicSubgraph.Method, dynamicsubgraphflowapi.DynamicSubgraph.Route),
		sub.Description(`DynamicSubgraph defines a single-task parent that invokes Inner via flow.Subgraph.`),
		sub.Workflow(dynamicsubgraphflowapi.DynamicSubgraphIn{}, dynamicsubgraphflowapi.DynamicSubgraphOut{}),
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

// doParent handles marshaling for Parent.
func (svc *Intermediate) doParent(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Parent
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in dynamicsubgraphflowapi.ParentIn
	flow.ParseState(&in)
	var out dynamicsubgraphflowapi.ParentOut
	out.ParentResult, err = svc.Parent(r.Context(), &flow, in.Value, in.InnerDone, in.InnerResult)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInnerA handles marshaling for InnerA.
func (svc *Intermediate) doInnerA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: InnerA
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in dynamicsubgraphflowapi.InnerAIn
	flow.ParseState(&in)
	var out dynamicsubgraphflowapi.InnerAOut
	out.InnerStage, err = svc.InnerA(r.Context(), &flow, in.Value)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInnerB handles marshaling for InnerB.
func (svc *Intermediate) doInnerB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: InnerB
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in dynamicsubgraphflowapi.InnerBIn
	flow.ParseState(&in)
	var out dynamicsubgraphflowapi.InnerBOut
	out.InnerResult, out.InnerDone, err = svc.InnerB(r.Context(), &flow, in.InnerStage)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInner handles marshaling for Inner.
func (svc *Intermediate) doInner(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Inner
	graph, err := svc.Inner(r.Context())
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

// doDynamicSubgraph handles marshaling for DynamicSubgraph.
func (svc *Intermediate) doDynamicSubgraph(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DynamicSubgraph
	graph, err := svc.DynamicSubgraph(r.Context())
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

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

package reducerflow

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

	"github.com/microbus-io/fabric/verify/reducerflow/reducerflowapi"
	"github.com/microbus-io/fabric/verify/reducerflow/resources"
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
	_ reducerflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = reducerflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                                                  // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error)                                            // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error)                                            // MARKER: TaskC
	TaskD(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error)                                            // MARKER: TaskD
	TaskE(ctx context.Context, flow *workflow.Flow, sumTotal int, listTags, setSeen []string) (finalSum int, finalList, finalSet []string, err error)         // MARKER: TaskE
	Reducer(ctx context.Context) (graph *workflow.Graph, err error)                                                                                            // MARKER: Reducer
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
	svc.SetDescription(`reducerflow.verify exercises sum/list/set reducers at fan-in.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(reducerflowapi.TaskA.Method, reducerflowapi.TaskA.Route),
		sub.Description(`TaskA is the fan-out source.`),
		sub.Task(reducerflowapi.TaskAIn{}, reducerflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(reducerflowapi.TaskB.Method, reducerflowapi.TaskB.Route),
		sub.Description(`TaskB contributes deltas to sumTotal/listTags/setSeen.`),
		sub.Task(reducerflowapi.TaskBIn{}, reducerflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(reducerflowapi.TaskC.Method, reducerflowapi.TaskC.Route),
		sub.Description(`TaskC contributes deltas with one overlapping setSeen element.`),
		sub.Task(reducerflowapi.TaskCIn{}, reducerflowapi.TaskCOut{}),
	)
	svc.Subscribe( // MARKER: TaskD
		"TaskD", svc.doTaskD,
		sub.At(reducerflowapi.TaskD.Method, reducerflowapi.TaskD.Route),
		sub.Description(`TaskD contributes deltas.`),
		sub.Task(reducerflowapi.TaskDIn{}, reducerflowapi.TaskDOut{}),
	)
	svc.Subscribe( // MARKER: TaskE
		"TaskE", svc.doTaskE,
		sub.At(reducerflowapi.TaskE.Method, reducerflowapi.TaskE.Route),
		sub.Description(`TaskE reads the reducer-merged values.`),
		sub.Task(reducerflowapi.TaskEIn{}, reducerflowapi.TaskEOut{}),
	)

	svc.Subscribe( // MARKER: Reducer
		"Reducer", svc.doReducer,
		sub.At(reducerflowapi.Reducer.Method, reducerflowapi.Reducer.Route),
		sub.Description(`Reducer defines the graph A -> {B, C, D} -> E with reducer-managed fan-in.`),
		sub.Workflow(reducerflowapi.ReducerIn{}, reducerflowapi.ReducerOut{}),
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
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in reducerflowapi.TaskAIn
	flow.ParseState(&in)
	var out reducerflowapi.TaskAOut
	out.Started, err = svc.TaskA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskB handles marshaling for TaskB.
func (svc *Intermediate) doTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskB
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in reducerflowapi.TaskBIn
	flow.ParseState(&in)
	var out reducerflowapi.TaskBOut
	out.SumTotalOut, out.ListTagsOut, out.SetSeenOut, err = svc.TaskB(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskC handles marshaling for TaskC.
func (svc *Intermediate) doTaskC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskC
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in reducerflowapi.TaskCIn
	flow.ParseState(&in)
	var out reducerflowapi.TaskCOut
	out.SumTotalOut, out.ListTagsOut, out.SetSeenOut, err = svc.TaskC(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskD handles marshaling for TaskD.
func (svc *Intermediate) doTaskD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskD
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in reducerflowapi.TaskDIn
	flow.ParseState(&in)
	var out reducerflowapi.TaskDOut
	out.SumTotalOut, out.ListTagsOut, out.SetSeenOut, err = svc.TaskD(r.Context(), &flow)
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
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in reducerflowapi.TaskEIn
	flow.ParseState(&in)
	var out reducerflowapi.TaskEOut
	out.FinalSum, out.FinalList, out.FinalSet, err = svc.TaskE(r.Context(), &flow, in.SumTotal, in.ListTags, in.SetSeen)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doReducer handles marshaling for Reducer.
func (svc *Intermediate) doReducer(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Reducer
	graph, err := svc.Reducer(r.Context())
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

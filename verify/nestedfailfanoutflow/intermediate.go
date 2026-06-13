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

package nestedfailfanoutflow

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

	"github.com/microbus-io/fabric/verify/nestedfailfanoutflow/nestedfailfanoutflowapi"
	"github.com/microbus-io/fabric/verify/nestedfailfanoutflow/resources"
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
	_ nestedfailfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = nestedfailfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (outers []int, err error)                                  // MARKER: TaskA
	TaskO(ctx context.Context, flow *workflow.Flow, outerItem int) (inners []int, currentOuter int, err error) // MARKER: TaskO
	TaskI(ctx context.Context, flow *workflow.Flow, currentOuter int, innerItem int) (err error)               // MARKER: TaskI
	JoinI(ctx context.Context, flow *workflow.Flow) (err error)                                                // MARKER: JoinI
	JoinO(ctx context.Context, flow *workflow.Flow) (done bool, err error)                                     // MARKER: JoinO
	Nested(ctx context.Context) (graph *workflow.Graph, err error)                                             // MARKER: Nested
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
	svc.SetDescription(`nestedfailfanoutflow.verify exercises 3x3 nested fan-out with a single-cell failure to verify that other branches still execute and the flow fails only when fully resolved.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(nestedfailfanoutflowapi.TaskA.Method, nestedfailfanoutflowapi.TaskA.Route),
		sub.Description(`TaskA is the entry that emits the outer forEach array.`),
		sub.Task(nestedfailfanoutflowapi.TaskAIn{}, nestedfailfanoutflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskO
		"TaskO", svc.doTaskO,
		sub.At(nestedfailfanoutflowapi.TaskO.Method, nestedfailfanoutflowapi.TaskO.Route),
		sub.Description(`TaskO is the per-outer-branch task.`),
		sub.Task(nestedfailfanoutflowapi.TaskOIn{}, nestedfailfanoutflowapi.TaskOOut{}),
	)
	svc.Subscribe( // MARKER: TaskI
		"TaskI", svc.doTaskI,
		sub.At(nestedfailfanoutflowapi.TaskI.Method, nestedfailfanoutflowapi.TaskI.Route),
		sub.Description(`TaskI is the per-(outer, inner) cell task.`),
		sub.Task(nestedfailfanoutflowapi.TaskIIn{}, nestedfailfanoutflowapi.TaskIOut{}),
	)
	svc.Subscribe( // MARKER: JoinI
		"JoinI", svc.doJoinI,
		sub.At(nestedfailfanoutflowapi.JoinI.Method, nestedfailfanoutflowapi.JoinI.Route),
		sub.Description(`JoinI is the inner cohort fan-in.`),
		sub.Task(nestedfailfanoutflowapi.JoinIIn{}, nestedfailfanoutflowapi.JoinIOut{}),
	)
	svc.Subscribe( // MARKER: JoinO
		"JoinO", svc.doJoinO,
		sub.At(nestedfailfanoutflowapi.JoinO.Method, nestedfailfanoutflowapi.JoinO.Route),
		sub.Description(`JoinO is the outer cohort fan-in.`),
		sub.Task(nestedfailfanoutflowapi.JoinOIn{}, nestedfailfanoutflowapi.JoinOOut{}),
	)

	svc.Subscribe( // MARKER: Nested
		"Nested", svc.doNested,
		sub.At(nestedfailfanoutflowapi.Nested.Method, nestedfailfanoutflowapi.Nested.Route),
		sub.Description(`Nested defines the 3x3 nested forEach graph.`),
		sub.Workflow(nestedfailfanoutflowapi.NestedIn{}, nestedfailfanoutflowapi.NestedOut{}),
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
	var in nestedfailfanoutflowapi.TaskAIn
	flow.ParseState(&in)
	var out nestedfailfanoutflowapi.TaskAOut
	out.Outers, err = svc.TaskA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskO handles marshaling for TaskO.
func (svc *Intermediate) doTaskO(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskO
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfailfanoutflowapi.TaskOIn
	flow.ParseState(&in)
	var out nestedfailfanoutflowapi.TaskOOut
	out.Inners, out.CurrentOuter, err = svc.TaskO(r.Context(), &flow, in.OuterItem)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskI handles marshaling for TaskI.
func (svc *Intermediate) doTaskI(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskI
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfailfanoutflowapi.TaskIIn
	flow.ParseState(&in)
	var out nestedfailfanoutflowapi.TaskIOut
	err = svc.TaskI(r.Context(), &flow, in.CurrentOuter, in.InnerItem)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doJoinI handles marshaling for JoinI.
func (svc *Intermediate) doJoinI(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JoinI
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfailfanoutflowapi.JoinIIn
	flow.ParseState(&in)
	var out nestedfailfanoutflowapi.JoinIOut
	err = svc.JoinI(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doJoinO handles marshaling for JoinO.
func (svc *Intermediate) doJoinO(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: JoinO
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfailfanoutflowapi.JoinOIn
	flow.ParseState(&in)
	var out nestedfailfanoutflowapi.JoinOOut
	out.Done, err = svc.JoinO(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doNested handles marshaling for the Nested workflow graph.
func (svc *Intermediate) doNested(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Nested
	graph, err := svc.Nested(r.Context())
	if err != nil {
		return err
	}
	wrapper := struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&wrapper))
}

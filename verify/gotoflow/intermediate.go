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

package gotoflow

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

	"github.com/microbus-io/fabric/verify/gotoflow/gotoflowapi"
	"github.com/microbus-io/fabric/verify/gotoflow/resources"
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
	_ gotoflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = gotoflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error)            // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, loops, target int) (visited bool, err error)    // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error)          // MARKER: TaskC
	BadGotoer(ctx context.Context, flow *workflow.Flow) (stamp bool, err error)                     // MARKER: BadGotoer
	Goto(ctx context.Context) (graph *workflow.Graph, err error)                                    // MARKER: Goto
	BadGoto(ctx context.Context) (graph *workflow.Graph, err error)                                 // MARKER: BadGoto
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
	svc.SetDescription(`gotoflow.verify exercises flow.Goto and AddTransitionGoto.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(gotoflowapi.TaskA.Method, gotoflowapi.TaskA.Route),
		sub.Description(`TaskA increments the loops counter.`),
		sub.Task(gotoflowapi.TaskAIn{}, gotoflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(gotoflowapi.TaskB.Method, gotoflowapi.TaskB.Route),
		sub.Description(`TaskB calls flow.Goto until loops reaches target.`),
		sub.Task(gotoflowapi.TaskBIn{}, gotoflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(gotoflowapi.TaskC.Method, gotoflowapi.TaskC.Route),
		sub.Description(`TaskC surfaces the final loops count.`),
		sub.Task(gotoflowapi.TaskCIn{}, gotoflowapi.TaskCOut{}),
	)

	svc.Subscribe( // MARKER: BadGotoer
		"BadGotoer", svc.doBadGotoer,
		sub.At(gotoflowapi.BadGotoer.Method, gotoflowapi.BadGotoer.Route),
		sub.Description(`BadGotoer requests a goto to an unregistered target.`),
		sub.Task(gotoflowapi.BadGotoerIn{}, gotoflowapi.BadGotoerOut{}),
	)

	svc.Subscribe( // MARKER: Goto
		"Goto", svc.doGoto,
		sub.At(gotoflowapi.Goto.Method, gotoflowapi.Goto.Route),
		sub.Description(`Goto defines A -> B -> C with B -> withGoto -> A.`),
		sub.Workflow(gotoflowapi.GotoIn{}, gotoflowapi.GotoOut{}),
	)
	svc.Subscribe( // MARKER: BadGoto
		"BadGoto", svc.doBadGoto,
		sub.At(gotoflowapi.BadGoto.Method, gotoflowapi.BadGoto.Route),
		sub.Description(`BadGoto defines a single-task graph with an unregistered Goto target.`),
		sub.Workflow(gotoflowapi.BadGotoIn{}, gotoflowapi.BadGotoOut{}),
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
	var in gotoflowapi.TaskAIn
	flow.ParseState(&in)
	var out gotoflowapi.TaskAOut
	out.LoopsOut, err = svc.TaskA(r.Context(), &flow, in.Loops)
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
	var in gotoflowapi.TaskBIn
	flow.ParseState(&in)
	var out gotoflowapi.TaskBOut
	out.Visited, err = svc.TaskB(r.Context(), &flow, in.Loops, in.Target)
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
	var in gotoflowapi.TaskCIn
	flow.ParseState(&in)
	var out gotoflowapi.TaskCOut
	out.FinalLoops, err = svc.TaskC(r.Context(), &flow, in.Loops)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doGoto handles marshaling for Goto.
func (svc *Intermediate) doGoto(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Goto
	graph, err := svc.Goto(r.Context())
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

// doBadGotoer handles marshaling for BadGotoer.
func (svc *Intermediate) doBadGotoer(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BadGotoer
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in gotoflowapi.BadGotoerIn
	flow.ParseState(&in)
	var out gotoflowapi.BadGotoerOut
	out.Stamp, err = svc.BadGotoer(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doBadGoto handles marshaling for BadGoto.
func (svc *Intermediate) doBadGoto(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BadGoto
	graph, err := svc.BadGoto(r.Context())
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

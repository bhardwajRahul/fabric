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

package breakpointflow

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

	"github.com/microbus-io/fabric/verify/breakpointflow/breakpointflowapi"
	"github.com/microbus-io/fabric/verify/breakpointflow/resources"
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
	_ breakpointflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = breakpointflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (stepA bool, err error)                       // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, stepA bool) (stepB bool, err error)           // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, stepB bool) (stepC bool, err error)           // MARKER: TaskC
	Breakpoint(ctx context.Context) (graph *workflow.Graph, err error)                            // MARKER: Breakpoint
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
	svc.SetDescription(`breakpointflow.verify exercises BreakBefore + Resume.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(breakpointflowapi.TaskA.Method, breakpointflowapi.TaskA.Route),
		sub.Description(`TaskA marks stepA as visited.`),
		sub.Task(breakpointflowapi.TaskAIn{}, breakpointflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(breakpointflowapi.TaskB.Method, breakpointflowapi.TaskB.Route),
		sub.Description(`TaskB marks stepB.`),
		sub.Task(breakpointflowapi.TaskBIn{}, breakpointflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(breakpointflowapi.TaskC.Method, breakpointflowapi.TaskC.Route),
		sub.Description(`TaskC marks stepC.`),
		sub.Task(breakpointflowapi.TaskCIn{}, breakpointflowapi.TaskCOut{}),
	)

	svc.Subscribe( // MARKER: Breakpoint
		"Breakpoint", svc.doBreakpoint,
		sub.At(breakpointflowapi.Breakpoint.Method, breakpointflowapi.Breakpoint.Route),
		sub.Description(`Breakpoint defines A -> B -> C.`),
		sub.Workflow(breakpointflowapi.BreakpointIn{}, breakpointflowapi.BreakpointOut{}),
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
	var in breakpointflowapi.TaskAIn
	flow.ParseState(&in)
	var out breakpointflowapi.TaskAOut
	out.StepA, err = svc.TaskA(r.Context(), &flow)
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
	var in breakpointflowapi.TaskBIn
	flow.ParseState(&in)
	var out breakpointflowapi.TaskBOut
	out.StepB, err = svc.TaskB(r.Context(), &flow, in.StepA)
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
	var in breakpointflowapi.TaskCIn
	flow.ParseState(&in)
	var out breakpointflowapi.TaskCOut
	out.StepC, err = svc.TaskC(r.Context(), &flow, in.StepB)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doBreakpoint handles marshaling for Breakpoint.
func (svc *Intermediate) doBreakpoint(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Breakpoint
	graph, err := svc.Breakpoint(r.Context())
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

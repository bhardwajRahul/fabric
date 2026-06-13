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

package onerrorsiblingsflow

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

	"github.com/microbus-io/fabric/verify/onerrorsiblingsflow/onerrorsiblingsflowapi"
	"github.com/microbus-io/fabric/verify/onerrorsiblingsflow/resources"
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
	_ onerrorsiblingsflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = onerrorsiblingsflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                                          // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, started bool) (markB bool, err error)                                                              // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, started bool) (markC bool, err error)                                                              // MARKER: TaskC
	TaskD(ctx context.Context, flow *workflow.Flow, started bool) (markD bool, err error)                                                              // MARKER: TaskD
	Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (handled bool, err error)                                             // MARKER: Handler
	TaskE(ctx context.Context, flow *workflow.Flow, handled, markB, markC, markD bool) (recovered, siblingsRan bool, err error)                       // MARKER: TaskE
	FanOutError(ctx context.Context) (graph *workflow.Graph, err error)                                                                                // MARKER: FanOutError
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
	svc.SetDescription(`onerrorsiblingsflow.verify exercises OnError + fan-out: A -> {B, C, D} -> E with B onError -> Handler -> E.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(onerrorsiblingsflowapi.TaskA.Method, onerrorsiblingsflowapi.TaskA.Route),
		sub.Description(`TaskA is the fan-out source.`),
		sub.Task(onerrorsiblingsflowapi.TaskAIn{}, onerrorsiblingsflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(onerrorsiblingsflowapi.TaskB.Method, onerrorsiblingsflowapi.TaskB.Route),
		sub.Description(`TaskB always errors, triggering its onError transition.`),
		sub.Task(onerrorsiblingsflowapi.TaskBIn{}, onerrorsiblingsflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(onerrorsiblingsflowapi.TaskC.Method, onerrorsiblingsflowapi.TaskC.Route),
		sub.Description(`TaskC is a normal sibling.`),
		sub.Task(onerrorsiblingsflowapi.TaskCIn{}, onerrorsiblingsflowapi.TaskCOut{}),
	)
	svc.Subscribe( // MARKER: TaskD
		"TaskD", svc.doTaskD,
		sub.At(onerrorsiblingsflowapi.TaskD.Method, onerrorsiblingsflowapi.TaskD.Route),
		sub.Description(`TaskD is a normal sibling.`),
		sub.Task(onerrorsiblingsflowapi.TaskDIn{}, onerrorsiblingsflowapi.TaskDOut{}),
	)
	svc.Subscribe( // MARKER: Handler
		"Handler", svc.doHandler,
		sub.At(onerrorsiblingsflowapi.Handler.Method, onerrorsiblingsflowapi.Handler.Route),
		sub.Description(`Handler handles TaskB's error.`),
		sub.Task(onerrorsiblingsflowapi.HandlerIn{}, onerrorsiblingsflowapi.HandlerOut{}),
	)
	svc.Subscribe( // MARKER: TaskE
		"TaskE", svc.doTaskE,
		sub.At(onerrorsiblingsflowapi.TaskE.Method, onerrorsiblingsflowapi.TaskE.Route),
		sub.Description(`TaskE is the fan-in target.`),
		sub.Task(onerrorsiblingsflowapi.TaskEIn{}, onerrorsiblingsflowapi.TaskEOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: FanOutError
		"FanOutError", svc.doFanOutError,
		sub.At(onerrorsiblingsflowapi.FanOutError.Method, onerrorsiblingsflowapi.FanOutError.Route),
		sub.Description(`FanOutError defines the graph A -> {B, C, D} -> E with B onError -> Handler -> E.`),
		sub.Workflow(onerrorsiblingsflowapi.FanOutErrorIn{}, onerrorsiblingsflowapi.FanOutErrorOut{}),
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

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.TaskAIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.TaskAOut
	out.Started, err = svc.TaskA(r.Context(), &flow)
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

// doTaskB handles marshaling for TaskB.
func (svc *Intermediate) doTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskB
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.TaskBIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.TaskBOut
	out.MarkB, err = svc.TaskB(r.Context(), &flow, in.Started)
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

// doTaskC handles marshaling for TaskC.
func (svc *Intermediate) doTaskC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskC
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.TaskCIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.TaskCOut
	out.MarkC, err = svc.TaskC(r.Context(), &flow, in.Started)
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

// doTaskD handles marshaling for TaskD.
func (svc *Intermediate) doTaskD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskD
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.TaskDIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.TaskDOut
	out.MarkD, err = svc.TaskD(r.Context(), &flow, in.Started)
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

// doHandler handles marshaling for Handler.
func (svc *Intermediate) doHandler(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Handler
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.HandlerIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.HandlerOut
	out.Handled, err = svc.Handler(r.Context(), &flow, in.OnErr)
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

// doTaskE handles marshaling for TaskE.
func (svc *Intermediate) doTaskE(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskE
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in onerrorsiblingsflowapi.TaskEIn
	flow.ParseState(&in)
	var out onerrorsiblingsflowapi.TaskEOut
	out.Recovered, out.SiblingsRan, err = svc.TaskE(r.Context(), &flow, in.Handled, in.MarkB, in.MarkC, in.MarkD)
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

// doFanOutError handles marshaling for FanOutError.
func (svc *Intermediate) doFanOutError(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: FanOutError
	graph, err := svc.FanOutError(r.Context())
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

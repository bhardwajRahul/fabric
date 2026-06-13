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

package subgraphentryflow

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

	"github.com/microbus-io/fabric/verify/subgraphentryflow/resources"
	"github.com/microbus-io/fabric/verify/subgraphentryflow/subgraphentryflowapi"
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
	_ subgraphentryflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = subgraphentryflowapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskInner(ctx context.Context, flow *workflow.Flow) (innerResult string, err error)                            // MARKER: TaskInner
	TaskTail(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error)        // MARKER: TaskTail
	RunInner(ctx context.Context, flow *workflow.Flow) (innerResult string, err error)                             // MARKER: RunInner
	RunTail(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error)         // MARKER: RunTail
	Inner(ctx context.Context) (graph *workflow.Graph, err error)                                                  // MARKER: Inner
	Tail(ctx context.Context) (graph *workflow.Graph, err error)                                                   // MARKER: Tail
	Outer(ctx context.Context) (graph *workflow.Graph, err error)                                                  // MARKER: Outer
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
	svc.SetDescription(`subgraphentryflow.verify exercises a subgraph as both the first and last node of a workflow graph.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskInner
		"TaskInner", svc.doTaskInner,
		sub.At(subgraphentryflowapi.TaskInner.Method, subgraphentryflowapi.TaskInner.Route),
		sub.Description(`TaskInner is the sole task of the Inner subgraph.`),
		sub.Task(subgraphentryflowapi.TaskInnerIn{}, subgraphentryflowapi.TaskInnerOut{}),
	)
	svc.Subscribe( // MARKER: TaskTail
		"TaskTail", svc.doTaskTail,
		sub.At(subgraphentryflowapi.TaskTail.Method, subgraphentryflowapi.TaskTail.Route),
		sub.Description(`TaskTail is the sole task of the Tail subgraph; it reads the upstream subgraph's output.`),
		sub.Task(subgraphentryflowapi.TaskTailIn{}, subgraphentryflowapi.TaskTailOut{}),
	)

	svc.Subscribe( // MARKER: RunInner
		"RunInner", svc.doRunInner,
		sub.At(subgraphentryflowapi.RunInner.Method, subgraphentryflowapi.RunInner.Route),
		sub.Description(`RunInner invokes the Inner subgraph via flow.Subgraph and adopts its innerResult.`),
		sub.Task(subgraphentryflowapi.RunInnerIn{}, subgraphentryflowapi.RunInnerOut{}),
	)
	svc.Subscribe( // MARKER: RunTail
		"RunTail", svc.doRunTail,
		sub.At(subgraphentryflowapi.RunTail.Method, subgraphentryflowapi.RunTail.Route),
		sub.Description(`RunTail invokes the Tail subgraph via flow.Subgraph and adopts its finalResult.`),
		sub.Task(subgraphentryflowapi.RunTailIn{}, subgraphentryflowapi.RunTailOut{}),
	)

	svc.Subscribe( // MARKER: Inner
		"Inner", svc.doInner,
		sub.At(subgraphentryflowapi.Inner.Method, subgraphentryflowapi.Inner.Route),
		sub.Description(`Inner defines the single-task subgraph taskInner -> END.`),
		sub.Workflow(subgraphentryflowapi.InnerIn{}, subgraphentryflowapi.InnerOut{}),
	)
	svc.Subscribe( // MARKER: Tail
		"Tail", svc.doTail,
		sub.At(subgraphentryflowapi.Tail.Method, subgraphentryflowapi.Tail.Route),
		sub.Description(`Tail defines the single-task subgraph taskTail -> END.`),
		sub.Workflow(subgraphentryflowapi.TailIn{}, subgraphentryflowapi.TailOut{}),
	)
	svc.Subscribe( // MARKER: Outer
		"Outer", svc.doOuter,
		sub.At(subgraphentryflowapi.Outer.Method, subgraphentryflowapi.Outer.Route),
		sub.Description(`Outer defines the coordinator-shape graph inner -> tail -> END, where both nodes are subgraphs.`),
		sub.Workflow(subgraphentryflowapi.OuterIn{}, subgraphentryflowapi.OuterOut{}),
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

// doTaskInner handles marshaling for TaskInner.
func (svc *Intermediate) doTaskInner(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskInner
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphentryflowapi.TaskInnerIn
	flow.ParseState(&in)
	var out subgraphentryflowapi.TaskInnerOut
	out.InnerResult, err = svc.TaskInner(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskTail handles marshaling for TaskTail.
func (svc *Intermediate) doTaskTail(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskTail
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphentryflowapi.TaskTailIn
	flow.ParseState(&in)
	var out subgraphentryflowapi.TaskTailOut
	out.FinalResult, err = svc.TaskTail(r.Context(), &flow, in.InnerResult)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRunInner handles marshaling for RunInner.
func (svc *Intermediate) doRunInner(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RunInner
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphentryflowapi.RunInnerIn
	flow.ParseState(&in)
	var out subgraphentryflowapi.RunInnerOut
	out.InnerResult, err = svc.RunInner(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRunTail handles marshaling for RunTail.
func (svc *Intermediate) doRunTail(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RunTail
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphentryflowapi.RunTailIn
	flow.ParseState(&in)
	var out subgraphentryflowapi.RunTailOut
	out.FinalResult, err = svc.RunTail(r.Context(), &flow, in.InnerResult)
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

// doTail handles marshaling for Tail.
func (svc *Intermediate) doTail(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Tail
	graph, err := svc.Tail(r.Context())
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

// doOuter handles marshaling for Outer.
func (svc *Intermediate) doOuter(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Outer
	graph, err := svc.Outer(r.Context())
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

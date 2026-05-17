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

package perelementpipelineflow

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

	"github.com/microbus-io/fabric/verify/perelementpipelineflow/perelementpipelineflowapi"
	"github.com/microbus-io/fabric/verify/perelementpipelineflow/resources"
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
	_ perelementpipelineflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = perelementpipelineflowapi.Hostname
	Version  = 3
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskS(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error)                          // MARKER: TaskS
	TaskH(ctx context.Context, flow *workflow.Flow, item string) (itemUpper string, err error)                              // MARKER: TaskH
	TaskA(ctx context.Context, flow *workflow.Flow, itemUpper string) (aProcessed string, err error)                        // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, itemUpper string) (bProcessed string, err error)                        // MARKER: TaskB
	TaskM(ctx context.Context, flow *workflow.Flow, aProcessed, bProcessed string) (setMerged []string, err error)          // MARKER: TaskM
	TaskL(ctx context.Context, flow *workflow.Flow, setMerged []string) (finalCount int, err error)                         // MARKER: TaskL
	PerElementPipeline(ctx context.Context) (graph *workflow.Graph, err error)                                              // MARKER: PerElementPipeline
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
	svc.SetDescription(`perelementpipelineflow.verify is the SKIP-marked per-element pipeline pattern (lineage redesign required).`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskS
		"TaskS", svc.doTaskS,
		sub.At(perelementpipelineflowapi.TaskS.Method, perelementpipelineflowapi.TaskS.Route),
		sub.Description(`TaskS passes items through.`),
		sub.Task(perelementpipelineflowapi.TaskSIn{}, perelementpipelineflowapi.TaskSOut{}),
	)
	svc.Subscribe( // MARKER: TaskH
		"TaskH", svc.doTaskH,
		sub.At(perelementpipelineflowapi.TaskH.Method, perelementpipelineflowapi.TaskH.Route),
		sub.Description(`TaskH uppercases the per-element item.`),
		sub.Task(perelementpipelineflowapi.TaskHIn{}, perelementpipelineflowapi.TaskHOut{}),
	)
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(perelementpipelineflowapi.TaskA.Method, perelementpipelineflowapi.TaskA.Route),
		sub.Description(`TaskA is one parallel branch in the per-element inner pipeline.`),
		sub.Task(perelementpipelineflowapi.TaskAIn{}, perelementpipelineflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(perelementpipelineflowapi.TaskB.Method, perelementpipelineflowapi.TaskB.Route),
		sub.Description(`TaskB is the other parallel branch.`),
		sub.Task(perelementpipelineflowapi.TaskBIn{}, perelementpipelineflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskM
		"TaskM", svc.doTaskM,
		sub.At(perelementpipelineflowapi.TaskM.Method, perelementpipelineflowapi.TaskM.Route),
		sub.Description(`TaskM is the per-element fan-in.`),
		sub.Task(perelementpipelineflowapi.TaskMIn{}, perelementpipelineflowapi.TaskMOut{}),
	)
	svc.Subscribe( // MARKER: TaskL
		"TaskL", svc.doTaskL,
		sub.At(perelementpipelineflowapi.TaskL.Method, perelementpipelineflowapi.TaskL.Route),
		sub.Description(`TaskL is the outer fan-in.`),
		sub.Task(perelementpipelineflowapi.TaskLIn{}, perelementpipelineflowapi.TaskLOut{}),
	)

	svc.Subscribe( // MARKER: PerElementPipeline
		"PerElementPipeline", svc.doPerElementPipeline,
		sub.At(perelementpipelineflowapi.PerElementPipeline.Method, perelementpipelineflowapi.PerElementPipeline.Route),
		sub.Description(`PerElementPipeline defines S -> forEach -> H -> {A, B} -> M -> L.`),
		sub.Workflow(perelementpipelineflowapi.PerElementPipelineIn{}, perelementpipelineflowapi.PerElementPipelineOut{}),
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

// doTaskS handles marshaling for TaskS.
func (svc *Intermediate) doTaskS(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskS
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in perelementpipelineflowapi.TaskSIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskSOut
	out.ItemsOut, err = svc.TaskS(r.Context(), &flow, in.Items)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskH handles marshaling for TaskH.
func (svc *Intermediate) doTaskH(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskH
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in perelementpipelineflowapi.TaskHIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskHOut
	out.ItemUpper, err = svc.TaskH(r.Context(), &flow, in.Item)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in perelementpipelineflowapi.TaskAIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskAOut
	out.AProcessed, err = svc.TaskA(r.Context(), &flow, in.ItemUpper)
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
	var in perelementpipelineflowapi.TaskBIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskBOut
	out.BProcessed, err = svc.TaskB(r.Context(), &flow, in.ItemUpper)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskM handles marshaling for TaskM.
func (svc *Intermediate) doTaskM(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskM
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in perelementpipelineflowapi.TaskMIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskMOut
	out.SetMerged, err = svc.TaskM(r.Context(), &flow, in.AProcessed, in.BProcessed)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskL handles marshaling for TaskL.
func (svc *Intermediate) doTaskL(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskL
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in perelementpipelineflowapi.TaskLIn
	flow.ParseState(&in)
	var out perelementpipelineflowapi.TaskLOut
	out.FinalCount, err = svc.TaskL(r.Context(), &flow, in.SetMerged)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doPerElementPipeline handles marshaling for PerElementPipeline.
func (svc *Intermediate) doPerElementPipeline(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: PerElementPipeline
	graph, err := svc.PerElementPipeline(r.Context())
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

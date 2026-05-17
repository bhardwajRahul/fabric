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

package dynamicfanoutflow

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

	"github.com/microbus-io/fabric/verify/dynamicfanoutflow/dynamicfanoutflowapi"
	"github.com/microbus-io/fabric/verify/dynamicfanoutflow/resources"
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
	_ dynamicfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = dynamicfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, item string) (sumProcessedOut int, err error)  // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, sumProcessed int) (processedCount int, err error) // MARKER: TaskC
	DynamicFanOut(ctx context.Context) (graph *workflow.Graph, err error)                          // MARKER: DynamicFanOut
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
	svc.SetDescription(`dynamicfanoutflow.verify exercises forEach dynamic fan-out: A -> forEach(items) -> B -> C.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(dynamicfanoutflowapi.TaskA.Method, dynamicfanoutflowapi.TaskA.Route),
		sub.Description(`TaskA is the forEach source.`),
		sub.Task(dynamicfanoutflowapi.TaskAIn{}, dynamicfanoutflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(dynamicfanoutflowapi.TaskB.Method, dynamicfanoutflowapi.TaskB.Route),
		sub.Description(`TaskB runs once per forEach element.`),
		sub.Task(dynamicfanoutflowapi.TaskBIn{}, dynamicfanoutflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(dynamicfanoutflowapi.TaskC.Method, dynamicfanoutflowapi.TaskC.Route),
		sub.Description(`TaskC is the fan-in target.`),
		sub.Task(dynamicfanoutflowapi.TaskCIn{}, dynamicfanoutflowapi.TaskCOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: DynamicFanOut
		"DynamicFanOut", svc.doDynamicFanOut,
		sub.At(dynamicfanoutflowapi.DynamicFanOut.Method, dynamicfanoutflowapi.DynamicFanOut.Route),
		sub.Description(`DynamicFanOut defines the graph: A -> forEach(items) -> B -> C.`),
		sub.Workflow(dynamicfanoutflowapi.DynamicFanOutIn{}, dynamicfanoutflowapi.DynamicFanOutOut{}),
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
	var in dynamicfanoutflowapi.TaskAIn
	flow.ParseState(&in)
	var out dynamicfanoutflowapi.TaskAOut
	out.ItemsOut, err = svc.TaskA(r.Context(), &flow, in.Items)
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
	var in dynamicfanoutflowapi.TaskBIn
	flow.ParseState(&in)
	var out dynamicfanoutflowapi.TaskBOut
	out.SumProcessedOut, err = svc.TaskB(r.Context(), &flow, in.Item)
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
	var in dynamicfanoutflowapi.TaskCIn
	flow.ParseState(&in)
	var out dynamicfanoutflowapi.TaskCOut
	out.ProcessedCount, err = svc.TaskC(r.Context(), &flow, in.SumProcessed)
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

// doDynamicFanOut handles marshaling for DynamicFanOut.
func (svc *Intermediate) doDynamicFanOut(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DynamicFanOut
	graph, err := svc.DynamicFanOut(r.Context())
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

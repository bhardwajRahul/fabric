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

package superflow

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

	"github.com/microbus-io/fabric/verify/superflow/resources"
	"github.com/microbus-io/fabric/verify/superflow/superflowapi"
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
	_ superflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = superflowapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskC
	TaskD(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskD
	TaskE(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskE
	TaskZ(ctx context.Context, flow *workflow.Flow) (err error)         // MARKER: TaskZ
	ErrorHandler(ctx context.Context, flow *workflow.Flow) (err error)  // MARKER: ErrorHandler
	SubTaskA(ctx context.Context, flow *workflow.Flow) (err error)      // MARKER: SubTaskA
	SubTaskB(ctx context.Context, flow *workflow.Flow) (err error)      // MARKER: SubTaskB
	RunSuperSub(ctx context.Context, flow *workflow.Flow) (err error)   // MARKER: RunSuperSub
	Super(ctx context.Context) (graph *workflow.Graph, err error)       // MARKER: Super
	SuperSub(ctx context.Context) (graph *workflow.Graph, err error)    // MARKER: SuperSub
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
	svc.SetDescription(`superflow.verify is a unified workflow fixture covering every transition primitive in a single graph, with per-task behavior driven by state.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(superflowapi.TaskA.Method, superflowapi.TaskA.Route),
		sub.Description(`TaskA is the entry of the Super graph.`),
		sub.Task(superflowapi.TaskAIn{}, superflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(superflowapi.TaskB.Method, superflowapi.TaskB.Route),
		sub.Description(`TaskB is the forEach source.`),
		sub.Task(superflowapi.TaskBIn{}, superflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(superflowapi.TaskC.Method, superflowapi.TaskC.Route),
		sub.Description(`TaskC runs once per forEach element and converges into TaskD's fan-in.`),
		sub.Task(superflowapi.TaskCIn{}, superflowapi.TaskCOut{}),
	)
	svc.Subscribe( // MARKER: TaskD
		"TaskD", svc.doTaskD,
		sub.At(superflowapi.TaskD.Method, superflowapi.TaskD.Route),
		sub.Description(`TaskD is the forEach fan-in target with a conditional fan-out.`),
		sub.Task(superflowapi.TaskDIn{}, superflowapi.TaskDOut{}),
	)
	svc.Subscribe( // MARKER: TaskE
		"TaskE", svc.doTaskE,
		sub.At(superflowapi.TaskE.Method, superflowapi.TaskE.Route),
		sub.Description(`TaskE is the conditional fan-in target; supports a goto to TaskZ.`),
		sub.Task(superflowapi.TaskEIn{}, superflowapi.TaskEOut{}),
	)
	svc.Subscribe( // MARKER: TaskZ
		"TaskZ", svc.doTaskZ,
		sub.At(superflowapi.TaskZ.Method, superflowapi.TaskZ.Route),
		sub.Description(`TaskZ is the goto target out of TaskE.`),
		sub.Task(superflowapi.TaskZIn{}, superflowapi.TaskZOut{}),
	)
	svc.Subscribe( // MARKER: ErrorHandler
		"ErrorHandler", svc.doErrorHandler,
		sub.At(superflowapi.ErrorHandler.Method, superflowapi.ErrorHandler.Route),
		sub.Description(`ErrorHandler handles errors raised by TaskC and rejoins the fan-in.`),
		sub.Task(superflowapi.ErrorHandlerIn{}, superflowapi.ErrorHandlerOut{}),
	)
	svc.Subscribe( // MARKER: SubTaskA
		"SubTaskA", svc.doSubTaskA,
		sub.At(superflowapi.SubTaskA.Method, superflowapi.SubTaskA.Route),
		sub.Description(`SubTaskA is the entry of the SuperSub subgraph.`),
		sub.Task(superflowapi.SubTaskAIn{}, superflowapi.SubTaskAOut{}),
	)
	svc.Subscribe( // MARKER: SubTaskB
		"SubTaskB", svc.doSubTaskB,
		sub.At(superflowapi.SubTaskB.Method, superflowapi.SubTaskB.Route),
		sub.Description(`SubTaskB is the second step of the SuperSub subgraph.`),
		sub.Task(superflowapi.SubTaskBIn{}, superflowapi.SubTaskBOut{}),
	)

	svc.Subscribe( // MARKER: RunSuperSub
		"RunSuperSub", svc.doRunSuperSub,
		sub.At(superflowapi.RunSuperSub.Method, superflowapi.RunSuperSub.Route),
		sub.Description(`RunSuperSub invokes the SuperSub subgraph via flow.Subgraph.`),
		sub.Task(superflowapi.RunSuperSubIn{}, superflowapi.RunSuperSubOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: Super
		"Super", svc.doSuper,
		sub.At(superflowapi.Super.Method, superflowapi.Super.Route),
		sub.Description(`Super is the unified workflow graph covering every transition primitive.`),
		sub.Workflow(superflowapi.SuperIn{}, superflowapi.SuperOut{}),
	)
	svc.Subscribe( // MARKER: SuperSub
		"SuperSub", svc.doSuperSub,
		sub.At(superflowapi.SuperSub.Method, superflowapi.SuperSub.Route),
		sub.Description(`SuperSub is the nested subgraph used by the Super workflow's conditional branch.`),
		sub.Workflow(superflowapi.SuperSubIn{}, superflowapi.SuperSubOut{}),
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

// doTask is the shared marshaler shape: every superflow task has empty In/Out
// and reads its real inputs from flow state.
func doTask(w http.ResponseWriter, r *http.Request, fn func(context.Context, *workflow.Flow) error) (err error) {
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	err = fn(r.Context(), &flow)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(struct{}{}, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	return doTask(w, r, svc.TaskA)
}

// doTaskB handles marshaling for TaskB.
func (svc *Intermediate) doTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskB
	return doTask(w, r, svc.TaskB)
}

// doTaskC handles marshaling for TaskC.
func (svc *Intermediate) doTaskC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskC
	return doTask(w, r, svc.TaskC)
}

// doTaskD handles marshaling for TaskD.
func (svc *Intermediate) doTaskD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskD
	return doTask(w, r, svc.TaskD)
}

// doTaskE handles marshaling for TaskE.
func (svc *Intermediate) doTaskE(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskE
	return doTask(w, r, svc.TaskE)
}

// doTaskZ handles marshaling for TaskZ.
func (svc *Intermediate) doTaskZ(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskZ
	return doTask(w, r, svc.TaskZ)
}

// doErrorHandler handles marshaling for ErrorHandler.
func (svc *Intermediate) doErrorHandler(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ErrorHandler
	return doTask(w, r, svc.ErrorHandler)
}

// doRunSuperSub handles marshaling for RunSuperSub.
func (svc *Intermediate) doRunSuperSub(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RunSuperSub
	return doTask(w, r, svc.RunSuperSub)
}

// doSubTaskA handles marshaling for SubTaskA.
func (svc *Intermediate) doSubTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SubTaskA
	return doTask(w, r, svc.SubTaskA)
}

// doSubTaskB handles marshaling for SubTaskB.
func (svc *Intermediate) doSubTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SubTaskB
	return doTask(w, r, svc.SubTaskB)
}

// doSuper handles marshaling for the Super workflow.
func (svc *Intermediate) doSuper(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Super
	graph, err := svc.Super(r.Context())
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

// doSuperSub handles marshaling for the SuperSub workflow.
func (svc *Intermediate) doSuperSub(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SuperSub
	graph, err := svc.SuperSub(r.Context())
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

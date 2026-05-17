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

package retryflow

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

	"github.com/microbus-io/fabric/verify/retryflow/resources"
	"github.com/microbus-io/fabric/verify/retryflow/retryflowapi"
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
	_ retryflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = retryflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error)            // MARKER: TaskA
	Flaky(ctx context.Context, flow *workflow.Flow, attempts, target int) (attemptsOut int, err error) // MARKER: Flaky
	TaskB(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error)      // MARKER: TaskB
	Retry(ctx context.Context) (graph *workflow.Graph, err error)                                     // MARKER: Retry
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
	svc.SetDescription(`retryflow.verify exercises flow.Retry with backoff and exhaustion.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(retryflowapi.TaskA.Method, retryflowapi.TaskA.Route),
		sub.Description(`TaskA passes the target through.`),
		sub.Task(retryflowapi.TaskAIn{}, retryflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: Flaky
		"Flaky", svc.doFlaky,
		sub.At(retryflowapi.Flaky.Method, retryflowapi.Flaky.Route),
		sub.Description(`Flaky increments attempts and calls flow.Retry while attempts<target.`),
		sub.Task(retryflowapi.FlakyIn{}, retryflowapi.FlakyOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(retryflowapi.TaskB.Method, retryflowapi.TaskB.Route),
		sub.Description(`TaskB surfaces the final attempts count.`),
		sub.Task(retryflowapi.TaskBIn{}, retryflowapi.TaskBOut{}),
	)

	svc.Subscribe( // MARKER: Retry
		"Retry", svc.doRetry,
		sub.At(retryflowapi.Retry.Method, retryflowapi.Retry.Route),
		sub.Description(`Retry defines A -> Flaky -> B.`),
		sub.Workflow(retryflowapi.RetryIn{}, retryflowapi.RetryOut{}),
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
	var in retryflowapi.TaskAIn
	flow.ParseState(&in)
	var out retryflowapi.TaskAOut
	out.TargetOut, err = svc.TaskA(r.Context(), &flow, in.Target)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doFlaky handles marshaling for Flaky.
func (svc *Intermediate) doFlaky(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Flaky
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryflowapi.FlakyIn
	flow.ParseState(&in)
	var out retryflowapi.FlakyOut
	out.AttemptsOut, err = svc.Flaky(r.Context(), &flow, in.Attempts, in.Target)
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
	var in retryflowapi.TaskBIn
	flow.ParseState(&in)
	var out retryflowapi.TaskBOut
	out.FinalAttempts, err = svc.TaskB(r.Context(), &flow, in.Attempts)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRetry handles marshaling for Retry.
func (svc *Intermediate) doRetry(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Retry
	graph, err := svc.Retry(r.Context())
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

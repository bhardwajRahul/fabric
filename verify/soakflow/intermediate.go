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

package soakflow

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

	"github.com/microbus-io/fabric/verify/soakflow/resources"
	"github.com/microbus-io/fabric/verify/soakflow/soakflowapi"
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
	_ soakflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = soakflowapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Seed(ctx context.Context, flow *workflow.Flow) (done bool, err error)       // MARKER: Seed
	FanA(ctx context.Context, flow *workflow.Flow) (done bool, err error)       // MARKER: FanA
	Work(ctx context.Context, flow *workflow.Flow) (done bool, err error)       // MARKER: Work
	Collect(ctx context.Context, flow *workflow.Flow) (done bool, err error)    // MARKER: Collect
	Loop(ctx context.Context, flow *workflow.Flow) (done bool, err error)       // MARKER: Loop
	BoomR(ctx context.Context, flow *workflow.Flow) (done bool, err error)      // MARKER: BoomR
	Recover(ctx context.Context, flow *workflow.Flow) (done bool, err error)    // MARKER: Recover
	BoomF(ctx context.Context, flow *workflow.Flow) (done bool, err error)      // MARKER: BoomF
	Join(ctx context.Context, flow *workflow.Flow) (done bool, err error)       // MARKER: Join
	InnerEntry(ctx context.Context, flow *workflow.Flow) (done bool, err error) // MARKER: InnerEntry
	RunSub(ctx context.Context, flow *workflow.Flow) (done bool, err error)     // MARKER: RunSub
	Soak(ctx context.Context) (graph *workflow.Graph, err error)                // MARKER: Soak
	Inner(ctx context.Context) (graph *workflow.Graph, err error)               // MARKER: Inner
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
	svc.SetDescription(`soakflow.verify is a high-volume liveness soak over a complex input-driven workflow.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: Seed
		"Seed", svc.doSeed,
		sub.At(soakflowapi.Seed.Method, soakflowapi.Seed.Route),
		sub.Description(`Seed normalizes and clamps the random inputs into bounded workflow state.`),
		sub.Task(soakflowapi.SeedIn{}, soakflowapi.SeedOut{}),
	)
	svc.Subscribe( // MARKER: FanA
		"FanA", svc.doFanA,
		sub.At(soakflowapi.FanA.Method, soakflowapi.FanA.Route),
		sub.Description(`FanA is the dynamic fan-out source.`),
		sub.Task(soakflowapi.FanAIn{}, soakflowapi.FanAOut{}),
	)
	svc.Subscribe( // MARKER: Work
		"Work", svc.doWork,
		sub.At(soakflowapi.Work.Method, soakflowapi.Work.Route),
		sub.Description(`Work processes one fan-out element and contributes a sum-reducer delta.`),
		sub.Task(soakflowapi.WorkIn{}, soakflowapi.WorkOut{}),
	)
	svc.Subscribe( // MARKER: Collect
		"Collect", svc.doCollect,
		sub.At(soakflowapi.Collect.Method, soakflowapi.Collect.Route),
		sub.Description(`Collect is the fan-in target that merges the fan-out contributions.`),
		sub.Task(soakflowapi.CollectIn{}, soakflowapi.CollectOut{}),
	)
	svc.Subscribe( // MARKER: Loop
		"Loop", svc.doLoop,
		sub.At(soakflowapi.Loop.Method, soakflowapi.Loop.Route),
		sub.Description(`Loop decrements a bounded counter and gotos itself until it reaches zero.`),
		sub.Task(soakflowapi.LoopIn{}, soakflowapi.LoopOut{}),
	)
	svc.Subscribe( // MARKER: BoomR
		"BoomR", svc.doBoomR,
		sub.At(soakflowapi.BoomR.Method, soakflowapi.BoomR.Route),
		sub.Description(`BoomR always errors; its onError transition routes to Recover.`),
		sub.Task(soakflowapi.BoomRIn{}, soakflowapi.BoomROut{}),
	)
	svc.Subscribe( // MARKER: Recover
		"Recover", svc.doRecover,
		sub.At(soakflowapi.Recover.Method, soakflowapi.Recover.Route),
		sub.Description(`Recover is the onError handler that resumes the flow to completion.`),
		sub.Task(soakflowapi.RecoverIn{}, soakflowapi.RecoverOut{}),
	)
	svc.Subscribe( // MARKER: BoomF
		"BoomF", svc.doBoomF,
		sub.At(soakflowapi.BoomF.Method, soakflowapi.BoomF.Route),
		sub.Description(`BoomF always errors and has no error transition, so the flow fails.`),
		sub.Task(soakflowapi.BoomFIn{}, soakflowapi.BoomFOut{}),
	)
	svc.Subscribe( // MARKER: Join
		"Join", svc.doJoin,
		sub.At(soakflowapi.Join.Method, soakflowapi.Join.Route),
		sub.Description(`Join is the convergence point before the workflow ends.`),
		sub.Task(soakflowapi.JoinIn{}, soakflowapi.JoinOut{}),
	)
	svc.Subscribe( // MARKER: InnerEntry
		"InnerEntry", svc.doInnerEntry,
		sub.At(soakflowapi.InnerEntry.Method, soakflowapi.InnerEntry.Route),
		sub.Description(`InnerEntry is the single task of the Inner subgraph.`),
		sub.Task(soakflowapi.InnerEntryIn{}, soakflowapi.InnerEntryOut{}),
	)
	svc.Subscribe( // MARKER: RunSub
		"RunSub", svc.doRunSub,
		sub.At(soakflowapi.RunSub.Method, soakflowapi.RunSub.Route),
		sub.Description(`RunSub invokes the Inner subgraph via flow.Subgraph.`),
		sub.Task(soakflowapi.RunSubIn{}, soakflowapi.RunSubOut{}),
	)
	svc.Subscribe( // MARKER: Soak
		"Soak", svc.doSoak,
		sub.At(soakflowapi.Soak.Method, soakflowapi.Soak.Route),
		sub.Description(`Soak defines the complex, input-driven workflow graph exercised under volume.`),
		sub.Workflow(soakflowapi.SoakIn{}, soakflowapi.SoakOut{}),
	)
	svc.Subscribe( // MARKER: Inner
		"Inner", svc.doInner,
		sub.At(soakflowapi.Inner.Method, soakflowapi.Inner.Route),
		sub.Description(`Inner defines the subgraph invoked by Soak.`),
		sub.Workflow(soakflowapi.InnerIn{}, soakflowapi.InnerOut{}),
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

// doSeed handles marshaling for Seed.
func (svc *Intermediate) doSeed(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Seed
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.SeedIn
	flow.ParseState(&in)
	var out soakflowapi.SeedOut
	out.Done, err = svc.Seed(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doFanA handles marshaling for FanA.
func (svc *Intermediate) doFanA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: FanA
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.FanAIn
	flow.ParseState(&in)
	var out soakflowapi.FanAOut
	out.Done, err = svc.FanA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doWork handles marshaling for Work.
func (svc *Intermediate) doWork(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Work
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.WorkIn
	flow.ParseState(&in)
	var out soakflowapi.WorkOut
	out.Done, err = svc.Work(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doCollect handles marshaling for Collect.
func (svc *Intermediate) doCollect(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Collect
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.CollectIn
	flow.ParseState(&in)
	var out soakflowapi.CollectOut
	out.Done, err = svc.Collect(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doLoop handles marshaling for Loop.
func (svc *Intermediate) doLoop(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Loop
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.LoopIn
	flow.ParseState(&in)
	var out soakflowapi.LoopOut
	out.Done, err = svc.Loop(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doBoomR handles marshaling for BoomR.
func (svc *Intermediate) doBoomR(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BoomR
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.BoomRIn
	flow.ParseState(&in)
	var out soakflowapi.BoomROut
	out.Done, err = svc.BoomR(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRecover handles marshaling for Recover.
func (svc *Intermediate) doRecover(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Recover
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.RecoverIn
	flow.ParseState(&in)
	var out soakflowapi.RecoverOut
	out.Done, err = svc.Recover(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doBoomF handles marshaling for BoomF.
func (svc *Intermediate) doBoomF(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BoomF
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.BoomFIn
	flow.ParseState(&in)
	var out soakflowapi.BoomFOut
	out.Done, err = svc.BoomF(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doJoin handles marshaling for Join.
func (svc *Intermediate) doJoin(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Join
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.JoinIn
	flow.ParseState(&in)
	var out soakflowapi.JoinOut
	out.Done, err = svc.Join(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRunSub handles marshaling for RunSub.
func (svc *Intermediate) doRunSub(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RunSub
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.RunSubIn
	flow.ParseState(&in)
	var out soakflowapi.RunSubOut
	out.Done, err = svc.RunSub(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInnerEntry handles marshaling for InnerEntry.
func (svc *Intermediate) doInnerEntry(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: InnerEntry
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in soakflowapi.InnerEntryIn
	flow.ParseState(&in)
	var out soakflowapi.InnerEntryOut
	out.Done, err = svc.InnerEntry(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doSoak handles marshaling for Soak.
func (svc *Intermediate) doSoak(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Soak
	graph, err := svc.Soak(r.Context())
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

// doInner handles marshaling for Inner.
func (svc *Intermediate) doInner(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Inner
	graph, err := svc.Inner(r.Context())
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

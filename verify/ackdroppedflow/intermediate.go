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

package ackdroppedflow

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

	"github.com/microbus-io/fabric/verify/ackdroppedflow/ackdroppedflowapi"
	"github.com/microbus-io/fabric/verify/ackdroppedflow/resources"
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
	_ ackdroppedflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = ackdroppedflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Park(ctx context.Context, flow *workflow.Flow, tag string) (parked bool, err error) // MARKER: Park
	Ping(ctx context.Context, flow *workflow.Flow, tag string) (pinged bool, err error) // MARKER: Ping
	AckDropped(ctx context.Context) (graph *workflow.Graph, err error)                  // MARKER: AckDropped
	Echo(ctx context.Context) (graph *workflow.Graph, err error)                        // MARKER: Echo
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
	svc.SetDescription(`ackdroppedflow.verify exercises the foreman's per-task 404 ack-timeout breaker by deactivating Park's subscription to provoke ack-timeouts.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: Park
		"Park", svc.doPark,
		sub.At(ackdroppedflowapi.Park.Method, ackdroppedflowapi.Park.Route),
		sub.Description(`Park returns true; subscription is deactivated by the test to provoke ack-timeouts.`),
		sub.Task(ackdroppedflowapi.ParkIn{}, ackdroppedflowapi.ParkOut{}),
	)

	svc.Subscribe( // MARKER: Ping
		"Ping", svc.doPing,
		sub.At(ackdroppedflowapi.Ping.Method, ackdroppedflowapi.Ping.Route),
		sub.Description(`Ping returns true; always reachable, the unrelated-task control.`),
		sub.Task(ackdroppedflowapi.PingIn{}, ackdroppedflowapi.PingOut{}),
	)

	svc.Subscribe( // MARKER: AckDropped
		"AckDropped", svc.doAckDropped,
		sub.At(ackdroppedflowapi.AckDropped.Method, ackdroppedflowapi.AckDropped.Route),
		sub.Description(`AckDropped defines the single-task graph (park -> END).`),
		sub.Workflow(ackdroppedflowapi.AckDroppedIn{}, ackdroppedflowapi.AckDroppedOut{}),
	)

	svc.Subscribe( // MARKER: Echo
		"Echo", svc.doEcho,
		sub.At(ackdroppedflowapi.Echo.Method, ackdroppedflowapi.Echo.Route),
		sub.Description(`Echo defines the single-task graph (ping -> END), the unrelated-task control.`),
		sub.Workflow(ackdroppedflowapi.EchoIn{}, ackdroppedflowapi.EchoOut{}),
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

// doPark handles marshaling for Park.
func (svc *Intermediate) doPark(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Park
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in ackdroppedflowapi.ParkIn
	flow.ParseState(&in)
	var out ackdroppedflowapi.ParkOut
	out.Parked, err = svc.Park(r.Context(), &flow, in.Tag)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doPing handles marshaling for Ping.
func (svc *Intermediate) doPing(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Ping
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in ackdroppedflowapi.PingIn
	flow.ParseState(&in)
	var out ackdroppedflowapi.PingOut
	out.Pinged, err = svc.Ping(r.Context(), &flow, in.Tag)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doAckDropped handles marshaling for AckDropped.
func (svc *Intermediate) doAckDropped(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AckDropped
	graph, err := svc.AckDropped(r.Context())
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

// doEcho handles marshaling for Echo.
func (svc *Intermediate) doEcho(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Echo
	graph, err := svc.Echo(r.Context())
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

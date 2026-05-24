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
	"net/http"
	"sync/atomic"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/ackdroppedflow/ackdroppedflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ ackdroppedflowapi.Client
)

/*
Service implements ackdroppedflow.verify, exercising the foreman's per-task 404 ack-timeout
breaker. Park's subscription is deactivated by the test before any flows are created, so dispatch
to it ack-times-out and trips the breaker. Ping's subscription stays on-bus, exercising the
"unrelated tasks keep dispatching" property.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	parkHits atomic.Int64
	pingHits atomic.Int64
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// ParkHits returns the number of times Park's handler actually ran (i.e. dispatches that did
// NOT ack-timeout). Always zero while Park's subscription is deactivated.
func (svc *Service) ParkHits() int64 {
	return svc.parkHits.Load()
}

// PingHits returns the number of times Ping's handler actually ran.
func (svc *Service) PingHits() int64 {
	return svc.pingHits.Load()
}

// Park returns true. Subscription is deactivated by the test to provoke ack-timeouts.
func (svc *Service) Park(ctx context.Context, flow *workflow.Flow, tag string) (parked bool, err error) { // MARKER: Park
	_ = tag
	svc.parkHits.Add(1)
	return true, nil
}

// Ping returns true. Always reachable; the unrelated-task control.
func (svc *Service) Ping(ctx context.Context, flow *workflow.Flow, tag string) (pinged bool, err error) { // MARKER: Ping
	_ = tag
	svc.pingHits.Add(1)
	return true, nil
}

// AckDropped defines the single-task graph (park -> END).
func (svc *Service) AckDropped(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: AckDropped
	graph = workflow.NewGraph(ackdroppedflowapi.AckDropped.URL())
	graph.AddTask("park", ackdroppedflowapi.Park.URL())
	graph.AddTransition("park", workflow.END)
	return graph, nil
}

// Echo defines the single-task graph (ping -> END) used as the unrelated-task control.
func (svc *Service) Echo(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Echo
	graph = workflow.NewGraph(ackdroppedflowapi.Echo.URL())
	graph.AddTask("ping", ackdroppedflowapi.Ping.URL())
	graph.AddTransition("ping", workflow.END)
	return graph, nil
}

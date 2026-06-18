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

package svc

import (
	"context"
	"net/http"

	"github.com/microbus-io/dwarf/workflow"
)

// Service is the hand-written half of the fixture; intermediate.go is generated alongside it.
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) { return nil }

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) { return nil }

// Greet returns a greeting for a name.
func (svc *Service) Greet(ctx context.Context, name string) (greeting string, err error) {
	return "", nil
}

// Ping checks liveness.
func (svc *Service) Ping(ctx context.Context) (err error) { return nil }

// Dashboard serves an HTML dashboard.
func (svc *Service) Dashboard(w http.ResponseWriter, r *http.Request) (err error) { return nil }

// ProcessStep processes an item.
func (svc *Service) ProcessStep(ctx context.Context, flow *workflow.Flow, item string) (done bool, err error) {
	return false, nil
}

// ReviewStep reviews and updates the count.
func (svc *Service) ReviewStep(ctx context.Context, flow *workflow.Flow, count int) (countOut int, err error) {
	return 0, nil
}

// MainFlow defines the top-level workflow graph.
func (svc *Service) MainFlow(ctx context.Context) (graph *workflow.Graph, err error) {
	return nil, nil
}

// OnSrcEvent handles the upstream srcapi.OnSrcEvent event.
func (svc *Service) OnSrcEvent(ctx context.Context, detail string) (ok bool, err error) {
	return false, nil
}

// OnObserveQueueDepth records the queue-depth gauge just-in-time.
func (svc *Service) OnObserveQueueDepth(ctx context.Context) (err error) {
	return svc.RecordQueueDepth(ctx, 0)
}

// OnChangedMaxItems is the callback fired when the MaxItems config changes.
func (svc *Service) OnChangedMaxItems(ctx context.Context) (err error) { return nil }

// Reconcile runs the periodic reconciliation.
func (svc *Service) Reconcile(ctx context.Context) (err error) { return nil }

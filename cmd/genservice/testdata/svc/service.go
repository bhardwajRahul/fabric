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
	"time"

	"github.com/microbus-io/dwarf/workflow"

	"github.com/microbus-io/fabric/cmd/genservice/testdata/svc/svcapi"
)

// Service is the hand-written half of the fixture; intermediate.go is generated alongside it.
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) { return nil }

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) { return nil }

/*
Greet returns a greeting for a name.
*/
func (svc *Service) Greet(ctx context.Context, name string) (greeting string, err error) {
	return "", nil
}

/*
Adopt registers a pet and returns the adoption time. It exercises qualification of a domain type
(Pet -> svcapi.Pet) and an external type (time.Time) in the generated service-package files.
*/
func (svc *Service) Adopt(ctx context.Context, pet svcapi.Pet) (since time.Time, err error) {
	return time.Time{}, nil
}

/*
Ping checks liveness; it takes and returns nothing.
*/
func (svc *Service) Ping(ctx context.Context) (err error) { return nil }

/*
Dashboard serves an HTML dashboard on any method (ANY -> 4-arg web client).
*/
func (svc *Service) Dashboard(w http.ResponseWriter, r *http.Request) (err error) { return nil }

/*
Status serves a plain status page (GET -> 2-arg web client, no body).
*/
func (svc *Service) Status(w http.ResponseWriter, r *http.Request) (err error) { return nil }

/*
Upload accepts a file upload (POST -> 3-arg web client, with body).
*/
func (svc *Service) Upload(w http.ResponseWriter, r *http.Request) (err error) { return nil }

/*
ProcessStep is a workflow task that processes an item.
*/
func (svc *Service) ProcessStep(ctx context.Context, flow *workflow.Flow, item string) (done bool, err error) {
	return false, nil
}

/*
ReviewStep is a workflow task whose output read-modify-writes the shared "count" state field.
*/
func (svc *Service) ReviewStep(ctx context.Context, flow *workflow.Flow, count int) (countOut int, err error) {
	return 0, nil
}

/*
MainFlow is the top-level workflow graph.
*/
func (svc *Service) MainFlow(ctx context.Context) (graph *workflow.Graph, err error) {
	return nil, nil
}

/*
OnSrcEvent handles the upstream srcapi.OnSrcEvent event.
*/
func (svc *Service) OnSrcEvent(ctx context.Context, detail string) (ok bool, err error) {
	return false, nil
}

/*
OnObserveQueueDepth emits the observed value of the QueueDepth metric.

QueueDepth records the current queue depth, observed just-in-time via OnObserveQueueDepth.
*/
func (svc *Service) OnObserveQueueDepth(ctx context.Context) (err error) {
	return svc.RecordQueueDepth(ctx, 0)
}

/*
OnChangedMaxItems is called when the MaxItems config property changes.

MaxItems caps the number of items processed per run; changes fire OnChangedMaxItems.
*/
func (svc *Service) OnChangedMaxItems(ctx context.Context) (err error) { return nil }

/*
Reconcile runs a periodic reconciliation.
*/
func (svc *Service) Reconcile(ctx context.Context) (err error) { return nil }

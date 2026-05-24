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

package priorityflow

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/priorityflow/priorityflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ priorityflowapi.Client
)

/*
Service implements priorityflow.verify, exercising priority-based dispatch and starvation by design.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	mu    sync.Mutex
	order []string
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Order returns the tags in the order their flows were dispatched, oldest first.
func (svc *Service) Order() []string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return append([]string(nil), svc.order...)
}

/*
Record sleeps for the requested delay, then appends its tag to the completion order. The delay forces
saturation so the order in which a single worker picks up flows reflects the foreman's selection, not
arrival timing.
*/
func (svc *Service) Record(ctx context.Context, flow *workflow.Flow) (recorded bool, err error) { // MARKER: Record
	var in priorityflowapi.RecordIn
	flow.ParseState(&in)
	if in.DelayMs > 0 {
		err = svc.Sleep(ctx, time.Duration(in.DelayMs)*time.Millisecond)
		if err != nil {
			return false, errors.Trace(err)
		}
	}
	svc.mu.Lock()
	svc.order = append(svc.order, in.Tag)
	svc.mu.Unlock()
	return true, nil
}

/*
Priority defines the single-task graph (record -> END) used to observe dispatch order under saturation.
The graph carries no timing or priority; priority is a per-flow option supplied at Create.
*/
func (svc *Service) Priority(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Priority
	graph = workflow.NewGraph(priorityflowapi.Priority.URL())
	graph.AddTask("record", priorityflowapi.Record.URL())
	graph.AddTransition("record", workflow.END)
	return graph, nil
}

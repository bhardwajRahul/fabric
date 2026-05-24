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

package continueflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/continueflow/continueflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ continueflowapi.Client
)

/*
Service implements continueflow.verify, exercising multi-turn flows via Continue.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
Increment reads counter and writes counter+1 back via the Out suffix.
*/
func (svc *Service) Increment(ctx context.Context, flow *workflow.Flow, counter int) (counterOut int, err error) { // MARKER: Increment
	return counter + 1, nil
}

/*
Counting defines a single-task workflow that increments counter and persists it across turns.
*/
func (svc *Service) Counting(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Counting
	graph = workflow.NewGraph(continueflowapi.Counting.URL())
	// Counter is declared as both input and output so it persists across Continue turns.
	graph.AddTask("increment", continueflowapi.Increment.URL())
	graph.AddTransition("increment", workflow.END)
	return graph, nil
}

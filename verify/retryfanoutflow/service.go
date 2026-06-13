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

package retryfanoutflow

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/retryfanoutflow/retryfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ retryfanoutflowapi.Client
)

/*
Service implements retryfanoutflow.verify, which exercises ordered list fan-in across a
forEach fan-out whose per-element task retries infinitely on a random 10% failure.
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
Enter is the forEach source. It echoes the input elements back to state so the
forEach transition can fan out one Increment per element.
*/
func (svc *Service) Enter(ctx context.Context, flow *workflow.Flow, elements []int) (elementsOut []int, err error) { // MARKER: Enter
	return elements, nil
}

/*
Increment runs once per element. It fails with a 10% probability and retries
immediately with no limit, so every element eventually succeeds. On success it
increments the element by one and contributes it as the results delta for
this branch; the list* reducer appends deltas in fan_out_ordinal order at fan-in.
*/
func (svc *Service) Increment(ctx context.Context, flow *workflow.Flow, element int) (resultsOut []int, err error) { // MARKER: Increment
	if rand.Float64() < 0.10 {
		flow.Retry(math.MaxInt32, 0, 0, 0)
		return nil, nil
	}
	return []int{element + 1}, nil
}

/*
Join is the fan-in target. The list* reducer has already appended every branch's
delta into results in fan_out_ordinal order; Join surfaces it as the workflow result.
*/
func (svc *Service) Join(ctx context.Context, flow *workflow.Flow, results []int) (resultsOut []int, err error) { // MARKER: Join
	return results, nil
}

/*
RetryFanOut defines the graph: Enter -> forEach(elements) -> Increment -> Join.
Join is the explicit fan-in; results collects each branch's incremented value.
*/
func (svc *Service) RetryFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: RetryFanOut
	graph = workflow.NewGraph(retryfanoutflowapi.RetryFanOut.URL())
	graph.AddTask("enter", retryfanoutflowapi.Enter.URL())
	graph.AddTask("increment", retryfanoutflowapi.Increment.URL())
	graph.AddTask("join", retryfanoutflowapi.Join.URL())
	graph.SetFanIn("join")
	graph.SetReducer("results", workflow.ReducerAppend)
	graph.AddTransitionForEach("enter", "increment", "elements", "element")
	graph.AddTransition("increment", "join")
	graph.AddTransition("join", workflow.END)
	return graph, nil
}

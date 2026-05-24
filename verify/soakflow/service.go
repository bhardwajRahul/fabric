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
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/soakflow/soakflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ soakflowapi.Client
)

/*
Service implements soakflow.verify: a high-volume liveness soak over a complex,
input-driven workflow. Tasks are intentionally trivial (no sleeps) so flows
churn fast and maximize the volume exercised; the graph's shape - not the task
work - is what stresses the dispatcher.
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

// clampNonNeg folds an arbitrary (possibly negative) random int into [0, mod).
func clampNonNeg(v, mod int) int {
	v %= mod
	if v < 0 {
		v += mod
	}
	return v
}

/*
Seed normalizes the random inputs into bounded state so every path terminates:
branch in [0,4] selects the route, fanWidth in [1,6] sizes the fan-out array,
and loops in [0,5] bounds the goto loop.
*/
func (svc *Service) Seed(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Seed
	branch := clampNonNeg(flow.GetInt("branch"), 5)
	width := clampNonNeg(flow.GetInt("fanWidth"), 6) + 1 // 1..6
	loops := clampNonNeg(flow.GetInt("loops"), 6)        // 0..5

	items := make([]int, width)
	for i := range items {
		items[i] = i
	}
	flow.Set("branch", branch)
	flow.Set("items", items)
	flow.Set("loopsLeft", loops)
	return true, nil
}

/*
FanA is the dynamic fan-out source; the forEach transition spans `items`.
*/
func (svc *Service) FanA(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: FanA
	return true, nil
}

/*
Work processes one fan-out element and contributes a delta to the sum reducer.
The `sumWork` field name auto-selects the numeric-add reducer at fan-in.
*/
func (svc *Service) Work(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Work
	flow.Set("sumWork", 1)
	return true, nil
}

/*
Collect is the fan-in target; the reducer has merged the per-element deltas.
*/
func (svc *Service) Collect(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Collect
	return true, nil
}

/*
Loop decrements loopsLeft and gotos itself until it reaches zero, then falls
through. Bounded by Seed's clamp, so it always terminates.
*/
func (svc *Service) Loop(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Loop
	n := flow.GetInt("loopsLeft")
	if n > 0 {
		flow.Set("loopsLeft", n-1)
		flow.Goto(soakflowapi.Loop.URL())
	}
	return true, nil
}

/*
BoomR always errors; the graph's onError transition routes it to Recover, so
the flow recovers and completes.
*/
func (svc *Service) BoomR(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: BoomR
	return false, errors.New("soak boom (recoverable)")
}

/*
Recover is the onError handler; it resumes the flow to completion.
*/
func (svc *Service) Recover(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Recover
	return true, nil
}

/*
BoomF always errors and the graph has no error transition for it, so the flow
fails - a terminal status, which is still "finished" for the soak.
*/
func (svc *Service) BoomF(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: BoomF
	return false, errors.New("soak boom (fatal)")
}

/*
Join is the convergence point before the workflow ends.
*/
func (svc *Service) Join(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: Join
	return true, nil
}

/*
InnerEntry is the single task of the Inner subgraph.
*/
func (svc *Service) InnerEntry(ctx context.Context, flow *workflow.Flow) (done bool, err error) { // MARKER: InnerEntry
	return true, nil
}

/*
Soak defines the complex, input-driven workflow graph. From a clamped `branch`
the entry fans into one of five mutually exclusive routes - a dynamic forEach
fan-out with a sum-reducer fan-in, a bounded goto loop, a subgraph, an
onError-recovered failure, and an unhandled failure - all converging at Join
(except the unhandled failure, which terminates the flow as failed).
*/
func (svc *Service) Soak(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Soak
	graph = workflow.NewGraph(soakflowapi.Soak.URL())
	graph.AddTask("seed", soakflowapi.Seed.URL())
	graph.AddTask("fanA", soakflowapi.FanA.URL())
	graph.AddTask("work", soakflowapi.Work.URL())
	graph.AddTask("collect", soakflowapi.Collect.URL())
	graph.AddTask("loop", soakflowapi.Loop.URL())
	graph.AddTask("boomR", soakflowapi.BoomR.URL())
	graph.AddTask("recover", soakflowapi.Recover.URL())
	graph.AddTask("boomF", soakflowapi.BoomF.URL())
	graph.AddTask("join", soakflowapi.Join.URL())
	graph.AddSubgraph("sub", soakflowapi.Inner.URL())
	graph.SetFanIn("collect") // pops the fanA forEach frame
	graph.SetFanIn("join")    // pops the seed conditional-split frame

	// Mutually exclusive 5-way conditional split (branch is clamped to 0..4).
	graph.AddTransitionWhen("seed", "fanA", "branch == 0")
	graph.AddTransitionWhen("seed", "loop", "branch == 1")
	graph.AddTransitionWhen("seed", "sub", "branch == 2")
	graph.AddTransitionWhen("seed", "boomR", "branch == 3")
	graph.AddTransitionWhen("seed", "boomF", "branch == 4")

	// Dynamic fan-out with sum-reducer fan-in.
	graph.AddTransitionForEach("fanA", "work", "items", "item")
	graph.AddTransition("work", "collect")
	graph.AddTransition("collect", "join")

	// Bounded goto loop.
	graph.AddTransitionGoto("loop", "loop")
	graph.AddTransition("loop", "join")

	// Subgraph.
	graph.AddTransition("sub", "join")

	// onError recovery.
	graph.AddTransitionOnError("boomR", "recover")
	graph.AddTransition("boomR", "join") // unused (boomR always errors); satisfies validation
	graph.AddTransition("recover", "join")

	// boomF: no error transition -> unhandled failure -> flow fails (terminal).
	graph.AddTransition("boomF", "join") // unused (boomF always errors); satisfies validation

	graph.AddTransition("join", workflow.END)
	return graph, nil
}

/*
Inner defines the subgraph invoked by Soak: a single task then END.
*/
func (svc *Service) Inner(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Inner
	graph = workflow.NewGraph(soakflowapi.Inner.URL())
	graph.AddTask("innerEntry", soakflowapi.InnerEntry.URL())
	graph.AddTransition("innerEntry", workflow.END)
	return graph, nil
}

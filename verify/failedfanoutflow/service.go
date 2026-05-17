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

package failedfanoutflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/failedfanoutflow/failedfanoutflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ failedfanoutflowapi.Client
)

/*
Service implements failedfanoutflow.verify, which exercises a fan-out branch that
hard-fails with no OnError transition, cascading the whole flow to failed.
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
Src is the static fan-out source. It returns instantly so the foreman fans out
to A, B and C.
*/
func (svc *Service) Src(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: Src
	return true, nil
}

/*
A is a normal fan-out branch contributing 1 to sumExecuted.
*/
func (svc *Service) A(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: A
	return 1, nil
}

/*
B is the failing fan-out branch. It always returns an error. With no OnError
transition, this fails B's step and cascades the whole flow to failed.
*/
func (svc *Service) B(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: B
	return 0, errors.New("triggered failure in B")
}

/*
C is a normal fan-out branch contributing 1 to sumExecuted.
*/
func (svc *Service) C(ctx context.Context, flow *workflow.Flow) (sumExecutedOut int, err error) { // MARKER: C
	return 1, nil
}

/*
J is the fan-in target. Because B fails with no OnError route, the flow fails
before fan-in, so J never runs.
*/
func (svc *Service) J(ctx context.Context, flow *workflow.Flow, sumExecuted int) (totalExecuted int, err error) { // MARKER: J
	return sumExecuted, nil
}

/*
FailedFanOut defines the graph: Src -> {A, B, C} -> J. There is no OnError
transition, so B's error fails the whole flow.
*/
func (svc *Service) FailedFanOut(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: FailedFanOut
	graph = workflow.NewGraph(failedfanoutflowapi.FailedFanOut.URL())
	graph.DeclareInputs()
	graph.DeclareOutputs("sumExecuted", "totalExecuted")
	graph.AddTask("src", failedfanoutflowapi.Src.URL())
	graph.AddTask("a", failedfanoutflowapi.A.URL())
	graph.AddTask("b", failedfanoutflowapi.B.URL())
	graph.AddTask("c", failedfanoutflowapi.C.URL())
	graph.AddTask("j", failedfanoutflowapi.J.URL())
	graph.SetFanIn("j")
	graph.AddTransition("src", "a")
	graph.AddTransition("src", "b")
	graph.AddTransition("src", "c")
	graph.AddTransition("a", "j")
	graph.AddTransition("b", "j")
	graph.AddTransition("c", "j")
	graph.AddTransition("j", workflow.END)
	return graph, nil
}

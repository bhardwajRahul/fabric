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

package gotoflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/gotoflow/gotoflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ gotoflowapi.Client
)

/*
Service implements gotoflow.verify, exercising flow.Goto and AddTransitionGoto.
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
TaskA increments the loops counter.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, loops int) (loopsOut int, err error) { // MARKER: TaskA
	return loops + 1, nil
}

/*
TaskB calls flow.Goto(TaskA) until the loop counter reaches target, then falls through.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, loops int, target int) (visited bool, err error) { // MARKER: TaskB
	if loops < target {
		flow.Goto(gotoflowapi.TaskA.URL())
	}
	return true, nil
}

/*
TaskC surfaces the final loops count.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow, loops int) (finalLoops int, err error) { // MARKER: TaskC
	return loops, nil
}

/*
Goto defines the graph A -> B -> C with B -> withGoto -> A.
*/
func (svc *Service) Goto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Goto
	graph = workflow.NewGraph(gotoflowapi.Goto.URL())
	graph.AddTask("taskA", gotoflowapi.TaskA.URL())
	graph.AddTask("taskB", gotoflowapi.TaskB.URL())
	graph.AddTask("taskC", gotoflowapi.TaskC.URL())
	graph.AddTransition("taskA", "taskB")
	graph.AddTransitionGoto("taskB", "taskA")
	graph.AddTransition("taskB", "taskC")
	graph.AddTransition("taskC", workflow.END)
	return graph, nil
}

/*
BadGotoer requests a goto to a target that has no AddTransitionGoto registered for it.
The foreman surfaces the unregistered target as a step failure.
*/
func (svc *Service) BadGotoer(ctx context.Context, flow *workflow.Flow) (stamp bool, err error) { // MARKER: BadGotoer
	flow.Goto("https://gotoflow.verify:428/no-such-task")
	return true, nil
}

/*
BadGoto defines a single-task graph whose only task issues an unregistered Goto.
The flow must end with status=failed.
*/
func (svc *Service) BadGoto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: BadGoto
	graph = workflow.NewGraph(gotoflowapi.BadGoto.URL())
	graph.AddTask("badGotoer", gotoflowapi.BadGotoer.URL())
	graph.AddTransition("badGotoer", workflow.END)
	return graph, nil
}

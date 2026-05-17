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

package intrathreadgotoflow

import (
	"context"
	"fmt"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/intrathreadgotoflow/intrathreadgotoflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ intrathreadgotoflowapi.Client
)

/*
Service implements intrathreadgotoflow.verify, the SKIP-marked intra-thread Goto pattern.
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

// TaskA passes target through.
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error) { // MARKER: TaskA
	return target, nil
}

// LoopTask increments loops; if loops<target, calls flow.Goto(LoopTask) to loop back.
func (svc *Service) LoopTask(ctx context.Context, flow *workflow.Flow, loops, target int) (loopsOut int, err error) { // MARKER: LoopTask
	loops++
	if loops < target {
		flow.Goto(intrathreadgotoflowapi.LoopTask.URL())
	}
	return loops, nil
}

// NormalC produces a stamp.
func (svc *Service) NormalC(ctx context.Context, flow *workflow.Flow) (stamp string, err error) { // MARKER: NormalC
	return "stamped", nil
}

// TaskD is the outer fan-in: combines loops with stamp.
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow, loops int, stamp string) (finalResult string, err error) { // MARKER: TaskD
	return fmt.Sprintf("%s/%d", stamp, loops), nil
}

/*
IntraThreadGoto defines A -> {LoopTask (self-Goto), NormalC} -> D.
*/
func (svc *Service) IntraThreadGoto(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: IntraThreadGoto
	graph = workflow.NewGraph(intrathreadgotoflowapi.IntraThreadGoto.URL())
	graph.DeclareInputs("target")
	graph.DeclareOutputs("finalResult")
	graph.AddTask("taskA", intrathreadgotoflowapi.TaskA.URL())
	graph.AddTask("loopTask", intrathreadgotoflowapi.LoopTask.URL())
	graph.AddTask("normalC", intrathreadgotoflowapi.NormalC.URL())
	graph.AddTask("taskD", intrathreadgotoflowapi.TaskD.URL())
	graph.SetFanIn("taskD")
	graph.AddTransition("taskA", "loopTask")
	graph.AddTransition("taskA", "normalC")
	graph.AddTransitionGoto("loopTask", "loopTask")
	graph.AddTransition("loopTask", "taskD")
	graph.AddTransition("normalC", "taskD")
	graph.AddTransition("taskD", workflow.END)
	return graph, nil
}

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

package perelementpipelineflow

import (
	"context"
	"net/http"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/perelementpipelineflow/perelementpipelineflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ perelementpipelineflowapi.Client
)

/*
Service implements perelementpipelineflow.verify, the SKIP-marked per-element pipeline pattern.
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
TaskS passes items through to the forEach source.
*/
func (svc *Service) TaskS(ctx context.Context, flow *workflow.Flow, items []string) (itemsOut []string, err error) { // MARKER: TaskS
	return items, nil
}

/*
TaskH uppercases the per-element item. Runs once per forEach instance.
*/
func (svc *Service) TaskH(ctx context.Context, flow *workflow.Flow, item string) (itemUpper string, err error) { // MARKER: TaskH
	return strings.ToUpper(item), nil
}

/*
TaskA is one parallel branch in the per-element inner pipeline.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow, itemUpper string) (aProcessed string, err error) { // MARKER: TaskA
	return "a:" + itemUpper, nil
}

/*
TaskB is the other parallel branch in the per-element inner pipeline.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow, itemUpper string) (bProcessed string, err error) { // MARKER: TaskB
	return "b:" + itemUpper, nil
}

/*
TaskM is the per-element fan-in. Emits one merged entry into the outer set* reducer field.
*/
func (svc *Service) TaskM(ctx context.Context, flow *workflow.Flow, aProcessed, bProcessed string) (setMerged []string, err error) { // MARKER: TaskM
	return []string{aProcessed + "+" + bProcessed}, nil
}

/*
TaskL is the outer fan-in. Counts the merged entries.
*/
func (svc *Service) TaskL(ctx context.Context, flow *workflow.Flow, setMerged []string) (finalCount int, err error) { // MARKER: TaskL
	return len(setMerged), nil
}

/*
PerElementPipeline defines the graph S -> forEach(items) -> H -> {A, B} -> M -> L.
*/
func (svc *Service) PerElementPipeline(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: PerElementPipeline
	graph = workflow.NewGraph(perelementpipelineflowapi.PerElementPipeline.URL())
	graph.AddTask("taskS", perelementpipelineflowapi.TaskS.URL())
	graph.AddTask("taskH", perelementpipelineflowapi.TaskH.URL())
	graph.AddTask("taskA", perelementpipelineflowapi.TaskA.URL())
	graph.AddTask("taskB", perelementpipelineflowapi.TaskB.URL())
	graph.AddTask("taskM", perelementpipelineflowapi.TaskM.URL())
	graph.AddTask("taskL", perelementpipelineflowapi.TaskL.URL())
	graph.SetFanIn("taskM") // inner per-element {A, B} converge here
	graph.SetFanIn("taskL") // outer fan-in across forEach elements
	// forEach from S to H, one H per element.
	graph.AddTransitionForEach("taskS", "taskH", "items", "item")
	// Inner fan-out per element from H.
	graph.AddTransition("taskH", "taskA")
	graph.AddTransition("taskH", "taskB")
	// Inner fan-in per element at M.
	graph.AddTransition("taskA", "taskM")
	graph.AddTransition("taskB", "taskM")
	// Outer fan-in across all elements at L.
	graph.AddTransition("taskM", "taskL")
	graph.AddTransition("taskL", workflow.END)
	return graph, nil
}

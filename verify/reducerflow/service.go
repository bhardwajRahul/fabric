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

package reducerflow

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/reducerflow/reducerflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ reducerflowapi.Client
)

/*
Service implements reducerflow.verify, exercising sum/list/set reducers at fan-in.
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
TaskA is the fan-out source.
*/
func (svc *Service) TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error) { // MARKER: TaskA
	return true, nil
}

/*
TaskB contributes deltas: +10 to sumTotal, ["b"] to listTags, ["x"] to setSeen.
*/
func (svc *Service) TaskB(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error) { // MARKER: TaskB
	return 10, []string{"b"}, []string{"x"}, nil
}

/*
TaskC contributes deltas: +20 to sumTotal, ["c"] to listTags, ["y","x"] to setSeen.
The "x" overlaps with TaskB's contribution; setSeen's union reducer dedupes it.
*/
func (svc *Service) TaskC(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error) { // MARKER: TaskC
	return 20, []string{"c"}, []string{"y", "x"}, nil
}

/*
TaskD contributes deltas: +30 to sumTotal, ["d"] to listTags, ["z"] to setSeen.
*/
func (svc *Service) TaskD(ctx context.Context, flow *workflow.Flow) (sumTotalOut int, listTagsOut, setSeenOut []string, err error) { // MARKER: TaskD
	return 30, []string{"d"}, []string{"z"}, nil
}

/*
TaskE reads the reducer-merged values and surfaces them.
*/
func (svc *Service) TaskE(ctx context.Context, flow *workflow.Flow, sumTotal int, listTags, setSeen []string) (finalSum int, finalList, finalSet []string, err error) { // MARKER: TaskE
	return sumTotal, listTags, setSeen, nil
}

/*
Reducer defines the graph A -> {B, C, D} -> E with reducer-managed fan-in fields.
*/
func (svc *Service) Reducer(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: Reducer
	graph = workflow.NewGraph(reducerflowapi.Reducer.URL())
	graph.AddTask("taskA", reducerflowapi.TaskA.URL())
	graph.AddTask("taskB", reducerflowapi.TaskB.URL())
	graph.AddTask("taskC", reducerflowapi.TaskC.URL())
	graph.AddTask("taskD", reducerflowapi.TaskD.URL())
	graph.AddTask("taskE", reducerflowapi.TaskE.URL())
	graph.SetFanIn("taskE")
	graph.AddTransition("taskA", "taskB")
	graph.AddTransition("taskA", "taskC")
	graph.AddTransition("taskA", "taskD")
	graph.AddTransition("taskB", "taskE")
	graph.AddTransition("taskC", "taskE")
	graph.AddTransition("taskD", "taskE")
	graph.AddTransition("taskE", workflow.END)
	return graph, nil
}

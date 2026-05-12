package subgraphflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/verify/subgraphflow/resources"
	"github.com/microbus-io/fabric/verify/subgraphflow/subgraphflowapi"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ subgraphflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = subgraphflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, seed string) (seedOut string, err error)              // MARKER: TaskA
	TaskX(ctx context.Context, flow *workflow.Flow, seed string) (innerStage string, err error)          // MARKER: TaskX
	TaskY(ctx context.Context, flow *workflow.Flow, innerStage string) (innerResult string, err error)   // MARKER: TaskY
	TaskZ(ctx context.Context, flow *workflow.Flow, innerResult string) (finalResult string, err error)  // MARKER: TaskZ
	Inner(ctx context.Context) (graph *workflow.Graph, err error)                                         // MARKER: Inner
	Parent(ctx context.Context) (graph *workflow.Graph, err error)                                        // MARKER: Parent
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`subgraphflow.verify exercises subgraph invocation, output merging, and the parent-child state boundary.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(subgraphflowapi.TaskA.Method, subgraphflowapi.TaskA.Route),
		sub.Description(`TaskA passes the seed through.`),
		sub.Task(subgraphflowapi.TaskAIn{}, subgraphflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskX
		"TaskX", svc.doTaskX,
		sub.At(subgraphflowapi.TaskX.Method, subgraphflowapi.TaskX.Route),
		sub.Description(`TaskX is the subgraph entry.`),
		sub.Task(subgraphflowapi.TaskXIn{}, subgraphflowapi.TaskXOut{}),
	)
	svc.Subscribe( // MARKER: TaskY
		"TaskY", svc.doTaskY,
		sub.At(subgraphflowapi.TaskY.Method, subgraphflowapi.TaskY.Route),
		sub.Description(`TaskY runs after TaskX in the subgraph.`),
		sub.Task(subgraphflowapi.TaskYIn{}, subgraphflowapi.TaskYOut{}),
	)
	svc.Subscribe( // MARKER: TaskZ
		"TaskZ", svc.doTaskZ,
		sub.At(subgraphflowapi.TaskZ.Method, subgraphflowapi.TaskZ.Route),
		sub.Description(`TaskZ runs in the parent after the subgraph.`),
		sub.Task(subgraphflowapi.TaskZIn{}, subgraphflowapi.TaskZOut{}),
	)

	svc.Subscribe( // MARKER: Inner
		"Inner", svc.doInner,
		sub.At(subgraphflowapi.Inner.Method, subgraphflowapi.Inner.Route),
		sub.Description(`Inner defines the subgraph X -> Y.`),
		sub.Workflow(subgraphflowapi.InnerIn{}, subgraphflowapi.InnerOut{}),
	)
	svc.Subscribe( // MARKER: Parent
		"Parent", svc.doParent,
		sub.At(subgraphflowapi.Parent.Method, subgraphflowapi.Parent.Route),
		sub.Description(`Parent defines the graph A -> [Inner subgraph] -> Z.`),
		sub.Workflow(subgraphflowapi.ParentIn{}, subgraphflowapi.ParentOut{}),
	)

	_ = marshalFunction
	return svc
}

func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphflowapi.TaskAIn
	flow.ParseState(&in)
	var out subgraphflowapi.TaskAOut
	out.SeedOut, err = svc.TaskA(r.Context(), &flow, in.Seed)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskX handles marshaling for TaskX.
func (svc *Intermediate) doTaskX(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskX
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphflowapi.TaskXIn
	flow.ParseState(&in)
	var out subgraphflowapi.TaskXOut
	out.InnerStage, err = svc.TaskX(r.Context(), &flow, in.Seed)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskY handles marshaling for TaskY.
func (svc *Intermediate) doTaskY(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskY
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphflowapi.TaskYIn
	flow.ParseState(&in)
	var out subgraphflowapi.TaskYOut
	out.InnerResult, err = svc.TaskY(r.Context(), &flow, in.InnerStage)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskZ handles marshaling for TaskZ.
func (svc *Intermediate) doTaskZ(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskZ
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in subgraphflowapi.TaskZIn
	flow.ParseState(&in)
	var out subgraphflowapi.TaskZOut
	out.FinalResult, err = svc.TaskZ(r.Context(), &flow, in.InnerResult)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInner handles marshaling for Inner.
func (svc *Intermediate) doInner(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Inner
	graph, err := svc.Inner(r.Context())
	if err != nil {
		return err
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

// doParent handles marshaling for Parent.
func (svc *Intermediate) doParent(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Parent
	graph, err := svc.Parent(r.Context())
	if err != nil {
		return err
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

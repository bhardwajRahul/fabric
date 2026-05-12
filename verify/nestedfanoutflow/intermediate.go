package nestedfanoutflow

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

	"github.com/microbus-io/fabric/verify/nestedfanoutflow/nestedfanoutflowapi"
	"github.com/microbus-io/fabric/verify/nestedfanoutflow/resources"
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
	_ nestedfanoutflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = nestedfanoutflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error)                                                       // MARKER: TaskA
	NormalB(ctx context.Context, flow *workflow.Flow) (normalResult string, err error)                                              // MARKER: NormalB
	TaskX(ctx context.Context, flow *workflow.Flow) (innerStarted bool, err error)                                                  // MARKER: TaskX
	TaskY(ctx context.Context, flow *workflow.Flow) (sumInnerOut int, err error)                                                    // MARKER: TaskY
	TaskZ(ctx context.Context, flow *workflow.Flow) (sumInnerOut int, err error)                                                    // MARKER: TaskZ
	TaskW(ctx context.Context, flow *workflow.Flow, sumInner int) (innerResult int, err error)                                      // MARKER: TaskW
	TaskJ(ctx context.Context, flow *workflow.Flow, normalResult string, innerResult int) (finalResult string, err error)           // MARKER: TaskJ
	Inner(ctx context.Context) (graph *workflow.Graph, err error)                                                                    // MARKER: Inner
	Nested(ctx context.Context) (graph *workflow.Graph, err error)                                                                   // MARKER: Nested
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
	svc.SetDescription(`nestedfanoutflow.verify exercises nested fan-out via the subgraph escape hatch.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(nestedfanoutflowapi.TaskA.Method, nestedfanoutflowapi.TaskA.Route),
		sub.Description(`TaskA is the outer fan-out source.`),
		sub.Task(nestedfanoutflowapi.TaskAIn{}, nestedfanoutflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: NormalB
		"NormalB", svc.doNormalB,
		sub.At(nestedfanoutflowapi.NormalB.Method, nestedfanoutflowapi.NormalB.Route),
		sub.Description(`NormalB is the non-subgraph sibling.`),
		sub.Task(nestedfanoutflowapi.NormalBIn{}, nestedfanoutflowapi.NormalBOut{}),
	)
	svc.Subscribe( // MARKER: TaskX
		"TaskX", svc.doTaskX,
		sub.At(nestedfanoutflowapi.TaskX.Method, nestedfanoutflowapi.TaskX.Route),
		sub.Description(`TaskX is the inner subgraph entry.`),
		sub.Task(nestedfanoutflowapi.TaskXIn{}, nestedfanoutflowapi.TaskXOut{}),
	)
	svc.Subscribe( // MARKER: TaskY
		"TaskY", svc.doTaskY,
		sub.At(nestedfanoutflowapi.TaskY.Method, nestedfanoutflowapi.TaskY.Route),
		sub.Description(`TaskY contributes a delta to the inner sum reducer.`),
		sub.Task(nestedfanoutflowapi.TaskYIn{}, nestedfanoutflowapi.TaskYOut{}),
	)
	svc.Subscribe( // MARKER: TaskZ
		"TaskZ", svc.doTaskZ,
		sub.At(nestedfanoutflowapi.TaskZ.Method, nestedfanoutflowapi.TaskZ.Route),
		sub.Description(`TaskZ contributes a delta to the inner sum reducer.`),
		sub.Task(nestedfanoutflowapi.TaskZIn{}, nestedfanoutflowapi.TaskZOut{}),
	)
	svc.Subscribe( // MARKER: TaskW
		"TaskW", svc.doTaskW,
		sub.At(nestedfanoutflowapi.TaskW.Method, nestedfanoutflowapi.TaskW.Route),
		sub.Description(`TaskW is the inner subgraph fan-in.`),
		sub.Task(nestedfanoutflowapi.TaskWIn{}, nestedfanoutflowapi.TaskWOut{}),
	)
	svc.Subscribe( // MARKER: TaskJ
		"TaskJ", svc.doTaskJ,
		sub.At(nestedfanoutflowapi.TaskJ.Method, nestedfanoutflowapi.TaskJ.Route),
		sub.Description(`TaskJ is the outer fan-in.`),
		sub.Task(nestedfanoutflowapi.TaskJIn{}, nestedfanoutflowapi.TaskJOut{}),
	)

	svc.Subscribe( // MARKER: Inner
		"Inner", svc.doInner,
		sub.At(nestedfanoutflowapi.Inner.Method, nestedfanoutflowapi.Inner.Route),
		sub.Description(`Inner defines the inner subgraph X -> {Y, Z} -> W.`),
		sub.Workflow(nestedfanoutflowapi.InnerIn{}, nestedfanoutflowapi.InnerOut{}),
	)
	svc.Subscribe( // MARKER: Nested
		"Nested", svc.doNested,
		sub.At(nestedfanoutflowapi.Nested.Method, nestedfanoutflowapi.Nested.Route),
		sub.Description(`Nested defines the outer graph A -> {NormalB, Inner subgraph} -> J.`),
		sub.Workflow(nestedfanoutflowapi.NestedIn{}, nestedfanoutflowapi.NestedOut{}),
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskAIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskAOut
	out.Started, err = svc.TaskA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doNormalB handles marshaling for NormalB.
func (svc *Intermediate) doNormalB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NormalB
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.NormalBIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.NormalBOut
	out.NormalResult, err = svc.NormalB(r.Context(), &flow)
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskXIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskXOut
	out.InnerStarted, err = svc.TaskX(r.Context(), &flow)
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskYIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskYOut
	out.SumInnerOut, err = svc.TaskY(r.Context(), &flow)
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskZIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskZOut
	out.SumInnerOut, err = svc.TaskZ(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskW handles marshaling for TaskW.
func (svc *Intermediate) doTaskW(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskW
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskWIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskWOut
	out.InnerResult, err = svc.TaskW(r.Context(), &flow, in.SumInner)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskJ handles marshaling for TaskJ.
func (svc *Intermediate) doTaskJ(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskJ
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in nestedfanoutflowapi.TaskJIn
	flow.ParseState(&in)
	var out nestedfanoutflowapi.TaskJOut
	out.FinalResult, err = svc.TaskJ(r.Context(), &flow, in.NormalResult, in.InnerResult)
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
	if err = graph.Validate(); err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

// doNested handles marshaling for Nested.
func (svc *Intermediate) doNested(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Nested
	graph, err := svc.Nested(r.Context())
	if err != nil {
		return err
	}
	if err = graph.Validate(); err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph}))
}

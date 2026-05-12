package conditionalflow

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

	"github.com/microbus-io/fabric/verify/conditionalflow/conditionalflowapi"
	"github.com/microbus-io/fabric/verify/conditionalflow/resources"
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
	_ conditionalflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = conditionalflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, score int) (scoreOut int, err error)         // MARKER: TaskA
	TaskHigh(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)    // MARKER: TaskHigh
	TaskLow(ctx context.Context, flow *workflow.Flow, score int) (branch string, err error)     // MARKER: TaskLow
	TaskC(ctx context.Context, flow *workflow.Flow, branch string) (finalBranch string, err error) // MARKER: TaskC
	Conditional(ctx context.Context) (graph *workflow.Graph, err error)                          // MARKER: Conditional
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
	svc.SetDescription(`conditionalflow.verify exercises AddTransitionWhen for boolean-expression branching.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(conditionalflowapi.TaskA.Method, conditionalflowapi.TaskA.Route),
		sub.Description(`TaskA passes the score through to state.`),
		sub.Task(conditionalflowapi.TaskAIn{}, conditionalflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskHigh
		"TaskHigh", svc.doTaskHigh,
		sub.At(conditionalflowapi.TaskHigh.Method, conditionalflowapi.TaskHigh.Route),
		sub.Description(`TaskHigh runs when score>=50.`),
		sub.Task(conditionalflowapi.TaskHighIn{}, conditionalflowapi.TaskHighOut{}),
	)
	svc.Subscribe( // MARKER: TaskLow
		"TaskLow", svc.doTaskLow,
		sub.At(conditionalflowapi.TaskLow.Method, conditionalflowapi.TaskLow.Route),
		sub.Description(`TaskLow runs when score<50.`),
		sub.Task(conditionalflowapi.TaskLowIn{}, conditionalflowapi.TaskLowOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(conditionalflowapi.TaskC.Method, conditionalflowapi.TaskC.Route),
		sub.Description(`TaskC surfaces the branch that ran.`),
		sub.Task(conditionalflowapi.TaskCIn{}, conditionalflowapi.TaskCOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: Conditional
		"Conditional", svc.doConditional,
		sub.At(conditionalflowapi.Conditional.Method, conditionalflowapi.Conditional.Route),
		sub.Description(`Conditional defines the graph with when transitions.`),
		sub.Workflow(conditionalflowapi.ConditionalIn{}, conditionalflowapi.ConditionalOut{}),
	)

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
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
	var in conditionalflowapi.TaskAIn
	flow.ParseState(&in)
	var out conditionalflowapi.TaskAOut
	out.ScoreOut, err = svc.TaskA(r.Context(), &flow, in.Score)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskHigh handles marshaling for TaskHigh.
func (svc *Intermediate) doTaskHigh(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskHigh
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in conditionalflowapi.TaskHighIn
	flow.ParseState(&in)
	var out conditionalflowapi.TaskHighOut
	out.Branch, err = svc.TaskHigh(r.Context(), &flow, in.Score)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskLow handles marshaling for TaskLow.
func (svc *Intermediate) doTaskLow(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskLow
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in conditionalflowapi.TaskLowIn
	flow.ParseState(&in)
	var out conditionalflowapi.TaskLowOut
	out.Branch, err = svc.TaskLow(r.Context(), &flow, in.Score)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskC handles marshaling for TaskC.
func (svc *Intermediate) doTaskC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskC
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in conditionalflowapi.TaskCIn
	flow.ParseState(&in)
	var out conditionalflowapi.TaskCOut
	out.FinalBranch, err = svc.TaskC(r.Context(), &flow, in.Branch)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doConditional handles marshaling for Conditional.
func (svc *Intermediate) doConditional(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Conditional
	graph, err := svc.Conditional(r.Context())
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

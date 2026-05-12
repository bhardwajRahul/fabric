package timebudgetflow

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

	"github.com/microbus-io/fabric/verify/timebudgetflow/resources"
	"github.com/microbus-io/fabric/verify/timebudgetflow/timebudgetflowapi"
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
	_ timebudgetflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = timebudgetflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow) (started bool, err error)         // MARKER: TaskA
	Slow(ctx context.Context, flow *workflow.Flow) (done bool, err error)             // MARKER: Slow
	TimeBudget(ctx context.Context) (graph *workflow.Graph, err error)                // MARKER: TimeBudget
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
	svc.SetDescription(`timebudgetflow.verify exercises per-task time budget timeouts.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(timebudgetflowapi.TaskA.Method, timebudgetflowapi.TaskA.Route),
		sub.Description(`TaskA is the entry point.`),
		sub.Task(timebudgetflowapi.TaskAIn{}, timebudgetflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: Slow
		"Slow", svc.doSlow,
		sub.At(timebudgetflowapi.Slow.Method, timebudgetflowapi.Slow.Route),
		sub.Description(`Slow sleeps longer than the time budget.`),
		sub.Task(timebudgetflowapi.SlowIn{}, timebudgetflowapi.SlowOut{}),
	)

	svc.Subscribe( // MARKER: TimeBudget
		"TimeBudget", svc.doTimeBudget,
		sub.At(timebudgetflowapi.TimeBudget.Method, timebudgetflowapi.TimeBudget.Route),
		sub.Description(`TimeBudget defines A -> Slow with a 50ms budget on Slow.`),
		sub.Workflow(timebudgetflowapi.TimeBudgetIn{}, timebudgetflowapi.TimeBudgetOut{}),
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
	var in timebudgetflowapi.TaskAIn
	flow.ParseState(&in)
	var out timebudgetflowapi.TaskAOut
	out.Started, err = svc.TaskA(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doSlow handles marshaling for Slow.
func (svc *Intermediate) doSlow(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Slow
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in timebudgetflowapi.SlowIn
	flow.ParseState(&in)
	var out timebudgetflowapi.SlowOut
	out.Done, err = svc.Slow(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTimeBudget handles marshaling for TimeBudget.
func (svc *Intermediate) doTimeBudget(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TimeBudget
	graph, err := svc.TimeBudget(r.Context())
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

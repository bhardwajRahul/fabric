package sleepflow

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

	"github.com/microbus-io/fabric/verify/sleepflow/resources"
	"github.com/microbus-io/fabric/verify/sleepflow/sleepflowapi"
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
	_ sleepflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = sleepflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (sleepForOut time.Duration, err error) // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, sleepFor time.Duration) (marked bool, err error)                // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, marked bool) (completed bool, err error)                        // MARKER: TaskC
	Delay(ctx context.Context) (graph *workflow.Graph, err error)                                                    // MARKER: Delay
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
	svc.SetDescription(`sleepflow.verify exercises flow.Sleep.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(sleepflowapi.TaskA.Method, sleepflowapi.TaskA.Route),
		sub.Description(`TaskA passes the sleep duration through.`),
		sub.Task(sleepflowapi.TaskAIn{}, sleepflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(sleepflowapi.TaskB.Method, sleepflowapi.TaskB.Route),
		sub.Description(`TaskB calls flow.Sleep.`),
		sub.Task(sleepflowapi.TaskBIn{}, sleepflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(sleepflowapi.TaskC.Method, sleepflowapi.TaskC.Route),
		sub.Description(`TaskC runs after the sleep elapses.`),
		sub.Task(sleepflowapi.TaskCIn{}, sleepflowapi.TaskCOut{}),
	)

	svc.Subscribe( // MARKER: Delay
		"Delay", svc.doDelay,
		sub.At(sleepflowapi.Delay.Method, sleepflowapi.Delay.Route),
		sub.Description(`Delay defines A -> B -> C with B calling flow.Sleep.`),
		sub.Workflow(sleepflowapi.DelayIn{}, sleepflowapi.DelayOut{}),
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
	var in sleepflowapi.TaskAIn
	flow.ParseState(&in)
	var out sleepflowapi.TaskAOut
	out.SleepForOut, err = svc.TaskA(r.Context(), &flow, in.SleepFor)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskB handles marshaling for TaskB.
func (svc *Intermediate) doTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskB
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in sleepflowapi.TaskBIn
	flow.ParseState(&in)
	var out sleepflowapi.TaskBOut
	out.Marked, err = svc.TaskB(r.Context(), &flow, in.SleepFor)
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
	var in sleepflowapi.TaskCIn
	flow.ParseState(&in)
	var out sleepflowapi.TaskCOut
	out.Completed, err = svc.TaskC(r.Context(), &flow, in.Marked)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doDelay handles marshaling for Delay.
func (svc *Intermediate) doDelay(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Delay
	graph, err := svc.Delay(r.Context())
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

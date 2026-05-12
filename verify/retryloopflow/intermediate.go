package retryloopflow

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

	"github.com/microbus-io/fabric/verify/retryloopflow/resources"
	"github.com/microbus-io/fabric/verify/retryloopflow/retryloopflowapi"
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
	_ retryloopflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = retryloopflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error)                            // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, attempts, target int) (succeeded bool, err error)                  // MARKER: TaskB
	Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError, attempts int) (attemptsOut int, err error) // MARKER: Handler
	TaskC(ctx context.Context, flow *workflow.Flow, attempts int) (finalAttempts int, err error)                       // MARKER: TaskC
	RetryLoop(ctx context.Context) (graph *workflow.Graph, err error)                                                  // MARKER: RetryLoop
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
	svc.SetDescription(`retryloopflow.verify is the SKIP-marked OnError retry loop (pending lineage redesign).`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(retryloopflowapi.TaskA.Method, retryloopflowapi.TaskA.Route),
		sub.Description(`TaskA passes target through.`),
		sub.Task(retryloopflowapi.TaskAIn{}, retryloopflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(retryloopflowapi.TaskB.Method, retryloopflowapi.TaskB.Route),
		sub.Description(`TaskB errors if attempts<target.`),
		sub.Task(retryloopflowapi.TaskBIn{}, retryloopflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: Handler
		"Handler", svc.doHandler,
		sub.At(retryloopflowapi.Handler.Method, retryloopflowapi.Handler.Route),
		sub.Description(`Handler increments attempts and routes back to B.`),
		sub.Task(retryloopflowapi.HandlerIn{}, retryloopflowapi.HandlerOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(retryloopflowapi.TaskC.Method, retryloopflowapi.TaskC.Route),
		sub.Description(`TaskC surfaces final attempts.`),
		sub.Task(retryloopflowapi.TaskCIn{}, retryloopflowapi.TaskCOut{}),
	)

	svc.Subscribe( // MARKER: RetryLoop
		"RetryLoop", svc.doRetryLoop,
		sub.At(retryloopflowapi.RetryLoop.Method, retryloopflowapi.RetryLoop.Route),
		sub.Description(`RetryLoop defines A -> B -> C with B onError -> Handler -> B.`),
		sub.Workflow(retryloopflowapi.RetryLoopIn{}, retryloopflowapi.RetryLoopOut{}),
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
	var in retryloopflowapi.TaskAIn
	flow.ParseState(&in)
	var out retryloopflowapi.TaskAOut
	out.TargetOut, err = svc.TaskA(r.Context(), &flow, in.Target)
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryloopflowapi.TaskBIn
	flow.ParseState(&in)
	var out retryloopflowapi.TaskBOut
	out.Succeeded, err = svc.TaskB(r.Context(), &flow, in.Attempts, in.Target)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doHandler handles marshaling for Handler.
func (svc *Intermediate) doHandler(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Handler
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryloopflowapi.HandlerIn
	flow.ParseState(&in)
	var out retryloopflowapi.HandlerOut
	out.AttemptsOut, err = svc.Handler(r.Context(), &flow, in.OnErr, in.Attempts)
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
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in retryloopflowapi.TaskCIn
	flow.ParseState(&in)
	var out retryloopflowapi.TaskCOut
	out.FinalAttempts, err = svc.TaskC(r.Context(), &flow, in.Attempts)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doRetryLoop handles marshaling for RetryLoop.
func (svc *Intermediate) doRetryLoop(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RetryLoop
	graph, err := svc.RetryLoop(r.Context())
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

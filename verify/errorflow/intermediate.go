package errorflow

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

	"github.com/microbus-io/fabric/verify/errorflow/errorflowapi"
	"github.com/microbus-io/fabric/verify/errorflow/resources"
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
	_ errorflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = errorflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, trigger string) (triggerOut string, err error)           // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, trigger string) (result string, err error)               // MARKER: TaskB
	Handler(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (result string, err error)  // MARKER: Handler
	TaskC(ctx context.Context, flow *workflow.Flow, result string) (finalResult string, err error)           // MARKER: TaskC
	Error(ctx context.Context) (graph *workflow.Graph, err error)                                            // MARKER: Error
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
	svc.SetDescription(`errorflow.verify exercises onError routing in a sequential flow.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(errorflowapi.TaskA.Method, errorflowapi.TaskA.Route),
		sub.Description(`TaskA passes the trigger through to state.`),
		sub.Task(errorflowapi.TaskAIn{}, errorflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(errorflowapi.TaskB.Method, errorflowapi.TaskB.Route),
		sub.Description(`TaskB returns "ok" normally, or an error when the trigger is "fail".`),
		sub.Task(errorflowapi.TaskBIn{}, errorflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: Handler
		"Handler", svc.doHandler,
		sub.At(errorflowapi.Handler.Method, errorflowapi.Handler.Route),
		sub.Description(`Handler runs when TaskB errors.`),
		sub.Task(errorflowapi.HandlerIn{}, errorflowapi.HandlerOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(errorflowapi.TaskC.Method, errorflowapi.TaskC.Route),
		sub.Description(`TaskC consumes whichever result reached it.`),
		sub.Task(errorflowapi.TaskCIn{}, errorflowapi.TaskCOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: Error
		"Error", svc.doError,
		sub.At(errorflowapi.Error.Method, errorflowapi.Error.Route),
		sub.Description(`Error defines the graph A -> B -> C with B onError -> Handler -> C.`),
		sub.Workflow(errorflowapi.ErrorIn{}, errorflowapi.ErrorOut{}),
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
	var in errorflowapi.TaskAIn
	flow.ParseState(&in)
	var out errorflowapi.TaskAOut
	out.TriggerOut, err = svc.TaskA(r.Context(), &flow, in.Trigger)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doTaskB handles marshaling for TaskB.
func (svc *Intermediate) doTaskB(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskB
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in errorflowapi.TaskBIn
	flow.ParseState(&in)
	var out errorflowapi.TaskBOut
	out.Result, err = svc.TaskB(r.Context(), &flow, in.Trigger)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doHandler handles marshaling for Handler.
func (svc *Intermediate) doHandler(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Handler
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in errorflowapi.HandlerIn
	flow.ParseState(&in)
	var out errorflowapi.HandlerOut
	out.Result, err = svc.Handler(r.Context(), &flow, in.OnErr)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doTaskC handles marshaling for TaskC.
func (svc *Intermediate) doTaskC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskC
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in errorflowapi.TaskCIn
	flow.ParseState(&in)
	var out errorflowapi.TaskCOut
	out.FinalResult, err = svc.TaskC(r.Context(), &flow, in.Result)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doError handles marshaling for Error.
func (svc *Intermediate) doError(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Error
	graph, err := svc.Error(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

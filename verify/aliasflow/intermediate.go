package aliasflow

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

	"github.com/microbus-io/fabric/verify/aliasflow/aliasflowapi"
	"github.com/microbus-io/fabric/verify/aliasflow/resources"
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
	_ aliasflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = aliasflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskS(ctx context.Context, flow *workflow.Flow, branch string) (branchOut string, err error) // MARKER: TaskS
	TaskA(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error)    // MARKER: TaskA
	TaskB(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error)    // MARKER: TaskB
	TaskC(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error)    // MARKER: TaskC
	TaskD(ctx context.Context, flow *workflow.Flow, path string) (pathOut string, err error)    // MARKER: TaskD
	Alias(ctx context.Context) (graph *workflow.Graph, err error)                               // MARKER: Alias
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
	svc.SetDescription(`aliasflow.verify exercises a workflow graph in which the same task URL appears at two distinct positions under two distinct node names.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: TaskS
		"TaskS", svc.doTaskS,
		sub.At(aliasflowapi.TaskS.Method, aliasflowapi.TaskS.Route),
		sub.Description(`TaskS dispatches to either the long path (a -> b -> c) or the alternate path (bPrime -> d) via flow.Goto.`),
		sub.Task(aliasflowapi.TaskSIn{}, aliasflowapi.TaskSOut{}),
	)
	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(aliasflowapi.TaskA.Method, aliasflowapi.TaskA.Route),
		sub.Description(`TaskA appends "A" to the path.`),
		sub.Task(aliasflowapi.TaskAIn{}, aliasflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: TaskB
		"TaskB", svc.doTaskB,
		sub.At(aliasflowapi.TaskB.Method, aliasflowapi.TaskB.Route),
		sub.Description(`TaskB appends "B" to the path. Reused at two graph positions under names "b" and "bPrime".`),
		sub.Task(aliasflowapi.TaskBIn{}, aliasflowapi.TaskBOut{}),
	)
	svc.Subscribe( // MARKER: TaskC
		"TaskC", svc.doTaskC,
		sub.At(aliasflowapi.TaskC.Method, aliasflowapi.TaskC.Route),
		sub.Description(`TaskC appends "C" to the path.`),
		sub.Task(aliasflowapi.TaskCIn{}, aliasflowapi.TaskCOut{}),
	)
	svc.Subscribe( // MARKER: TaskD
		"TaskD", svc.doTaskD,
		sub.At(aliasflowapi.TaskD.Method, aliasflowapi.TaskD.Route),
		sub.Description(`TaskD appends "D" to the path.`),
		sub.Task(aliasflowapi.TaskDIn{}, aliasflowapi.TaskDOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: Alias
		"Alias", svc.doAlias,
		sub.At(aliasflowapi.Alias.Method, aliasflowapi.Alias.Route),
		sub.Description(`Alias defines the graph s -> a -> b -> c -> END with an alternate Goto-driven path s -> bPrime -> d -> END. Nodes "b" and "bPrime" share the same task URL.`),
		sub.Workflow(aliasflowapi.AliasIn{}, aliasflowapi.AliasOut{}),
	)

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
	// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
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

// doTaskS handles marshaling for TaskS.
func (svc *Intermediate) doTaskS(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskS
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in aliasflowapi.TaskSIn
	flow.ParseState(&in)
	var out aliasflowapi.TaskSOut
	out.BranchOut, err = svc.TaskS(r.Context(), &flow, in.Branch)
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

// doTaskA handles marshaling for TaskA.
func (svc *Intermediate) doTaskA(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskA
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in aliasflowapi.TaskAIn
	flow.ParseState(&in)
	var out aliasflowapi.TaskAOut
	out.PathOut, err = svc.TaskA(r.Context(), &flow, in.Path)
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
	var in aliasflowapi.TaskBIn
	flow.ParseState(&in)
	var out aliasflowapi.TaskBOut
	out.PathOut, err = svc.TaskB(r.Context(), &flow, in.Path)
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
	var in aliasflowapi.TaskCIn
	flow.ParseState(&in)
	var out aliasflowapi.TaskCOut
	out.PathOut, err = svc.TaskC(r.Context(), &flow, in.Path)
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

// doTaskD handles marshaling for TaskD.
func (svc *Intermediate) doTaskD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskD
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in aliasflowapi.TaskDIn
	flow.ParseState(&in)
	var out aliasflowapi.TaskDOut
	out.PathOut, err = svc.TaskD(r.Context(), &flow, in.Path)
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

// doAlias handles marshaling for Alias.
func (svc *Intermediate) doAlias(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Alias
	graph, err := svc.Alias(r.Context())
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

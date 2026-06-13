package switchflow

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

	"github.com/microbus-io/fabric/verify/switchflow/resources"
	"github.com/microbus-io/fabric/verify/switchflow/switchflowapi"
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
	_ switchflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = switchflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Router(ctx context.Context, flow *workflow.Flow, amount int) (amountOut int, err error)        // MARKER: Router
	HandleHigh(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error)    // MARKER: HandleHigh
	HandleMid(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error)     // MARKER: HandleMid
	HandleLow(ctx context.Context, flow *workflow.Flow, amount int) (branch string, err error)     // MARKER: HandleLow
	Switch(ctx context.Context) (graph *workflow.Graph, err error)                                  // MARKER: Switch
	SwitchNoMatch(ctx context.Context) (graph *workflow.Graph, err error)                           // MARKER: SwitchNoMatch
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
	svc.SetDescription(`switchflow.verify exercises AddTransitionSwitch for first-match-wins routing.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add task endpoints here
	svc.Subscribe( // MARKER: Router
		"Router", svc.doRouter,
		sub.At(switchflowapi.Router.Method, switchflowapi.Router.Route),
		sub.Description(`Router passes the amount through to state so downstream Switch transitions can match on it.`),
		sub.Task(switchflowapi.RouterIn{}, switchflowapi.RouterOut{}),
	)
	svc.Subscribe( // MARKER: HandleHigh
		"HandleHigh", svc.doHandleHigh,
		sub.At(switchflowapi.HandleHigh.Method, switchflowapi.HandleHigh.Route),
		sub.Description(`HandleHigh runs when the first switch arm matches (amount>=10000).`),
		sub.Task(switchflowapi.HandleHighIn{}, switchflowapi.HandleHighOut{}),
	)
	svc.Subscribe( // MARKER: HandleMid
		"HandleMid", svc.doHandleMid,
		sub.At(switchflowapi.HandleMid.Method, switchflowapi.HandleMid.Route),
		sub.Description(`HandleMid runs when the second switch arm matches (amount>=1000).`),
		sub.Task(switchflowapi.HandleMidIn{}, switchflowapi.HandleMidOut{}),
	)
	svc.Subscribe( // MARKER: HandleLow
		"HandleLow", svc.doHandleLow,
		sub.At(switchflowapi.HandleLow.Method, switchflowapi.HandleLow.Route),
		sub.Description(`HandleLow runs as the default switch arm (when="true").`),
		sub.Task(switchflowapi.HandleLowIn{}, switchflowapi.HandleLowOut{}),
	)

	// HINT: Add graph endpoints here
	svc.Subscribe( // MARKER: Switch
		"Switch", svc.doSwitch,
		sub.At(switchflowapi.Switch.Method, switchflowapi.Switch.Route),
		sub.Description(`Switch defines a graph that routes the amount to one of three handlers via a switch ladder ending in a when="true" default.`),
		sub.Workflow(switchflowapi.SwitchIn{}, switchflowapi.SwitchOut{}),
	)
	svc.Subscribe( // MARKER: SwitchNoMatch
		"SwitchNoMatch", svc.doSwitchNoMatch,
		sub.At(switchflowapi.SwitchNoMatch.Method, switchflowapi.SwitchNoMatch.Route),
		sub.Description(`SwitchNoMatch defines a graph whose final switch arm is when="false", so inputs below the lowest threshold end the flow without routing.`),
		sub.Workflow(switchflowapi.SwitchNoMatchIn{}, switchflowapi.SwitchNoMatchOut{}),
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

// doRouter handles marshaling for Router.
func (svc *Intermediate) doRouter(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Router
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in switchflowapi.RouterIn
	flow.ParseState(&in)
	var out switchflowapi.RouterOut
	out.AmountOut, err = svc.Router(r.Context(), &flow, in.Amount)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doHandleHigh handles marshaling for HandleHigh.
func (svc *Intermediate) doHandleHigh(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HandleHigh
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in switchflowapi.HandleHighIn
	flow.ParseState(&in)
	var out switchflowapi.HandleHighOut
	out.Branch, err = svc.HandleHigh(r.Context(), &flow, in.Amount)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doHandleMid handles marshaling for HandleMid.
func (svc *Intermediate) doHandleMid(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HandleMid
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in switchflowapi.HandleMidIn
	flow.ParseState(&in)
	var out switchflowapi.HandleMidOut
	out.Branch, err = svc.HandleMid(r.Context(), &flow, in.Amount)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doHandleLow handles marshaling for HandleLow.
func (svc *Intermediate) doHandleLow(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HandleLow
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in switchflowapi.HandleLowIn
	flow.ParseState(&in)
	var out switchflowapi.HandleLowOut
	out.Branch, err = svc.HandleLow(r.Context(), &flow, in.Amount)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doSwitch handles marshaling for Switch.
func (svc *Intermediate) doSwitch(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Switch
	graph, err := svc.Switch(r.Context())
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

// doSwitchNoMatch handles marshaling for SwitchNoMatch.
func (svc *Intermediate) doSwitchNoMatch(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SwitchNoMatch
	graph, err := svc.SwitchNoMatch(r.Context())
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

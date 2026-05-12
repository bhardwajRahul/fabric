package intrathreadgotoflow

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

	"github.com/microbus-io/fabric/verify/intrathreadgotoflow/intrathreadgotoflowapi"
	"github.com/microbus-io/fabric/verify/intrathreadgotoflow/resources"
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
	_ intrathreadgotoflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = intrathreadgotoflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, target int) (targetOut int, err error)                       // MARKER: TaskA
	LoopTask(ctx context.Context, flow *workflow.Flow, loops, target int) (loopsOut int, err error)              // MARKER: LoopTask
	NormalC(ctx context.Context, flow *workflow.Flow) (stamp string, err error)                                   // MARKER: NormalC
	TaskD(ctx context.Context, flow *workflow.Flow, loops int, stamp string) (finalResult string, err error)      // MARKER: TaskD
	IntraThreadGoto(ctx context.Context) (graph *workflow.Graph, err error)                                       // MARKER: IntraThreadGoto
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
	svc.SetDescription(`intrathreadgotoflow.verify is the SKIP-marked intra-thread Goto pattern (pending lineage redesign).`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(intrathreadgotoflowapi.TaskA.Method, intrathreadgotoflowapi.TaskA.Route),
		sub.Description(`TaskA passes target through.`),
		sub.Task(intrathreadgotoflowapi.TaskAIn{}, intrathreadgotoflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: LoopTask
		"LoopTask", svc.doLoopTask,
		sub.At(intrathreadgotoflowapi.LoopTask.Method, intrathreadgotoflowapi.LoopTask.Route),
		sub.Description(`LoopTask self-loops via flow.Goto until loops reaches target.`),
		sub.Task(intrathreadgotoflowapi.LoopTaskIn{}, intrathreadgotoflowapi.LoopTaskOut{}),
	)
	svc.Subscribe( // MARKER: NormalC
		"NormalC", svc.doNormalC,
		sub.At(intrathreadgotoflowapi.NormalC.Method, intrathreadgotoflowapi.NormalC.Route),
		sub.Description(`NormalC produces a stamp.`),
		sub.Task(intrathreadgotoflowapi.NormalCIn{}, intrathreadgotoflowapi.NormalCOut{}),
	)
	svc.Subscribe( // MARKER: TaskD
		"TaskD", svc.doTaskD,
		sub.At(intrathreadgotoflowapi.TaskD.Method, intrathreadgotoflowapi.TaskD.Route),
		sub.Description(`TaskD is the outer fan-in.`),
		sub.Task(intrathreadgotoflowapi.TaskDIn{}, intrathreadgotoflowapi.TaskDOut{}),
	)

	svc.Subscribe( // MARKER: IntraThreadGoto
		"IntraThreadGoto", svc.doIntraThreadGoto,
		sub.At(intrathreadgotoflowapi.IntraThreadGoto.Method, intrathreadgotoflowapi.IntraThreadGoto.Route),
		sub.Description(`IntraThreadGoto defines A -> {LoopTask (self-Goto), NormalC} -> D.`),
		sub.Workflow(intrathreadgotoflowapi.IntraThreadGotoIn{}, intrathreadgotoflowapi.IntraThreadGotoOut{}),
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
	var in intrathreadgotoflowapi.TaskAIn
	flow.ParseState(&in)
	var out intrathreadgotoflowapi.TaskAOut
	out.TargetOut, err = svc.TaskA(r.Context(), &flow, in.Target)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doLoopTask handles marshaling for LoopTask.
func (svc *Intermediate) doLoopTask(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: LoopTask
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in intrathreadgotoflowapi.LoopTaskIn
	flow.ParseState(&in)
	var out intrathreadgotoflowapi.LoopTaskOut
	out.LoopsOut, err = svc.LoopTask(r.Context(), &flow, in.Loops, in.Target)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doNormalC handles marshaling for NormalC.
func (svc *Intermediate) doNormalC(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NormalC
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in intrathreadgotoflowapi.NormalCIn
	flow.ParseState(&in)
	var out intrathreadgotoflowapi.NormalCOut
	out.Stamp, err = svc.NormalC(r.Context(), &flow)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doTaskD handles marshaling for TaskD.
func (svc *Intermediate) doTaskD(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TaskD
	var flow workflow.Flow
	if err = json.NewDecoder(r.Body).Decode(&flow); err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in intrathreadgotoflowapi.TaskDIn
	flow.ParseState(&in)
	var out intrathreadgotoflowapi.TaskDOut
	out.FinalResult, err = svc.TaskD(r.Context(), &flow, in.Loops, in.Stamp)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doIntraThreadGoto handles marshaling for IntraThreadGoto.
func (svc *Intermediate) doIntraThreadGoto(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: IntraThreadGoto
	graph, err := svc.IntraThreadGoto(r.Context())
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

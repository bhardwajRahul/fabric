package interruptflow

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

	"github.com/microbus-io/fabric/verify/interruptflow/interruptflowapi"
	"github.com/microbus-io/fabric/verify/interruptflow/resources"
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
	_ interruptflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = interruptflowapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	TaskA(ctx context.Context, flow *workflow.Flow, prompt string) (promptOut string, err error)               // MARKER: TaskA
	AwaitInput(ctx context.Context, flow *workflow.Flow, userInput string) (userInputOut string, err error)   // MARKER: AwaitInput
	Compose(ctx context.Context, flow *workflow.Flow, prompt, userInput string) (result string, err error)    // MARKER: Compose
	Interruptor(ctx context.Context) (graph *workflow.Graph, err error)                                       // MARKER: Interruptor
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
	svc.SetDescription(`interruptflow.verify exercises flow.Interrupt + Resume.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: TaskA
		"TaskA", svc.doTaskA,
		sub.At(interruptflowapi.TaskA.Method, interruptflowapi.TaskA.Route),
		sub.Description(`TaskA passes the prompt through.`),
		sub.Task(interruptflowapi.TaskAIn{}, interruptflowapi.TaskAOut{}),
	)
	svc.Subscribe( // MARKER: AwaitInput
		"AwaitInput", svc.doAwaitInput,
		sub.At(interruptflowapi.AwaitInput.Method, interruptflowapi.AwaitInput.Route),
		sub.Description(`AwaitInput interrupts until userInput is provided.`),
		sub.Task(interruptflowapi.AwaitInputIn{}, interruptflowapi.AwaitInputOut{}),
	)
	svc.Subscribe( // MARKER: Compose
		"Compose", svc.doCompose,
		sub.At(interruptflowapi.Compose.Method, interruptflowapi.Compose.Route),
		sub.Description(`Compose joins prompt and userInput.`),
		sub.Task(interruptflowapi.ComposeIn{}, interruptflowapi.ComposeOut{}),
	)

	svc.Subscribe( // MARKER: Interruptor
		"Interruptor", svc.doInterruptor,
		sub.At(interruptflowapi.Interruptor.Method, interruptflowapi.Interruptor.Route),
		sub.Description(`Interruptor defines A -> AwaitInput -> Compose.`),
		sub.Workflow(interruptflowapi.InterruptorIn{}, interruptflowapi.InterruptorOut{}),
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
	var in interruptflowapi.TaskAIn
	flow.ParseState(&in)
	var out interruptflowapi.TaskAOut
	out.PromptOut, err = svc.TaskA(r.Context(), &flow, in.Prompt)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doAwaitInput handles marshaling for AwaitInput.
func (svc *Intermediate) doAwaitInput(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AwaitInput
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in interruptflowapi.AwaitInputIn
	flow.ParseState(&in)
	var out interruptflowapi.AwaitInputOut
	out.UserInputOut, err = svc.AwaitInput(r.Context(), &flow, in.UserInput)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doCompose handles marshaling for Compose.
func (svc *Intermediate) doCompose(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Compose
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in interruptflowapi.ComposeIn
	flow.ParseState(&in)
	var out interruptflowapi.ComposeOut
	out.Result, err = svc.Compose(r.Context(), &flow, in.Prompt, in.UserInput)
	if err != nil {
		return err
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	return errors.Trace(json.NewEncoder(w).Encode(&flow))
}

// doInterruptor handles marshaling for Interruptor.
func (svc *Intermediate) doInterruptor(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Interruptor
	graph, err := svc.Interruptor(r.Context())
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

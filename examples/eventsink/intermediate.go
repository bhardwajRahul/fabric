package eventsink

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/examples/eventsink/eventsinkapi"
	"github.com/microbus-io/fabric/examples/eventsink/resources"
	"github.com/microbus-io/fabric/examples/eventsource/eventsourceapi"
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
	_ eventsinkapi.Client
)

const (
	Hostname = eventsinkapi.Hostname
	Version  = 260
)

// ToDo defines the interface of the microservice.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Registered(ctx context.Context) (emails []string, err error)                 // MARKER: Registered
	OnAllowRegister(ctx context.Context, email string) (allow bool, err error)   // MARKER: OnAllowRegister
	OnRegistered(ctx context.Context, email string) (err error)                  // MARKER: OnRegistered
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

// Intermediate is the intermediary between the service implementation and the framework.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new Intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`The event sink microservice handles events that are fired by the event source microservice.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functional endpoints
	svc.Subscribe("ANY", eventsinkapi.RouteOfRegistered, svc.doRegistered) // MARKER: Registered

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

	// Inbound event sinks
	eventsourceapi.NewHook(svc).OnAllowRegister(svc.OnAllowRegister) // MARKER: OnAllowRegister
	eventsourceapi.NewHook(svc).OnRegistered(svc.OnRegistered)       // MARKER: OnRegistered

	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
		{ // MARKER: Registered
			Type:        "function",
			Name:        "Registered",
			Method:      "ANY",
			Route:       eventsinkapi.RouteOfRegistered,
			Summary:     "Registered() (emails []string)",
			Description: `Registered returns the list of registered users.`,
			InputArgs:   eventsinkapi.RegisteredIn{},
			OutputArgs:  eventsinkapi.RegisteredOut{},
		},
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
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

// doRegistered handles marshaling for the Registered function.
func (svc *Intermediate) doRegistered(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Registered
	var i eventsinkapi.RegisteredIn
	var o eventsinkapi.RegisteredOut
	err = httpx.ReadInputPayload(r, eventsinkapi.RouteOfRegistered, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Emails, err = svc.Registered(r.Context())
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

package configurator

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

	"github.com/microbus-io/fabric/coreservices/configurator/configuratorapi"
	"github.com/microbus-io/fabric/coreservices/configurator/resources"
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
	_ configuratorapi.Client
)

const (
	Hostname = configuratorapi.Hostname
	Version  = 252
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Values(ctx context.Context, names []string) (values map[string]string, err error)                   // MARKER: Values
	Refresh(ctx context.Context) (err error)                                                             // MARKER: Refresh
	SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) // MARKER: SyncRepo
	Values443(ctx context.Context, names []string) (values map[string]string, err error)                 // MARKER: Values443
	Refresh443(ctx context.Context) (err error)                                                          // MARKER: Refresh443
	Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)  // MARKER: Sync443
	PeriodicRefresh(ctx context.Context) (err error)                                                     // MARKER: PeriodicRefresh
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
	svc.SetDescription(`The Configurator is a core microservice that centralizes the dissemination of configuration values to other microservices.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functional endpoints
	svc.Subscribe("ANY", configuratorapi.RouteOfValues, svc.doValues)              // MARKER: Values
	svc.Subscribe("ANY", configuratorapi.RouteOfRefresh, svc.doRefresh)            // MARKER: Refresh
	svc.Subscribe("ANY", configuratorapi.RouteOfSyncRepo, svc.doSyncRepo, sub.NoQueue()) // MARKER: SyncRepo

	// Deprecated functional endpoints
	svc.Subscribe("ANY", configuratorapi.RouteOfValues443, svc.doValues443)   // MARKER: Values443
	svc.Subscribe("ANY", configuratorapi.RouteOfRefresh443, svc.doRefresh443) // MARKER: Refresh443
	svc.Subscribe("ANY", configuratorapi.RouteOfSync443, svc.doSync443)       // MARKER: Sync443

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// Tickers
	svc.StartTicker("PeriodicRefresh", 20*time.Minute, svc.doPeriodicRefresh) // MARKER: PeriodicRefresh

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

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
		{ // MARKER: Values
			Type:        "function",
			Name:        "Values",
			Method:      "ANY",
			Route:       configuratorapi.RouteOfValues,
			Summary:     "Values(names []string) (values map[string]string)",
			Description: `Values returns the values associated with the specified config property names for the caller microservice.`,
			InputArgs:   configuratorapi.ValuesIn{},
			OutputArgs:  configuratorapi.ValuesOut{},
		},
		{ // MARKER: Refresh
			Type:    "function",
			Name:    "Refresh",
			Method:  "ANY",
			Route:   configuratorapi.RouteOfRefresh,
			Summary: "Refresh()",
			Description: `Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.`,
			InputArgs:  configuratorapi.RefreshIn{},
			OutputArgs: configuratorapi.RefreshOut{},
		},
		{ // MARKER: SyncRepo
			Type:        "function",
			Name:        "SyncRepo",
			Method:      "ANY",
			Route:       configuratorapi.RouteOfSyncRepo,
			Summary:     "SyncRepo(timestamp time.Time, values map[string]map[string]string)",
			Description: `SyncRepo is used to synchronize values among replica peers of the configurator.`,
			InputArgs:   configuratorapi.SyncRepoIn{},
			OutputArgs:  configuratorapi.SyncRepoOut{},
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

// doValues handles marshaling for the Values function.
func (svc *Intermediate) doValues(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Values
	var i configuratorapi.ValuesIn
	var o configuratorapi.ValuesOut
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfValues, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Values, err = svc.Values(r.Context(), i.Names)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doRefresh handles marshaling for the Refresh function.
func (svc *Intermediate) doRefresh(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Refresh
	var i configuratorapi.RefreshIn
	var o configuratorapi.RefreshOut
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfRefresh, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.Refresh(r.Context())
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doSyncRepo handles marshaling for the SyncRepo function.
func (svc *Intermediate) doSyncRepo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SyncRepo
	var i configuratorapi.SyncRepoIn
	var o configuratorapi.SyncRepoOut
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfSyncRepo, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.SyncRepo(r.Context(), i.Timestamp, i.Values)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doValues443 handles marshaling for the Values443 function.
func (svc *Intermediate) doValues443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Values443
	var i configuratorapi.Values443In
	var o configuratorapi.Values443Out
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfValues443, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Values, err = svc.Values443(r.Context(), i.Names)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doRefresh443 handles marshaling for the Refresh443 function.
func (svc *Intermediate) doRefresh443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Refresh443
	var i configuratorapi.Refresh443In
	var o configuratorapi.Refresh443Out
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfRefresh443, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.Refresh443(r.Context())
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doSync443 handles marshaling for the Sync443 function.
func (svc *Intermediate) doSync443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Sync443
	var i configuratorapi.Sync443In
	var o configuratorapi.Sync443Out
	err = httpx.ReadInputPayload(r, configuratorapi.RouteOfSync443, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.Sync443(r.Context(), i.Timestamp, i.Values)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doPeriodicRefresh handles the PeriodicRefresh ticker.
func (svc *Intermediate) doPeriodicRefresh(ctx context.Context) (err error) { // MARKER: PeriodicRefresh
	return svc.PeriodicRefresh(ctx)
}

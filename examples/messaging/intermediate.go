package messaging

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

	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
	"github.com/microbus-io/fabric/examples/messaging/resources"
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
	_ messagingapi.Client
)

const (
	Hostname = messagingapi.Hostname
	Version  = 229
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Home(w http.ResponseWriter, r *http.Request) (err error)         // MARKER: Home
	NoQueue(w http.ResponseWriter, r *http.Request) (err error)      // MARKER: NoQueue
	DefaultQueue(w http.ResponseWriter, r *http.Request) (err error) // MARKER: DefaultQueue
	CacheLoad(w http.ResponseWriter, r *http.Request) (err error)    // MARKER: CacheLoad
	CacheStore(w http.ResponseWriter, r *http.Request) (err error)   // MARKER: CacheStore
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
	svc.SetDescription(`The Messaging microservice demonstrates service-to-service communication patterns.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// Web endpoints
	svc.Subscribe("GET", messagingapi.RouteOfHome, svc.Home)                      // MARKER: Home
	svc.Subscribe("GET", messagingapi.RouteOfNoQueue, svc.NoQueue, sub.NoQueue()) // MARKER: NoQueue
	svc.Subscribe("GET", messagingapi.RouteOfDefaultQueue, svc.DefaultQueue)      // MARKER: DefaultQueue
	svc.Subscribe("GET", messagingapi.RouteOfCacheLoad, svc.CacheLoad)            // MARKER: CacheLoad
	svc.Subscribe("GET", messagingapi.RouteOfCacheStore, svc.CacheStore)          // MARKER: CacheStore

	// HINT: Add metrics here

	// HINT: Add tickers here

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
		{ // MARKER: Home
			Type:        "web",
			Name:        "Home",
			Method:      "GET",
			Route:       messagingapi.RouteOfHome,
			Summary:     "Home()",
			Description: `Home demonstrates making requests using multicast and unicast request/response patterns.`,
		},
		{ // MARKER: NoQueue
			Type:        "web",
			Name:        "NoQueue",
			Method:      "GET",
			Route:       messagingapi.RouteOfNoQueue,
			Summary:     "NoQueue()",
			Description: "NoQueue demonstrates how the NoQueue subscription option is used to create\na multicast request/response communication pattern.\nAll instances of this microservice will respond to each request.",
		},
		{ // MARKER: DefaultQueue
			Type:        "web",
			Name:        "DefaultQueue",
			Method:      "GET",
			Route:       messagingapi.RouteOfDefaultQueue,
			Summary:     "DefaultQueue()",
			Description: "DefaultQueue demonstrates how the DefaultQueue subscription option is used to create\na unicast request/response communication pattern.\nOnly one of the instances of this microservice will respond to each request.",
		},
		{ // MARKER: CacheLoad
			Type:        "web",
			Name:        "CacheLoad",
			Method:      "GET",
			Route:       messagingapi.RouteOfCacheLoad,
			Summary:     "CacheLoad()",
			Description: `CacheLoad looks up an element in the distributed cache of the microservice.`,
		},
		{ // MARKER: CacheStore
			Type:        "web",
			Name:        "CacheStore",
			Method:      "GET",
			Route:       messagingapi.RouteOfCacheStore,
			Summary:     "CacheStore()",
			Description: `CacheStore stores an element in the distributed cache of the microservice.`,
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

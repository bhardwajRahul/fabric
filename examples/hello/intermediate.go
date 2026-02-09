package hello

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

	"github.com/microbus-io/fabric/examples/hello/helloapi"
	"github.com/microbus-io/fabric/examples/hello/resources"
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
	_ helloapi.Client
)

const (
	Hostname = helloapi.Hostname
	Version  = 325
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Hello(w http.ResponseWriter, r *http.Request) error                    // MARKER: Hello
	Echo(w http.ResponseWriter, r *http.Request) error                     // MARKER: Echo
	Ping(w http.ResponseWriter, r *http.Request) error                     // MARKER: Ping
	Calculator(w http.ResponseWriter, r *http.Request) error               // MARKER: Calculator
	BusPNG(w http.ResponseWriter, r *http.Request) (err error)             // MARKER: BusPNG
	Localization(w http.ResponseWriter, r *http.Request) (err error)       // MARKER: Localization
	Root(w http.ResponseWriter, r *http.Request) (err error)               // MARKER: Root
	TickTock(ctx context.Context) error                                    // MARKER: TickTock
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
	svc.SetDescription(`The Hello microservice demonstrates the various capabilities of a microservice.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// Web endpoints
	svc.Subscribe("ANY", helloapi.RouteOfHello, svc.Hello)               // MARKER: Hello
	svc.Subscribe("ANY", helloapi.RouteOfEcho, svc.Echo)                 // MARKER: Echo
	svc.Subscribe("ANY", helloapi.RouteOfPing, svc.Ping)                 // MARKER: Ping
	svc.Subscribe("ANY", helloapi.RouteOfCalculator, svc.Calculator)     // MARKER: Calculator
	svc.Subscribe("GET", helloapi.RouteOfBusPNG, svc.BusPNG)             // MARKER: BusPNG
	svc.Subscribe("ANY", helloapi.RouteOfLocalization, svc.Localization)  // MARKER: Localization
	svc.Subscribe("ANY", helloapi.RouteOfRoot, svc.Root)                 // MARKER: Root

	// HINT: Add metrics here

	// Tickers
	svc.StartTicker("TickTock", 10*time.Second, svc.TickTock) // MARKER: TickTock

	// Config properties
	svc.DefineConfig( // MARKER: Greeting
		"Greeting",
		cfg.Description(`Greeting to use.`),
		cfg.DefaultValue("Hello"),
	)
	svc.DefineConfig( // MARKER: Repeat
		"Repeat",
		cfg.Description(`Repeat indicates how many times to display the greeting.`),
		cfg.DefaultValue("1"),
		cfg.Validation("int [0,100]"),
	)

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
		{ // MARKER: Hello
			Type:        "web",
			Name:        "Hello",
			Method:      "ANY",
			Route:       helloapi.RouteOfHello,
			Summary:     "Hello()",
			Description: `Hello prints a greeting.`,
		},
		{ // MARKER: Echo
			Type:        "web",
			Name:        "Echo",
			Method:      "ANY",
			Route:       helloapi.RouteOfEcho,
			Summary:     "Echo()",
			Description: `Echo back the incoming request in wire format.`,
		},
		{ // MARKER: Ping
			Type:        "web",
			Name:        "Ping",
			Method:      "ANY",
			Route:       helloapi.RouteOfPing,
			Summary:     "Ping()",
			Description: `Ping all microservices and list them.`,
		},
		{ // MARKER: Calculator
			Type:        "web",
			Name:        "Calculator",
			Method:      "ANY",
			Route:       helloapi.RouteOfCalculator,
			Summary:     "Calculator()",
			Description: `Calculator renders a UI for a calculator.
The calculation operation is delegated to another microservice in order to demonstrate
a call from one microservice to another.`,
		},
		{ // MARKER: BusPNG
			Type:        "web",
			Name:        "BusPNG",
			Method:      "GET",
			Route:       helloapi.RouteOfBusPNG,
			Summary:     "BusPNG()",
			Description: `BusPNG serves an image from the embedded resources.`,
		},
		{ // MARKER: Localization
			Type:        "web",
			Name:        "Localization",
			Method:      "ANY",
			Route:       helloapi.RouteOfLocalization,
			Summary:     "Localization()",
			Description: `Localization prints hello in the language best matching the request's Accept-Language header.`,
		},
		{ // MARKER: Root
			Type:        "web",
			Name:        "Root",
			Method:      "ANY",
			Route:       helloapi.RouteOfRoot,
			Summary:     "Root()",
			Description: `Root is the top-most root page.`,
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

/*
Greeting to use.
*/
func (svc *Intermediate) Greeting() (value string) { // MARKER: Greeting
	return svc.Config("Greeting")
}

/*
SetGreeting sets the value of the configuration property.
*/
func (svc *Intermediate) SetGreeting(value string) (err error) { // MARKER: Greeting
	return svc.SetConfig("Greeting", value)
}

/*
Repeat indicates how many times to display the greeting.
*/
func (svc *Intermediate) Repeat() (value int) { // MARKER: Repeat
	_val := svc.Config("Repeat")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetRepeat sets the value of the configuration property.
*/
func (svc *Intermediate) SetRepeat(value int) (err error) { // MARKER: Repeat
	return svc.SetConfig("Repeat", strconv.Itoa(value))
}

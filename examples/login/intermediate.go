package login

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

	"github.com/microbus-io/fabric/examples/login/loginapi"
	"github.com/microbus-io/fabric/examples/login/resources"
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
	_ loginapi.Client
)

const (
	Hostname = loginapi.Hostname
	Version  = 92
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Login(w http.ResponseWriter, r *http.Request) (err error)       // MARKER: Login
	Logout(w http.ResponseWriter, r *http.Request) (err error)      // MARKER: Logout
	Welcome(w http.ResponseWriter, r *http.Request) (err error)     // MARKER: Welcome
	AdminOnly(w http.ResponseWriter, r *http.Request) (err error)   // MARKER: AdminOnly
	ManagerOnly(w http.ResponseWriter, r *http.Request) (err error) // MARKER: ManagerOnly
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
	svc.SetDescription(`The Login microservice demonstrates usage of authentication and authorization.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// Web endpoints
	svc.Subscribe("ANY", loginapi.RouteOfLogin, svc.Login)                                                          // MARKER: Login
	svc.Subscribe("ANY", loginapi.RouteOfLogout, svc.Logout)                                                        // MARKER: Logout
	svc.Subscribe("ANY", loginapi.RouteOfWelcome, svc.Welcome, sub.RequiredClaims(`roles.a || roles.m || roles.u`)) // MARKER: Welcome
	svc.Subscribe("GET", loginapi.RouteOfAdminOnly, svc.AdminOnly, sub.RequiredClaims(`roles.a`))                   // MARKER: AdminOnly
	svc.Subscribe("GET", loginapi.RouteOfManagerOnly, svc.ManagerOnly, sub.RequiredClaims(`roles.m`))               // MARKER: ManagerOnly

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
		{ // MARKER: Login
			Type:    "web",
			Name:    "Login",
			Method:  "ANY",
			Route:   loginapi.RouteOfLogin,
			Summary: "Login()",
			Description: `Login renders a simple login screen that authenticates a user.
Known users are hardcoded as "admin", "manager" and "user".
The password is "password".`,
		},
		{ // MARKER: Logout
			Type:        "web",
			Name:        "Logout",
			Method:      "ANY",
			Route:       loginapi.RouteOfLogout,
			Summary:     "Logout()",
			Description: `Logout renders a page that logs out the user.`,
		},
		{ // MARKER: Welcome
			Type:    "web",
			Name:    "Welcome",
			Method:  "ANY",
			Route:   loginapi.RouteOfWelcome,
			Summary: "Welcome()",
			Description: `Welcome renders a page that is shown to the user after a successful login.
Rendering is adjusted based on the user's roles.`,
			RequiredClaims: `roles.a || roles.m || roles.u`,
		},
		{ // MARKER: AdminOnly
			Type:           "web",
			Name:           "AdminOnly",
			Method:         "GET",
			Route:          loginapi.RouteOfAdminOnly,
			Summary:        "AdminOnly()",
			Description:    `AdminOnly is only accessible by admins.`,
			RequiredClaims: `roles.a`,
		},
		{ // MARKER: ManagerOnly
			Type:           "web",
			Name:           "ManagerOnly",
			Method:         "GET",
			Route:          loginapi.RouteOfManagerOnly,
			Summary:        "ManagerOnly()",
			Description:    `ManagerOnly is only accessible by managers.`,
			RequiredClaims: `roles.m`,
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

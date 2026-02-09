package directory

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

	"github.com/microbus-io/fabric/examples/directory/directoryapi"
	"github.com/microbus-io/fabric/examples/directory/resources"
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
	_ directoryapi.Client
)

const (
	Hostname = directoryapi.Hostname
	Version  = 280
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, httpRequestBody directoryapi.Person) (key directoryapi.PersonKey, err error) // MARKER: Create
	Load(ctx context.Context, key directoryapi.PersonKey) (httpResponseBody directoryapi.Person, err error)  // MARKER: Load
	Delete(ctx context.Context, key directoryapi.PersonKey) (err error)                                      // MARKER: Delete
	Update(ctx context.Context, key directoryapi.PersonKey, httpRequestBody directoryapi.Person) (err error) // MARKER: Update
	LoadByEmail(ctx context.Context, email string) (httpResponseBody directoryapi.Person, err error)         // MARKER: LoadByEmail
	List(ctx context.Context) (httpResponseBody []directoryapi.PersonKey, err error)                         // MARKER: List
	WebUI(w http.ResponseWriter, r *http.Request) (err error)                                                // MARKER: WebUI
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
	svc.SetDescription(`The directory microservice exposes a RESTful API for persisting personal records in a SQL database.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functional endpoints
	svc.Subscribe("POST", directoryapi.RouteOfCreate, svc.doCreate)          // MARKER: Create
	svc.Subscribe("GET", directoryapi.RouteOfLoad, svc.doLoad)               // MARKER: Load
	svc.Subscribe("DELETE", directoryapi.RouteOfDelete, svc.doDelete)        // MARKER: Delete
	svc.Subscribe("PUT", directoryapi.RouteOfUpdate, svc.doUpdate)           // MARKER: Update
	svc.Subscribe("GET", directoryapi.RouteOfLoadByEmail, svc.doLoadByEmail) // MARKER: LoadByEmail
	svc.Subscribe("GET", directoryapi.RouteOfList, svc.doList)               // MARKER: List

	// Web endpoints
	svc.Subscribe("ANY", directoryapi.RouteOfWebUI, svc.WebUI) // MARKER: WebUI

	// HINT: Add metrics here

	// HINT: Add tickers here

	// Config properties
	svc.DefineConfig( // MARKER: SQL
		"SQL",
		cfg.Description(`SQL is the connection string to the database.`),
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
		{ // MARKER: Create
			Type:        "function",
			Name:        "Create",
			Method:      "POST",
			Route:       directoryapi.RouteOfCreate,
			Summary:     "Create(Person) (key PersonKey)",
			Description: `Create registers the person in the directory.`,
			InputArgs:   directoryapi.CreateIn{},
			OutputArgs:  directoryapi.CreateOut{},
		},
		{ // MARKER: Load
			Type:        "function",
			Name:        "Load",
			Method:      "GET",
			Route:       directoryapi.RouteOfLoad,
			Summary:     "Load(key PersonKey) (Person)",
			Description: `Load looks up a person in the directory.`,
			InputArgs:   directoryapi.LoadIn{},
			OutputArgs:  directoryapi.LoadOut{},
		},
		{ // MARKER: Delete
			Type:        "function",
			Name:        "Delete",
			Method:      "DELETE",
			Route:       directoryapi.RouteOfDelete,
			Summary:     "Delete(key PersonKey)",
			Description: `Delete removes a person from the directory.`,
			InputArgs:   directoryapi.DeleteIn{},
		},
		{ // MARKER: Update
			Type:        "function",
			Name:        "Update",
			Method:      "PUT",
			Route:       directoryapi.RouteOfUpdate,
			Summary:     "Update(key PersonKey, Person)",
			Description: `Update updates the person's data in the directory.`,
			InputArgs:   directoryapi.UpdateIn{},
		},
		{ // MARKER: LoadByEmail
			Type:        "function",
			Name:        "LoadByEmail",
			Method:      "GET",
			Route:       directoryapi.RouteOfLoadByEmail,
			Summary:     "LoadByEmail(email string) (Person)",
			Description: `LoadByEmail looks up a person in the directory by their email.`,
			InputArgs:   directoryapi.LoadByEmailIn{},
			OutputArgs:  directoryapi.LoadByEmailOut{},
		},
		{ // MARKER: List
			Type:        "function",
			Name:        "List",
			Method:      "GET",
			Route:       directoryapi.RouteOfList,
			Summary:     "List() ([]PersonKey)",
			Description: `List returns the keys of all the persons in the directory.`,
			OutputArgs:  directoryapi.ListOut{},
		},
		{ // MARKER: WebUI
			Type:        "web",
			Name:        "WebUI",
			Method:      "ANY",
			Route:       directoryapi.RouteOfWebUI,
			Summary:     "WebUI()",
			Description: `WebUI provides a form for making web requests to the CRUD endpoints.`,
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
SQL is the connection string to the database.
*/
func (svc *Intermediate) SQL() (dsn string) { // MARKER: SQL
	return svc.Config("SQL")
}

/*
SetSQL sets the value of the configuration property.
*/
func (svc *Intermediate) SetSQL(dsn string) (err error) { // MARKER: SQL
	return svc.SetConfig("SQL", dsn)
}

// doCreate handles marshaling for Create.
func (svc *Intermediate) doCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Create
	var i directoryapi.CreateIn
	var o directoryapi.CreateOut
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfCreate, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Key, err = svc.Create(r.Context(), i.HTTPRequestBody)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doLoad handles marshaling for Load.
func (svc *Intermediate) doLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Load
	var i directoryapi.LoadIn
	var o directoryapi.LoadOut
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfLoad, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.HTTPResponseBody, err = svc.Load(r.Context(), i.Key)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doDelete handles marshaling for Delete.
func (svc *Intermediate) doDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Delete
	var i directoryapi.DeleteIn
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfDelete, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.Delete(r.Context(), i.Key)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, directoryapi.DeleteOut{})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doUpdate handles marshaling for Update.
func (svc *Intermediate) doUpdate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Update
	var i directoryapi.UpdateIn
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfUpdate, &i)
	if err != nil {
		return errors.Trace(err)
	}
	err = svc.Update(r.Context(), i.Key, i.HTTPRequestBody)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, directoryapi.UpdateOut{})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doLoadByEmail handles marshaling for LoadByEmail.
func (svc *Intermediate) doLoadByEmail(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: LoadByEmail
	var i directoryapi.LoadByEmailIn
	var o directoryapi.LoadByEmailOut
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfLoadByEmail, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.HTTPResponseBody, err = svc.LoadByEmail(r.Context(), i.Email)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doList handles marshaling for List.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	var i directoryapi.ListIn
	var o directoryapi.ListOut
	err = httpx.ReadInputPayload(r, directoryapi.RouteOfList, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.HTTPResponseBody, err = svc.List(r.Context())
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

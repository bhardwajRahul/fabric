/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package yellowpages

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

	"github.com/microbus-io/fabric/examples/yellowpages/yellowpagesapi"
	"github.com/microbus-io/fabric/examples/yellowpages/resources"
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
	_ yellowpagesapi.Client
)

const (
	Hostname = yellowpagesapi.Hostname
	Version  = 4
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, obj *yellowpagesapi.Person) (objKey yellowpagesapi.PersonKey, err error)                                     // MARKER: Create
	Store(ctx context.Context, obj *yellowpagesapi.Person) (stored bool, err error)                                                         // MARKER: Store
	MustStore(ctx context.Context, obj *yellowpagesapi.Person) (err error)                                                                  // MARKER: MustStore
	Revise(ctx context.Context, obj *yellowpagesapi.Person) (revised bool, err error)                                                       // MARKER: Revise
	MustRevise(ctx context.Context, obj *yellowpagesapi.Person) (err error)                                                                 // MARKER: MustRevise
	Delete(ctx context.Context, objKey yellowpagesapi.PersonKey) (deleted bool, err error)                                                   // MARKER: Delete
	MustDelete(ctx context.Context, objKey yellowpagesapi.PersonKey) (err error)                                                             // MARKER: MustDelete
	List(ctx context.Context, query yellowpagesapi.Query) (objs []*yellowpagesapi.Person, totalCount int, err error)                         // MARKER: List
	Lookup(ctx context.Context, query yellowpagesapi.Query) (obj *yellowpagesapi.Person, found bool, err error)                              // MARKER: Lookup
	MustLookup(ctx context.Context, query yellowpagesapi.Query) (obj *yellowpagesapi.Person, err error)                                     // MARKER: MustLookup
	Load(ctx context.Context, objKey yellowpagesapi.PersonKey) (obj *yellowpagesapi.Person, found bool, err error)                           // MARKER: Load
	MustLoad(ctx context.Context, objKey yellowpagesapi.PersonKey) (obj *yellowpagesapi.Person, err error)                                   // MARKER: MustLoad
	BulkLoad(ctx context.Context, objKeys []yellowpagesapi.PersonKey) (objs []*yellowpagesapi.Person, err error)                             // MARKER: BulkLoad
	BulkDelete(ctx context.Context, objKeys []yellowpagesapi.PersonKey) (deletedKeys []yellowpagesapi.PersonKey, err error)                  // MARKER: BulkDelete
	BulkCreate(ctx context.Context, objs []*yellowpagesapi.Person) (objKeys []yellowpagesapi.PersonKey, err error)                           // MARKER: BulkCreate
	BulkStore(ctx context.Context, objs []*yellowpagesapi.Person) (storedKeys []yellowpagesapi.PersonKey, err error)                         // MARKER: BulkStore
	BulkRevise(ctx context.Context, objs []*yellowpagesapi.Person) (revisedKeys []yellowpagesapi.PersonKey, err error)                       // MARKER: BulkRevise
	Purge(ctx context.Context, query yellowpagesapi.Query) (deletedKeys []yellowpagesapi.PersonKey, err error)                               // MARKER: Purge
	Count(ctx context.Context, query yellowpagesapi.Query) (count int, err error)                                                            // MARKER: Count
	CreateREST(ctx context.Context, httpRequestBody *yellowpagesapi.Person) (objKey yellowpagesapi.PersonKey, httpStatusCode int, err error)  // MARKER: CreateREST
	StoreREST(ctx context.Context, key yellowpagesapi.PersonKey, httpRequestBody *yellowpagesapi.Person) (httpStatusCode int, err error)     // MARKER: StoreREST
	DeleteREST(ctx context.Context, key yellowpagesapi.PersonKey) (httpStatusCode int, err error)                                            // MARKER: DeleteREST
	LoadREST(ctx context.Context, key yellowpagesapi.PersonKey) (httpResponseBody *yellowpagesapi.Person, httpStatusCode int, err error)     // MARKER: LoadREST
	ListREST(ctx context.Context, q yellowpagesapi.Query) (httpResponseBody []*yellowpagesapi.Person, httpStatusCode int, err error)         // MARKER: ListREST
	WebUI(w http.ResponseWriter, r *http.Request) (err error)                                                                                // MARKER: WebUI
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
	svc.SetDescription(`Person persists persons in a SQL database.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(yellowpagesapi.Create.Method, yellowpagesapi.Create.Route, svc.doCreate)          // MARKER: Create
	svc.Subscribe(yellowpagesapi.Store.Method, yellowpagesapi.Store.Route, svc.doStore)              // MARKER: Store
	svc.Subscribe(yellowpagesapi.MustStore.Method, yellowpagesapi.MustStore.Route, svc.doMustStore)  // MARKER: MustStore
	svc.Subscribe(yellowpagesapi.Revise.Method, yellowpagesapi.Revise.Route, svc.doRevise)           // MARKER: Revise
	svc.Subscribe(yellowpagesapi.MustRevise.Method, yellowpagesapi.MustRevise.Route, svc.doMustRevise) // MARKER: MustRevise
	svc.Subscribe(yellowpagesapi.Delete.Method, yellowpagesapi.Delete.Route, svc.doDelete)           // MARKER: Delete
	svc.Subscribe(yellowpagesapi.MustDelete.Method, yellowpagesapi.MustDelete.Route, svc.doMustDelete) // MARKER: MustDelete
	svc.Subscribe(yellowpagesapi.List.Method, yellowpagesapi.List.Route, svc.doList)                 // MARKER: List
	svc.Subscribe(yellowpagesapi.Lookup.Method, yellowpagesapi.Lookup.Route, svc.doLookup)           // MARKER: Lookup
	svc.Subscribe(yellowpagesapi.MustLookup.Method, yellowpagesapi.MustLookup.Route, svc.doMustLookup) // MARKER: MustLookup
	svc.Subscribe(yellowpagesapi.Load.Method, yellowpagesapi.Load.Route, svc.doLoad)                 // MARKER: Load
	svc.Subscribe(yellowpagesapi.MustLoad.Method, yellowpagesapi.MustLoad.Route, svc.doMustLoad)     // MARKER: MustLoad
	svc.Subscribe(yellowpagesapi.BulkLoad.Method, yellowpagesapi.BulkLoad.Route, svc.doBulkLoad)     // MARKER: BulkLoad
	svc.Subscribe(yellowpagesapi.BulkDelete.Method, yellowpagesapi.BulkDelete.Route, svc.doBulkDelete) // MARKER: BulkDelete
	svc.Subscribe(yellowpagesapi.BulkCreate.Method, yellowpagesapi.BulkCreate.Route, svc.doBulkCreate) // MARKER: BulkCreate
	svc.Subscribe(yellowpagesapi.BulkStore.Method, yellowpagesapi.BulkStore.Route, svc.doBulkStore)  // MARKER: BulkStore
	svc.Subscribe(yellowpagesapi.BulkRevise.Method, yellowpagesapi.BulkRevise.Route, svc.doBulkRevise) // MARKER: BulkRevise
	svc.Subscribe(yellowpagesapi.Purge.Method, yellowpagesapi.Purge.Route, svc.doPurge)              // MARKER: Purge
	svc.Subscribe(yellowpagesapi.Count.Method, yellowpagesapi.Count.Route, svc.doCount)              // MARKER: Count
	svc.Subscribe(yellowpagesapi.CreateREST.Method, yellowpagesapi.CreateREST.Route, svc.doCreateREST) // MARKER: CreateREST
	svc.Subscribe(yellowpagesapi.StoreREST.Method, yellowpagesapi.StoreREST.Route, svc.doStoreREST) // MARKER: StoreREST
	svc.Subscribe(yellowpagesapi.DeleteREST.Method, yellowpagesapi.DeleteREST.Route, svc.doDeleteREST) // MARKER: DeleteREST
	svc.Subscribe(yellowpagesapi.LoadREST.Method, yellowpagesapi.LoadREST.Route, svc.doLoadREST)    // MARKER: LoadREST
	svc.Subscribe(yellowpagesapi.ListREST.Method, yellowpagesapi.ListREST.Route, svc.doListREST)    // MARKER: ListREST

	// HINT: Add web endpoints here
	svc.Subscribe("ANY", yellowpagesapi.RouteOfWebUI, svc.WebUI) // MARKER: WebUI

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: SQLDataSourceName
		"SQLDataSourceName",
		cfg.Description(`SQLDataSourceName is the connection string of the SQL database.`),
		cfg.Secret(),
	)

	// HINT: Add inbound event sinks here

	_ = marshalFunction
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
			Method:      yellowpagesapi.Create.Method,
			Route:       yellowpagesapi.Create.Route,
			Summary:     "Create(obj *Person) (objKey PersonKey)",
			Description: `Create creates a new person, returning its key.`,
			InputArgs:   yellowpagesapi.CreateIn{},
			OutputArgs:  yellowpagesapi.CreateOut{},
		},
		{ // MARKER: Store
			Type:        "function",
			Name:        "Store",
			Method:      yellowpagesapi.Store.Method,
			Route:       yellowpagesapi.Store.Route,
			Summary:     "Store(obj *Person) (stored bool)",
			Description: `Store updates the person.`,
			InputArgs:   yellowpagesapi.StoreIn{},
			OutputArgs:  yellowpagesapi.StoreOut{},
		},
		{ // MARKER: MustStore
			Type:        "function",
			Name:        "MustStore",
			Method:      yellowpagesapi.MustStore.Method,
			Route:       yellowpagesapi.MustStore.Route,
			Summary:     "MustStore(obj *Person)",
			Description: `MustStore updates the person, erroring if not found.`,
			InputArgs:   yellowpagesapi.MustStoreIn{},
			OutputArgs:  yellowpagesapi.MustStoreOut{},
		},
		{ // MARKER: Revise
			Type:        "function",
			Name:        "Revise",
			Method:      yellowpagesapi.Revise.Method,
			Route:       yellowpagesapi.Revise.Route,
			Summary:     "Revise(obj *Person) (revised bool)",
			Description: `Revise updates the person only if the revision matches.`,
			InputArgs:   yellowpagesapi.ReviseIn{},
			OutputArgs:  yellowpagesapi.ReviseOut{},
		},
		{ // MARKER: MustRevise
			Type:        "function",
			Name:        "MustRevise",
			Method:      yellowpagesapi.MustRevise.Method,
			Route:       yellowpagesapi.MustRevise.Route,
			Summary:     "MustRevise(obj *Person)",
			Description: `MustRevise updates the person only if the revision matches, erroring on conflict.`,
			InputArgs:   yellowpagesapi.MustReviseIn{},
			OutputArgs:  yellowpagesapi.MustReviseOut{},
		},
		{ // MARKER: Delete
			Type:        "function",
			Name:        "Delete",
			Method:      yellowpagesapi.Delete.Method,
			Route:       yellowpagesapi.Delete.Route,
			Summary:     "Delete(objKey PersonKey) (deleted bool)",
			Description: `Delete deletes the person.`,
			InputArgs:   yellowpagesapi.DeleteIn{},
			OutputArgs:  yellowpagesapi.DeleteOut{},
		},
		{ // MARKER: MustDelete
			Type:        "function",
			Name:        "MustDelete",
			Method:      yellowpagesapi.MustDelete.Method,
			Route:       yellowpagesapi.MustDelete.Route,
			Summary:     "MustDelete(objKey PersonKey)",
			Description: `MustDelete deletes the person, erroring if not found.`,
			InputArgs:   yellowpagesapi.MustDeleteIn{},
			OutputArgs:  yellowpagesapi.MustDeleteOut{},
		},
		{ // MARKER: List
			Type:        "function",
			Name:        "List",
			Method:      yellowpagesapi.List.Method,
			Route:       yellowpagesapi.List.Route,
			Summary:     "List(query Query) (objs []*Person, totalCount int)",
			Description: `List returns the persons matching the query, and the total count of matches regardless of the limit.`,
			InputArgs:   yellowpagesapi.ListIn{},
			OutputArgs:  yellowpagesapi.ListOut{},
		},
		{ // MARKER: Lookup
			Type:        "function",
			Name:        "Lookup",
			Method:      yellowpagesapi.Lookup.Method,
			Route:       yellowpagesapi.Lookup.Route,
			Summary:     "Lookup(query Query) (obj *Person, found bool)",
			Description: `Lookup returns the single person matching the query.`,
			InputArgs:   yellowpagesapi.LookupIn{},
			OutputArgs:  yellowpagesapi.LookupOut{},
		},
		{ // MARKER: MustLookup
			Type:        "function",
			Name:        "MustLookup",
			Method:      yellowpagesapi.MustLookup.Method,
			Route:       yellowpagesapi.MustLookup.Route,
			Summary:     "MustLookup(query Query) (obj *Person)",
			Description: `MustLookup returns the single person matching the query. It errors unless exactly one person matches the query.`,
			InputArgs:   yellowpagesapi.MustLookupIn{},
			OutputArgs:  yellowpagesapi.MustLookupOut{},
		},
		{ // MARKER: Load
			Type:        "function",
			Name:        "Load",
			Method:      yellowpagesapi.Load.Method,
			Route:       yellowpagesapi.Load.Route,
			Summary:     "Load(objKey PersonKey) (obj *Person, found bool)",
			Description: `Load returns the person associated with the key.`,
			InputArgs:   yellowpagesapi.LoadIn{},
			OutputArgs:  yellowpagesapi.LoadOut{},
		},
		{ // MARKER: MustLoad
			Type:        "function",
			Name:        "MustLoad",
			Method:      yellowpagesapi.MustLoad.Method,
			Route:       yellowpagesapi.MustLoad.Route,
			Summary:     "MustLoad(objKey PersonKey) (obj *Person)",
			Description: `MustLoad returns the person associated with the key, erroring if not found.`,
			InputArgs:   yellowpagesapi.MustLoadIn{},
			OutputArgs:  yellowpagesapi.MustLoadOut{},
		},
		{ // MARKER: BulkLoad
			Type:        "function",
			Name:        "BulkLoad",
			Method:      yellowpagesapi.BulkLoad.Method,
			Route:       yellowpagesapi.BulkLoad.Route,
			Summary:     "BulkLoad(objKeys []PersonKey) (objs []*Person)",
			Description: `BulkLoad returns the persons matching the keys.`,
			InputArgs:   yellowpagesapi.BulkLoadIn{},
			OutputArgs:  yellowpagesapi.BulkLoadOut{},
		},
		{ // MARKER: BulkDelete
			Type:        "function",
			Name:        "BulkDelete",
			Method:      yellowpagesapi.BulkDelete.Method,
			Route:       yellowpagesapi.BulkDelete.Route,
			Summary:     "BulkDelete(objKeys []PersonKey) (deletedKeys []PersonKey)",
			Description: `BulkDelete deletes the persons matching the keys, returning the keys of the deleted persons.`,
			InputArgs:   yellowpagesapi.BulkDeleteIn{},
			OutputArgs:  yellowpagesapi.BulkDeleteOut{},
		},
		{ // MARKER: BulkCreate
			Type:        "function",
			Name:        "BulkCreate",
			Method:      yellowpagesapi.BulkCreate.Method,
			Route:       yellowpagesapi.BulkCreate.Route,
			Summary:     "BulkCreate(objs []*Person) (objKeys []PersonKey)",
			Description: `BulkCreate creates multiple persons, returning their keys.`,
			InputArgs:   yellowpagesapi.BulkCreateIn{},
			OutputArgs:  yellowpagesapi.BulkCreateOut{},
		},
		{ // MARKER: BulkStore
			Type:        "function",
			Name:        "BulkStore",
			Method:      yellowpagesapi.BulkStore.Method,
			Route:       yellowpagesapi.BulkStore.Route,
			Summary:     "BulkStore(objs []*Person) (storedKeys []PersonKey)",
			Description: `BulkStore updates multiple persons, returning the keys of the stored persons.`,
			InputArgs:   yellowpagesapi.BulkStoreIn{},
			OutputArgs:  yellowpagesapi.BulkStoreOut{},
		},
		{ // MARKER: BulkRevise
			Type:        "function",
			Name:        "BulkRevise",
			Method:      yellowpagesapi.BulkRevise.Method,
			Route:       yellowpagesapi.BulkRevise.Route,
			Summary:     "BulkRevise(objs []*Person) (revisedKeys []PersonKey)",
			Description: `BulkRevise updates multiple persons only if the revisions match, returning the keys of the revised persons.`,
			InputArgs:   yellowpagesapi.BulkReviseIn{},
			OutputArgs:  yellowpagesapi.BulkReviseOut{},
		},
		{ // MARKER: Purge
			Type:        "function",
			Name:        "Purge",
			Method:      yellowpagesapi.Purge.Method,
			Route:       yellowpagesapi.Purge.Route,
			Summary:     "Purge(query Query) (deletedKeys []PersonKey)",
			Description: `Purge deletes all persons matching the query, returning the keys of the deleted persons.`,
			InputArgs:   yellowpagesapi.PurgeIn{},
			OutputArgs:  yellowpagesapi.PurgeOut{},
		},
		{ // MARKER: Count
			Type:        "function",
			Name:        "Count",
			Method:      yellowpagesapi.Count.Method,
			Route:       yellowpagesapi.Count.Route,
			Summary:     "Count(query Query) (count int)",
			Description: `Count returns the number of persons matching the query.`,
			InputArgs:   yellowpagesapi.CountIn{},
			OutputArgs:  yellowpagesapi.CountOut{},
		},
		{ // MARKER: CreateREST
			Type:        "function",
			Name:        "CreateREST",
			Method:      yellowpagesapi.CreateREST.Method,
			Route:       yellowpagesapi.CreateREST.Route,
			Summary:     "CreateREST(*Person) (objKey PersonKey)",
			Description: `CreateREST creates a new person via REST, returning its key.`,
			InputArgs:   yellowpagesapi.CreateRESTIn{},
			OutputArgs:  yellowpagesapi.CreateRESTOut{},
		},
		{ // MARKER: StoreREST
			Type:        "function",
			Name:        "StoreREST",
			Method:      yellowpagesapi.StoreREST.Method,
			Route:       yellowpagesapi.StoreREST.Route,
			Summary:     "StoreREST(key PersonKey, *Person)",
			Description: `StoreREST updates an existing person via REST.`,
			InputArgs:   yellowpagesapi.StoreRESTIn{},
			OutputArgs:  yellowpagesapi.StoreRESTOut{},
		},
		{ // MARKER: DeleteREST
			Type:        "function",
			Name:        "DeleteREST",
			Method:      yellowpagesapi.DeleteREST.Method,
			Route:       yellowpagesapi.DeleteREST.Route,
			Summary:     "DeleteREST(key PersonKey)",
			Description: `DeleteREST deletes an existing person via REST.`,
			InputArgs:   yellowpagesapi.DeleteRESTIn{},
			OutputArgs:  yellowpagesapi.DeleteRESTOut{},
		},
		{ // MARKER: LoadREST
			Type:        "function",
			Name:        "LoadREST",
			Method:      yellowpagesapi.LoadREST.Method,
			Route:       yellowpagesapi.LoadREST.Route,
			Summary:     "LoadREST(key PersonKey) (httpResponseBody *Person)",
			Description: `LoadREST loads a person by key via REST.`,
			InputArgs:   yellowpagesapi.LoadRESTIn{},
			OutputArgs:  yellowpagesapi.LoadRESTOut{},
		},
		{ // MARKER: ListREST
			Type:        "function",
			Name:        "ListREST",
			Method:      yellowpagesapi.ListREST.Method,
			Route:       yellowpagesapi.ListREST.Route,
			Summary:     "ListREST(q Query) ([]*Person)",
			Description: `ListREST lists persons matching the query via REST.`,
			InputArgs:   yellowpagesapi.ListRESTIn{},
			OutputArgs:  yellowpagesapi.ListRESTOut{},
		},
		{ // MARKER: WebUI
			Type:        "web",
			Name:        "WebUI",
			Method:      "ANY",
			Route:       yellowpagesapi.RouteOfWebUI,
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
SQLDataSourceName is the connection string of the SQL database.
*/
func (svc *Intermediate) SQLDataSourceName() (value string) { // MARKER: SQLDataSourceName
	return svc.Config("SQLDataSourceName")
}

/*
SetSQLDataSourceName sets the value of the configuration property.
*/
func (svc *Intermediate) SetSQLDataSourceName(value string) (err error) { // MARKER: SQLDataSourceName
	return svc.SetConfig("SQLDataSourceName", value)
}

// --- Marshaler Functions ---

// doCreate handles marshaling for Create.
func (svc *Intermediate) doCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Create
	var in yellowpagesapi.CreateIn
	var out yellowpagesapi.CreateOut
	err = marshalFunction(w, r, yellowpagesapi.Create.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKey, err = svc.Create(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doStore handles marshaling for Store.
func (svc *Intermediate) doStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Store
	var in yellowpagesapi.StoreIn
	var out yellowpagesapi.StoreOut
	err = marshalFunction(w, r, yellowpagesapi.Store.Route, &in, &out, func(_ any, _ any) error {
		out.Stored, err = svc.Store(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doMustStore handles marshaling for MustStore.
func (svc *Intermediate) doMustStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustStore
	var in yellowpagesapi.MustStoreIn
	var out yellowpagesapi.MustStoreOut
	err = marshalFunction(w, r, yellowpagesapi.MustStore.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustStore(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doRevise handles marshaling for Revise.
func (svc *Intermediate) doRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Revise
	var in yellowpagesapi.ReviseIn
	var out yellowpagesapi.ReviseOut
	err = marshalFunction(w, r, yellowpagesapi.Revise.Route, &in, &out, func(_ any, _ any) error {
		out.Revised, err = svc.Revise(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doMustRevise handles marshaling for MustRevise.
func (svc *Intermediate) doMustRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustRevise
	var in yellowpagesapi.MustReviseIn
	var out yellowpagesapi.MustReviseOut
	err = marshalFunction(w, r, yellowpagesapi.MustRevise.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustRevise(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doDelete handles marshaling for Delete.
func (svc *Intermediate) doDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Delete
	var in yellowpagesapi.DeleteIn
	var out yellowpagesapi.DeleteOut
	err = marshalFunction(w, r, yellowpagesapi.Delete.Route, &in, &out, func(_ any, _ any) error {
		out.Deleted, err = svc.Delete(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doMustDelete handles marshaling for MustDelete.
func (svc *Intermediate) doMustDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustDelete
	var in yellowpagesapi.MustDeleteIn
	var out yellowpagesapi.MustDeleteOut
	err = marshalFunction(w, r, yellowpagesapi.MustDelete.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustDelete(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doList handles marshaling for List.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	var in yellowpagesapi.ListIn
	var out yellowpagesapi.ListOut
	err = marshalFunction(w, r, yellowpagesapi.List.Route, &in, &out, func(_ any, _ any) error {
		out.Objs, out.TotalCount, err = svc.List(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doLookup handles marshaling for Lookup.
func (svc *Intermediate) doLookup(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Lookup
	var in yellowpagesapi.LookupIn
	var out yellowpagesapi.LookupOut
	err = marshalFunction(w, r, yellowpagesapi.Lookup.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, out.Found, err = svc.Lookup(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doMustLookup handles marshaling for MustLookup.
func (svc *Intermediate) doMustLookup(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustLookup
	var in yellowpagesapi.MustLookupIn
	var out yellowpagesapi.MustLookupOut
	err = marshalFunction(w, r, yellowpagesapi.MustLookup.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, err = svc.MustLookup(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doLoad handles marshaling for Load.
func (svc *Intermediate) doLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Load
	var in yellowpagesapi.LoadIn
	var out yellowpagesapi.LoadOut
	err = marshalFunction(w, r, yellowpagesapi.Load.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, out.Found, err = svc.Load(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doMustLoad handles marshaling for MustLoad.
func (svc *Intermediate) doMustLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustLoad
	var in yellowpagesapi.MustLoadIn
	var out yellowpagesapi.MustLoadOut
	err = marshalFunction(w, r, yellowpagesapi.MustLoad.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, err = svc.MustLoad(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doBulkLoad handles marshaling for BulkLoad.
func (svc *Intermediate) doBulkLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkLoad
	var in yellowpagesapi.BulkLoadIn
	var out yellowpagesapi.BulkLoadOut
	err = marshalFunction(w, r, yellowpagesapi.BulkLoad.Route, &in, &out, func(_ any, _ any) error {
		out.Objs, err = svc.BulkLoad(r.Context(), in.ObjKeys)
		return err
	})
	return err // No trace
}

// doBulkDelete handles marshaling for BulkDelete.
func (svc *Intermediate) doBulkDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkDelete
	var in yellowpagesapi.BulkDeleteIn
	var out yellowpagesapi.BulkDeleteOut
	err = marshalFunction(w, r, yellowpagesapi.BulkDelete.Route, &in, &out, func(_ any, _ any) error {
		out.DeletedKeys, err = svc.BulkDelete(r.Context(), in.ObjKeys)
		return err
	})
	return err // No trace
}

// doBulkCreate handles marshaling for BulkCreate.
func (svc *Intermediate) doBulkCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkCreate
	var in yellowpagesapi.BulkCreateIn
	var out yellowpagesapi.BulkCreateOut
	err = marshalFunction(w, r, yellowpagesapi.BulkCreate.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKeys, err = svc.BulkCreate(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doBulkStore handles marshaling for BulkStore.
func (svc *Intermediate) doBulkStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkStore
	var in yellowpagesapi.BulkStoreIn
	var out yellowpagesapi.BulkStoreOut
	err = marshalFunction(w, r, yellowpagesapi.BulkStore.Route, &in, &out, func(_ any, _ any) error {
		out.StoredKeys, err = svc.BulkStore(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doBulkRevise handles marshaling for BulkRevise.
func (svc *Intermediate) doBulkRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkRevise
	var in yellowpagesapi.BulkReviseIn
	var out yellowpagesapi.BulkReviseOut
	err = marshalFunction(w, r, yellowpagesapi.BulkRevise.Route, &in, &out, func(_ any, _ any) error {
		out.RevisedKeys, err = svc.BulkRevise(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doPurge handles marshaling for Purge.
func (svc *Intermediate) doPurge(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Purge
	var in yellowpagesapi.PurgeIn
	var out yellowpagesapi.PurgeOut
	err = marshalFunction(w, r, yellowpagesapi.Purge.Route, &in, &out, func(_ any, _ any) error {
		out.DeletedKeys, err = svc.Purge(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doCount handles marshaling for Count.
func (svc *Intermediate) doCount(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Count
	var in yellowpagesapi.CountIn
	var out yellowpagesapi.CountOut
	err = marshalFunction(w, r, yellowpagesapi.Count.Route, &in, &out, func(_ any, _ any) error {
		out.Count, err = svc.Count(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doCreateREST handles marshaling for CreateREST.
func (svc *Intermediate) doCreateREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CreateREST
	var in yellowpagesapi.CreateRESTIn
	var out yellowpagesapi.CreateRESTOut
	err = marshalFunction(w, r, yellowpagesapi.CreateREST.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKey, out.HTTPStatusCode, err = svc.CreateREST(r.Context(), in.HTTPRequestBody)
		return err
	})
	return err // No trace
}

// doStoreREST handles marshaling for StoreREST.
func (svc *Intermediate) doStoreREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: StoreREST
	var in yellowpagesapi.StoreRESTIn
	var out yellowpagesapi.StoreRESTOut
	err = marshalFunction(w, r, yellowpagesapi.StoreREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPStatusCode, err = svc.StoreREST(r.Context(), in.Key, in.HTTPRequestBody)
		return err
	})
	return err // No trace
}

// doDeleteREST handles marshaling for DeleteREST.
func (svc *Intermediate) doDeleteREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DeleteREST
	var in yellowpagesapi.DeleteRESTIn
	var out yellowpagesapi.DeleteRESTOut
	err = marshalFunction(w, r, yellowpagesapi.DeleteREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPStatusCode, err = svc.DeleteREST(r.Context(), in.Key)
		return err
	})
	return err // No trace
}

// doLoadREST handles marshaling for LoadREST.
func (svc *Intermediate) doLoadREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: LoadREST
	var in yellowpagesapi.LoadRESTIn
	var out yellowpagesapi.LoadRESTOut
	err = marshalFunction(w, r, yellowpagesapi.LoadREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.LoadREST(r.Context(), in.Key)
		return err
	})
	return err // No trace
}

// doListREST handles marshaling for ListREST.
func (svc *Intermediate) doListREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ListREST
	var in yellowpagesapi.ListRESTIn
	var out yellowpagesapi.ListRESTOut
	err = marshalFunction(w, r, yellowpagesapi.ListREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.ListREST(r.Context(), in.Q)
		return err
	})
	return err // No trace
}

// marshalFunction handled marshaling for functional endpoints.
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

/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package intermediate serves as the foundation of the directory.example microservice.

The directory microservice stores personal records in a SQL database.
*/
package intermediate

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/log"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/shardedsql"
	"github.com/microbus-io/fabric/sub"

	"gopkg.in/yaml.v3"

	"github.com/microbus-io/fabric/examples/directory/resources"
	"github.com/microbus-io/fabric/examples/directory/directoryapi"
)

var (
	_ context.Context
	_ *embed.FS
	_ *json.Decoder
	_ fmt.Stringer
	_ *http.Request
	_ filepath.WalkFunc
	_ strconv.NumError
	_ strings.Reader
	_ time.Duration
	_ cfg.Option
	_ *errors.TracedError
	_ frame.Frame
	_ *httpx.ResponseRecorder
	_ *log.Field
	_ *openapi.Service
	_ *shardedsql.DB
	_ sub.Option
	_ yaml.Encoder
	_ directoryapi.Client
)

// ToDo defines the interface that the microservice must implement.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, person *directoryapi.Person) (created *directoryapi.Person, err error)
	Load(ctx context.Context, key directoryapi.PersonKey) (person *directoryapi.Person, ok bool, err error)
	Delete(ctx context.Context, key directoryapi.PersonKey) (ok bool, err error)
	Update(ctx context.Context, person *directoryapi.Person) (updated *directoryapi.Person, ok bool, err error)
	LoadByEmail(ctx context.Context, email string) (person *directoryapi.Person, ok bool, err error)
	List(ctx context.Context) (keys []directoryapi.PersonKey, err error)
}

// Intermediate extends and customizes the generic base connector.
// Code generated microservices then extend the intermediate.
type Intermediate struct {
	*connector.Connector
	impl ToDo
	dbMaria *shardedsql.DB
}

// NewService creates a new intermediate service.
func NewService(impl ToDo, version int) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New("directory.example"),
		impl: impl,
	}
	svc.SetVersion(version)
	svc.SetDescription(`The directory microservice stores personal records in a SQL database.`)

	// SQL databases
	svc.SetOnStartup(svc.dbMariaOnStartup)
	svc.SetOnShutdown(svc.dbMariaOnShutdown)
	svc.DefineConfig(
		"Maria",
		cfg.Description("Maria is the connection string to the sharded SQL database."),
		cfg.Secret(),
	)
	svc.SetOnConfigChanged(svc.dbMariaOnConfigChanged)

	// Lifecycle
	svc.SetOnStartup(svc.impl.OnStartup)
	svc.SetOnShutdown(svc.impl.OnShutdown)
	
	// OpenAPI
	svc.Subscribe(`:443/openapi.json`, svc.doOpenAPI)	

	// Functions
	svc.Subscribe(`:443/create`, svc.doCreate)
	svc.Subscribe(`:443/load`, svc.doLoad)
	svc.Subscribe(`:443/delete`, svc.doDelete)
	svc.Subscribe(`:443/update`, svc.doUpdate)
	svc.Subscribe(`:443/load-by-email`, svc.doLoadByEmail)
	svc.Subscribe(`:443/list`, svc.doList)

	// Resources file system
	svc.SetResFS(resources.FS)

	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) error {
	oapiSvc := openapi.Service{
		ServiceName: svc.HostName(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `Create`,
			Path:        `:443/create`,
			Summary:     `Create(person *Person) (created *Person)`,
			Description: `Create registers the person in the directory.`,
			InputArgs: struct {
				Xperson *directoryapi.Person `json:"person"`
			}{},
			OutputArgs: struct {
				Xcreated *directoryapi.Person `json:"created"`
			}{},
		})
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `Load`,
			Path:        `:443/load`,
			Summary:     `Load(key PersonKey) (person *Person, ok bool)`,
			Description: `Load looks up a person in the directory.`,
			InputArgs: struct {
				Xkey directoryapi.PersonKey `json:"key"`
			}{},
			OutputArgs: struct {
				Xperson *directoryapi.Person `json:"person"`
				Xok bool `json:"ok"`
			}{},
		})
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `Delete`,
			Path:        `:443/delete`,
			Summary:     `Delete(key PersonKey) (ok bool)`,
			Description: `Delete removes a person from the directory.`,
			InputArgs: struct {
				Xkey directoryapi.PersonKey `json:"key"`
			}{},
			OutputArgs: struct {
				Xok bool `json:"ok"`
			}{},
		})
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `Update`,
			Path:        `:443/update`,
			Summary:     `Update(person *Person) (updated *Person, ok bool)`,
			Description: `Update updates the person's data in the directory.`,
			InputArgs: struct {
				Xperson *directoryapi.Person `json:"person"`
			}{},
			OutputArgs: struct {
				Xupdated *directoryapi.Person `json:"updated"`
				Xok bool `json:"ok"`
			}{},
		})
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `LoadByEmail`,
			Path:        `:443/load-by-email`,
			Summary:     `LoadByEmail(email string) (person *Person, ok bool)`,
			Description: `LoadByEmail looks up a person in the directory by their email.`,
			InputArgs: struct {
				Xemail string `json:"email"`
			}{},
			OutputArgs: struct {
				Xperson *directoryapi.Person `json:"person"`
				Xok bool `json:"ok"`
			}{},
		})
	}
	if r.URL.Port() == "443" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `function`,
			Name:        `List`,
			Path:        `:443/list`,
			Summary:     `List() (keys []PersonKey)`,
			Description: `List returns the keys of all the persons in the directory.`,
			InputArgs: struct {
			}{},
			OutputArgs: struct {
				Xkeys []directoryapi.PersonKey `json:"keys"`
			}{},
		})
	}

	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	b, err := json.MarshalIndent(&oapiSvc, "", "    ")
	if err != nil {
		return errors.Trace(err)
	}
	_, err = w.Write(b)
	return errors.Trace(err)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

// dbMariaOnStartup opens the connection to the Maria database and runs schema migrations.
func (svc *Intermediate) dbMariaOnStartup(ctx context.Context) (err error) {
	if svc.dbMaria != nil {
		svc.dbMariaOnShutdown(ctx)
	}
	dataSource := svc.Maria()
	if dataSource != "" {
		svc.dbMaria, err = shardedsql.Open(ctx, "mariadb", dataSource)
		if err != nil {
			return errors.Trace(err)
		}
		svc.LogInfo(ctx, "Opened database", log.String("db", "Maria"))

		migrations := shardedsql.NewStatementSequence(svc.HostName() + " Maria")
		scripts, err := svc.ReadResDir("maria")
		if err != nil {
			return errors.Trace(err)
		}
		for _, script := range scripts {
			if script.IsDir() || filepath.Ext(script.Name())!=".sql" {
				continue
			}
			dot := strings.Index(script.Name(), ".")
			number, err := strconv.Atoi(script.Name()[:dot])
			if err != nil {
				continue
			}
			statement, err := svc.ReadResFile(filepath.Join("maria", script.Name()))
			if err != nil {
				return errors.Trace(err)
			}
			migrations.Insert(number, string(statement))
		}
		err = svc.dbMaria.MigrateSchema(ctx, migrations)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// dbMariaOnStartup closes the connection to the Maria database.
func (svc *Intermediate) dbMariaOnShutdown(ctx context.Context) (err error) {
	if svc.dbMaria != nil {
		svc.dbMaria.Close()
		svc.dbMaria = nil
		svc.LogInfo(ctx, "Closed database", log.String("db", "Maria"))
	}
	return nil
}

// dbMariaOnConfigChanged reconnects to the Maria database when the data source name changes.
func (svc *Intermediate) dbMariaOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	if changed("Maria") {
		err = svc.dbMariaOnStartup(ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Maria is the data source name to the sharded SQL database.
func (svc *Intermediate) Maria() (dsn string) {
	return svc.Config("Maria")
}

// MariaDatabase is the sharded SQL database.
func (svc *Intermediate) MariaDatabase() *shardedsql.DB {
	return svc.dbMaria
}

// Maria initializes the Maria config property of the microservice.
func Maria(dsn string) (func(connector.Service) error) {
	return func(svc connector.Service) error {
		return svc.SetConfig("Maria", dsn)
	}
}

// doCreate handles marshaling for the Create function.
func (svc *Intermediate) doCreate(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.CreateIn
	var o directoryapi.CreateOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Created, err = svc.impl.Create(
		r.Context(),
		i.Person,
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doLoad handles marshaling for the Load function.
func (svc *Intermediate) doLoad(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.LoadIn
	var o directoryapi.LoadOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Person, o.Ok, err = svc.impl.Load(
		r.Context(),
		i.Key,
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doDelete handles marshaling for the Delete function.
func (svc *Intermediate) doDelete(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.DeleteIn
	var o directoryapi.DeleteOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Ok, err = svc.impl.Delete(
		r.Context(),
		i.Key,
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doUpdate handles marshaling for the Update function.
func (svc *Intermediate) doUpdate(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.UpdateIn
	var o directoryapi.UpdateOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Updated, o.Ok, err = svc.impl.Update(
		r.Context(),
		i.Person,
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doLoadByEmail handles marshaling for the LoadByEmail function.
func (svc *Intermediate) doLoadByEmail(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.LoadByEmailIn
	var o directoryapi.LoadByEmailOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Person, o.Ok, err = svc.impl.LoadByEmail(
		r.Context(),
		i.Email,
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doList handles marshaling for the List function.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) error {
	var i directoryapi.ListIn
	var o directoryapi.ListOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Keys, err = svc.impl.List(
		r.Context(),
	)
	if err != nil {
		return err // No trace
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(o)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

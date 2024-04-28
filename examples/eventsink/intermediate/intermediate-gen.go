/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package intermediate serves as the foundation of the eventsink.example microservice.

The event sink microservice handles events that are fired by the event source microservice.
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

	"github.com/microbus-io/fabric/examples/eventsink/resources"
	"github.com/microbus-io/fabric/examples/eventsink/eventsinkapi"
	
	eventsourceapi1 "github.com/microbus-io/fabric/examples/eventsource/eventsourceapi"
	eventsourceapi2 "github.com/microbus-io/fabric/examples/eventsource/eventsourceapi"
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
	_ eventsinkapi.Client
)

// ToDo defines the interface that the microservice must implement.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Registered(ctx context.Context) (emails []string, err error)
	OnAllowRegister(ctx context.Context, email string) (allow bool, err error)
	OnRegistered(ctx context.Context, email string) (err error)
}

// Intermediate extends and customizes the generic base connector.
// Code generated microservices then extend the intermediate.
type Intermediate struct {
	*connector.Connector
	impl ToDo
}

// NewService creates a new intermediate service.
func NewService(impl ToDo, version int) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New("eventsink.example"),
		impl: impl,
	}
	svc.SetVersion(version)
	svc.SetDescription(`The event sink microservice handles events that are fired by the event source microservice.`)

	// Lifecycle
	svc.SetOnStartup(svc.impl.OnStartup)
	svc.SetOnShutdown(svc.impl.OnShutdown)
	
	// OpenAPI
	svc.Subscribe(`:443/openapi.json`, svc.doOpenAPI)	

	// Functions
	svc.Subscribe(`:443/registered`, svc.doRegistered)
	
	// Sinks
	eventsourceapi1.NewHook(svc).OnAllowRegister(svc.impl.OnAllowRegister)
	eventsourceapi2.NewHook(svc).OnRegistered(svc.impl.OnRegistered)

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
			Name:        `Registered`,
			Path:        `:443/registered`,
			Summary:     `Registered() (emails []string)`,
			Description: `Registered returns the list of registered users.`,
			InputArgs: struct {
			}{},
			OutputArgs: struct {
				Xemails []string `json:"emails"`
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

// doRegistered handles marshaling for the Registered function.
func (svc *Intermediate) doRegistered(w http.ResponseWriter, r *http.Request) error {
	var i eventsinkapi.RegisteredIn
	var o eventsinkapi.RegisteredOut
	err := httpx.ParseRequestData(r, &i)
	if err != nil {
		return errors.Trace(err)
	}
	o.Emails, err = svc.impl.Registered(
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

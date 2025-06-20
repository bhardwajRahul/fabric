/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

// Code generated by Microbus. DO NOT EDIT.

/*
Package intermediate serves as the foundation of the hello.example microservice.

The Hello microservice demonstrates the various capabilities of a microservice.
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
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"gopkg.in/yaml.v3"

	"github.com/microbus-io/fabric/examples/hello/resources"
	"github.com/microbus-io/fabric/examples/hello/helloapi"
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
	_ *openapi.Service
	_ service.Service
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ yaml.Encoder
	_ helloapi.Client
)

// ToDo defines the interface that the microservice must implement.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Hello(w http.ResponseWriter, r *http.Request) (err error)
	Echo(w http.ResponseWriter, r *http.Request) (err error)
	Ping(w http.ResponseWriter, r *http.Request) (err error)
	Calculator(w http.ResponseWriter, r *http.Request) (err error)
	BusPNG(w http.ResponseWriter, r *http.Request) (err error)
	Localization(w http.ResponseWriter, r *http.Request) (err error)
	Root(w http.ResponseWriter, r *http.Request) (err error)
	TickTock(ctx context.Context) (err error)
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
		Connector: connector.New("hello.example"),
		impl: impl,
	}
	svc.SetVersion(version)
	svc.SetDescription(`The Hello microservice demonstrates the various capabilities of a microservice.`)
	
	// Lifecycle
	svc.SetOnStartup(svc.impl.OnStartup)
	svc.SetOnShutdown(svc.impl.OnShutdown)

	// Configs
	svc.SetOnConfigChanged(svc.doOnConfigChanged)
	svc.DefineConfig(
		"Greeting",
		cfg.Description(`Greeting to use.`),
		cfg.DefaultValue(`Hello`),
	)
	svc.DefineConfig(
		"Repeat",
		cfg.Description(`Repeat indicates how many times to display the greeting.`),
		cfg.Validation(`int [0,100]`),
		cfg.DefaultValue(`1`),
	)

	// OpenAPI
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)

	// Webs
	svc.Subscribe(`ANY`, `:443/hello`, svc.impl.Hello)
	svc.Subscribe(`ANY`, `:443/echo`, svc.impl.Echo)
	svc.Subscribe(`ANY`, `:443/ping`, svc.impl.Ping)
	svc.Subscribe(`ANY`, `:443/calculator`, svc.impl.Calculator)
	svc.Subscribe(`GET`, `:443/bus.png`, svc.impl.BusPNG)
	svc.Subscribe(`ANY`, `:443/localization`, svc.impl.Localization)
	svc.Subscribe(`ANY`, `//root`, svc.impl.Root)

	// Tickers
	intervalTickTock, _ := time.ParseDuration("10s")
	svc.StartTicker("TickTock", intervalTickTock, svc.impl.TickTock)

	// Resources file system
	svc.SetResFS(resources.FS)

	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) error {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Hello`,
			Method:      `ANY`,
			Path:        `:443/hello`,
			Summary:     `Hello()`,
			Description: `Hello prints a greeting.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Echo`,
			Method:      `ANY`,
			Path:        `:443/echo`,
			Summary:     `Echo()`,
			Description: `Echo back the incoming request in wire format.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Ping`,
			Method:      `ANY`,
			Path:        `:443/ping`,
			Summary:     `Ping()`,
			Description: `Ping all microservices and list them.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Calculator`,
			Method:      `ANY`,
			Path:        `:443/calculator`,
			Summary:     `Calculator()`,
			Description: `Calculator renders a UI for a calculator.
The calculation operation is delegated to another microservice in order to demonstrate
a call from one microservice to another.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `BusPNG`,
			Method:      `GET`,
			Path:        `:443/bus.png`,
			Summary:     `BusPNG()`,
			Description: `BusPNG serves an image from the embedded resources.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Localization`,
			Method:      `ANY`,
			Path:        `:443/localization`,
			Summary:     `Localization()`,
			Description: `Localization prints hello in the language best matching the request's Accept-Language header.`,
		})
	}
	if r.URL.Port() == "443" || "443" == "0" {
		oapiSvc.Endpoints = append(oapiSvc.Endpoints, &openapi.Endpoint{
			Type:        `web`,
			Name:        `Root`,
			Method:      `ANY`,
			Path:        `//root`,
			Summary:     `Root()`,
			Description: `Root is the top-most root page.`,
		})
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
	err := encoder.Encode(&oapiSvc)
	return errors.Trace(err)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}

/*
Greeting to use.
*/
func (svc *Intermediate) Greeting() (greeting string) {
	_val := svc.Config("Greeting")
	return _val
}

/*
SetGreeting sets the value of the configuration property.
This action is restricted to the TESTING deployment in which the fetching of values from the configurator is disabled.

Greeting to use.
*/
func (svc *Intermediate) SetGreeting(greeting string) error {
	return svc.SetConfig("Greeting", utils.AnyToString(greeting))
}

/*
Repeat indicates how many times to display the greeting.
*/
func (svc *Intermediate) Repeat() (count int) {
	_val := svc.Config("Repeat")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetRepeat sets the value of the configuration property.
This action is restricted to the TESTING deployment in which the fetching of values from the configurator is disabled.

Repeat indicates how many times to display the greeting.
*/
func (svc *Intermediate) SetRepeat(count int) error {
	return svc.SetConfig("Repeat", utils.AnyToString(count))
}

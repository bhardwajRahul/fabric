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

package control

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

	"github.com/microbus-io/fabric/coreservices/control/controlapi"
	"github.com/microbus-io/fabric/coreservices/control/resources"
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
	_ controlapi.Client
)

const (
	Hostname = controlapi.Hostname
	Version  = 235
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Ping(ctx context.Context) (pong int, err error)             // MARKER: Ping
	ConfigRefresh(ctx context.Context) (err error)              // MARKER: ConfigRefresh
	Trace(ctx context.Context, id string) (err error)           // MARKER: Trace
	Metrics(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Metrics
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
	svc.SetDescription(`This microservice is created for the sake of generating the client API for the :888 control subscriptions.
The microservice itself does nothing and should not be included in applications.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functions
	svc.Subscribe(controlapi.Ping.Method, controlapi.Ping.Route, svc.doPing, sub.NoQueue())                   // MARKER: Ping
	svc.Subscribe(controlapi.ConfigRefresh.Method, controlapi.ConfigRefresh.Route, svc.doConfigRefresh, sub.NoQueue()) // MARKER: ConfigRefresh
	svc.Subscribe(controlapi.Trace.Method, controlapi.Trace.Route, svc.doTrace, sub.NoQueue())                 // MARKER: Trace

	// Web endpoints
	svc.Subscribe(controlapi.Metrics.Method, controlapi.Metrics.Route, svc.Metrics, sub.NoQueue()) // MARKER: Metrics

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

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
		{ // MARKER: Ping
			Type:        "function",
			Name:        "Ping",
			Method:      controlapi.Ping.Method,
			Route:       controlapi.Ping.Route,
			Summary:     "Ping() (pong int)",
			Description: `Ping responds with a pong.`,
			InputArgs:   controlapi.PingIn{},
			OutputArgs:  controlapi.PingOut{},
		},
		{ // MARKER: ConfigRefresh
			Type:        "function",
			Name:        "ConfigRefresh",
			Method:      controlapi.ConfigRefresh.Method,
			Route:       controlapi.ConfigRefresh.Route,
			Summary:     "ConfigRefresh()",
			Description: `ConfigRefresh pulls the latest config values.`,
			InputArgs:   controlapi.ConfigRefreshIn{},
			OutputArgs:  controlapi.ConfigRefreshOut{},
		},
		{ // MARKER: Trace
			Type:        "function",
			Name:        "Trace",
			Method:      controlapi.Trace.Method,
			Route:       controlapi.Trace.Route,
			Summary:     "Trace(id string)",
			Description: `Trace forces exporting a tracing span.`,
			InputArgs:   controlapi.TraceIn{},
			OutputArgs:  controlapi.TraceOut{},
		},
		{ // MARKER: Metrics
			Type:        "web",
			Name:        "Metrics",
			Method:      controlapi.Metrics.Method,
			Route:       controlapi.Metrics.Route,
			Summary:     "Metrics()",
			Description: `Metrics returns Prometheus metrics.`,
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

// doPing handles marshaling for the Ping function.
func (svc *Intermediate) doPing(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Ping
	var in controlapi.PingIn
	var out controlapi.PingOut
	err = marshalFunction(w, r, controlapi.Ping.Route, &in, &out, func(_ any, _ any) error {
		out.Pong, err = svc.Ping(r.Context())
		return err
	})
	return err // No trace
}

// doConfigRefresh handles marshaling for the ConfigRefresh function.
func (svc *Intermediate) doConfigRefresh(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ConfigRefresh
	var in controlapi.ConfigRefreshIn
	var out controlapi.ConfigRefreshOut
	err = marshalFunction(w, r, controlapi.ConfigRefresh.Route, &in, &out, func(_ any, _ any) error {
		err = svc.ConfigRefresh(r.Context())
		return err
	})
	return err // No trace
}

// doTrace handles marshaling for the Trace function.
func (svc *Intermediate) doTrace(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Trace
	var in controlapi.TraceIn
	var out controlapi.TraceOut
	err = marshalFunction(w, r, controlapi.Trace.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Trace(r.Context(), in.ID)
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

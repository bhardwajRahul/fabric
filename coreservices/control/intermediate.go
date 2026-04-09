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
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

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
	_ *workflow.Flow
)

const (
	Hostname = controlapi.Hostname
	Version  = 237
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Ping(ctx context.Context) (pong int, err error)                                                     // MARKER: Ping
	ConfigRefresh(ctx context.Context) (err error)                                                      // MARKER: ConfigRefresh
	Trace(ctx context.Context, id string) (err error)                                                   // MARKER: Trace
	Metrics(w http.ResponseWriter, r *http.Request) (err error)                                         // MARKER: Metrics
	OpenAPI(ctx context.Context) (httpResponseBody *controlapi.Document, httpStatusCode int, err error) // MARKER: OpenAPI
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
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functions
	svc.Subscribe( // MARKER: Ping
		"Ping", svc.doPing,
		sub.At(controlapi.Ping.Method, controlapi.Ping.Route),
		sub.Description(`Ping responds to the message with a pong.`),
		sub.Function(controlapi.PingIn{}, controlapi.PingOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: ConfigRefresh
		"ConfigRefresh", svc.doConfigRefresh,
		sub.At(controlapi.ConfigRefresh.Method, controlapi.ConfigRefresh.Route),
		sub.Description(`ConfigRefresh pulls the latest config values from the configurator microservice.`),
		sub.Function(controlapi.ConfigRefreshIn{}, controlapi.ConfigRefreshOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: Trace
		"Trace", svc.doTrace,
		sub.At(controlapi.Trace.Method, controlapi.Trace.Route),
		sub.Description(`Trace forces exporting the indicated tracing span.`),
		sub.Function(controlapi.TraceIn{}, controlapi.TraceOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: OpenAPI
		"OpenAPI", svc.doOpenAPI,
		sub.At(controlapi.OpenAPI.Method, controlapi.OpenAPI.Route),
		sub.Description(`OpenAPI returns the OpenAPI 3.1 document of the microservice. Returns endpoints across all ports filtered by the caller's claims; consumers (portal/MCP) apply any port-based filtering at their ingress boundary.`),
		sub.Function(controlapi.OpenAPIIn{}, controlapi.OpenAPIOut{}),
	)

	// Web endpoints
	svc.Subscribe( // MARKER: Metrics
		"Metrics", svc.Metrics,
		sub.At(controlapi.Metrics.Method, controlapi.Metrics.Route),
		sub.Description(`Metrics returns the Prometheus metrics collected by the microservice.`),
		sub.Web(),
		sub.NoQueue(),
	)

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
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

// doOpenAPI handles marshaling for the OpenAPI function.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: OpenAPI
	var in controlapi.OpenAPIIn
	var out controlapi.OpenAPIOut
	err = marshalFunction(w, r, controlapi.OpenAPI.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.OpenAPI(r.Context())
		return err
	})
	return err // No trace
}

// marshalFunction handles marshaling for functional endpoints.
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

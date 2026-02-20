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

package smtpingress

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

	"github.com/microbus-io/fabric/coreservices/smtpingress/smtpingressapi"
	"github.com/microbus-io/fabric/coreservices/smtpingress/resources"
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
	_ smtpingressapi.Client
)

const (
	Hostname = smtpingressapi.Hostname
	Version  = 181
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	OnChangedPort(ctx context.Context) (err error)       // MARKER: Port
	OnChangedEnabled(ctx context.Context) (err error)    // MARKER: Enabled
	OnChangedMaxSize(ctx context.Context) (err error)    // MARKER: MaxSize
	OnChangedMaxClients(ctx context.Context) (err error) // MARKER: MaxClients
	OnChangedWorkers(ctx context.Context) (err error)    // MARKER: Workers
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
	svc.SetDescription(`The SMTP ingress microservice listens for incoming emails and fires corresponding events.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// Configs
	svc.DefineConfig( // MARKER: Port
		"Port",
		cfg.Description(`Port is the TCP port to listen to.`),
		cfg.DefaultValue(`25`),
		cfg.Validation(`int [1,65535]`),
	)
	svc.DefineConfig( // MARKER: Enabled
		"Enabled",
		cfg.Description(`Enabled determines whether the email server is started.`),
		cfg.DefaultValue(`true`),
		cfg.Validation(`bool`),
	)
	svc.DefineConfig( // MARKER: MaxSize
		"MaxSize",
		cfg.Description(`MaxSize is the maximum size of messages that will be accepted, in megabytes. Defaults to 10 megabytes.`),
		cfg.DefaultValue(`10`),
		cfg.Validation(`int [0,1024]`),
	)
	svc.DefineConfig( // MARKER: MaxClients
		"MaxClients",
		cfg.Description(`MaxClients controls how many client connections can be opened in parallel. Defaults to 128.`),
		cfg.DefaultValue(`128`),
		cfg.Validation(`int [1,1024]`),
	)
	svc.DefineConfig( // MARKER: Workers
		"Workers",
		cfg.Description(`Workers controls how many workers process incoming mail. Defaults to 8.`),
		cfg.DefaultValue(`8`),
		cfg.Validation(`int [1,1024]`),
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
	if changed("Port") { // MARKER: Port
		err = svc.OnChangedPort(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("Enabled") { // MARKER: Enabled
		err = svc.OnChangedEnabled(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("MaxSize") { // MARKER: MaxSize
		err = svc.OnChangedMaxSize(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("MaxClients") { // MARKER: MaxClients
		err = svc.OnChangedMaxClients(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("Workers") { // MARKER: Workers
		err = svc.OnChangedWorkers(ctx)
		if err != nil {
			return err // No trace
		}
	}
	return nil
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

/*
Port is the TCP port to listen to.
*/
func (svc *Intermediate) Port() (port int) { // MARKER: Port
	_val := svc.Config("Port")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetPort sets the value of the configuration property.
*/
func (svc *Intermediate) SetPort(port int) (err error) { // MARKER: Port
	return svc.SetConfig("Port", strconv.Itoa(port))
}

/*
Enabled determines whether the email server is started.
*/
func (svc *Intermediate) Enabled() (enabled bool) { // MARKER: Enabled
	_val := svc.Config("Enabled")
	_b, _ := strconv.ParseBool(_val)
	return _b
}

/*
SetEnabled sets the value of the configuration property.
*/
func (svc *Intermediate) SetEnabled(enabled bool) (err error) { // MARKER: Enabled
	return svc.SetConfig("Enabled", strconv.FormatBool(enabled))
}

/*
MaxSize is the maximum size of messages that will be accepted, in megabytes. Defaults to 10 megabytes.
*/
func (svc *Intermediate) MaxSize() (mb int) { // MARKER: MaxSize
	_val := svc.Config("MaxSize")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetMaxSize sets the value of the configuration property.
*/
func (svc *Intermediate) SetMaxSize(mb int) (err error) { // MARKER: MaxSize
	return svc.SetConfig("MaxSize", strconv.Itoa(mb))
}

/*
MaxClients controls how many client connections can be opened in parallel. Defaults to 128.
*/
func (svc *Intermediate) MaxClients() (clients int) { // MARKER: MaxClients
	_val := svc.Config("MaxClients")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetMaxClients sets the value of the configuration property.
*/
func (svc *Intermediate) SetMaxClients(clients int) (err error) { // MARKER: MaxClients
	return svc.SetConfig("MaxClients", strconv.Itoa(clients))
}

/*
Workers controls how many workers process incoming mail. Defaults to 8.
*/
func (svc *Intermediate) Workers() (clients int) { // MARKER: Workers
	_val := svc.Config("Workers")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetWorkers sets the value of the configuration property.
*/
func (svc *Intermediate) SetWorkers(clients int) (err error) { // MARKER: Workers
	return svc.SetConfig("Workers", strconv.Itoa(clients))
}

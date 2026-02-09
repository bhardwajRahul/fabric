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

package metrics

import (
	"context"
	"encoding/json"
	"net/http"

	"regexp"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
	"github.com/microbus-io/fabric/coreservices/metrics/resources"
)

// Version is the version of the code of the microservice.
const Version = 214

// Hostname is the default hostname of the microservice: metrics.core.
const Hostname = "metrics.core"

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Collect(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Collect
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the service.
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
	svc.SetDescription(`The Metrics service is a core microservice that aggregates metrics from other microservices and makes them available for collection.`)

	// Lifecycle
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)

	// Configs
	svc.DefineConfig(
		"SecretKey",
		cfg.Description(`SecretKey must be provided with the request to collect the metrics.
This key is required except in local development and tests.`),
		cfg.Secret(),
	)

	// OpenAPI
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)

	// Webs
	svc.Subscribe(`GET`, `:443/collect`, svc.Collect) // MARKER: Collect

	// Resources file system
	svc.SetResFS(resources.FS)

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
		{ // MARKER: Collect
			Type:        "web",
			Name:        "Collect",
			Method:      "GET",
			Route:       metricsapi.RouteOfCollect,
			Summary:     "Collect()",
			Description: `Collect the metrics of all microservices.`,
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

/*
SecretKey must be provided with the request to collect the metrics.
This key is required except in local development and tests.
*/
func (svc *Intermediate) SecretKey() (secretKey string) { // MARKER: SecretKey
	_val := svc.Config("SecretKey")
	return _val
}

/*
SetSecretKey sets the value of the configuration property.
This action is restricted to the TESTING deployment in which the fetching of values from the configurator is disabled.

SecretKey must be provided with the request to collect the metrics.
This key is required except in local development and tests.
*/
func (svc *Intermediate) SetSecretKey(secretKey string) (err error) {
	return svc.SetConfig("SecretKey", utils.AnyToString(secretKey))
}

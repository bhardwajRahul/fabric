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

package configurator

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

	"github.com/microbus-io/fabric/coreservices/configurator/configuratorapi"
	"github.com/microbus-io/fabric/coreservices/configurator/resources"
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
	_ configuratorapi.Client
	_ *workflow.Flow
)

const (
	Hostname = configuratorapi.Hostname
	Version  = 254
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Values(ctx context.Context, names []string) (values map[string]string, err error)                   // MARKER: Values
	Refresh(ctx context.Context) (err error)                                                            // MARKER: Refresh
	SyncRepo(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error) // MARKER: SyncRepo
	Values443(ctx context.Context, names []string) (values map[string]string, err error)                // MARKER: Values443
	Refresh443(ctx context.Context) (err error)                                                         // MARKER: Refresh443
	Sync443(ctx context.Context, timestamp time.Time, values map[string]map[string]string) (err error)  // MARKER: Sync443
	PeriodicRefresh(ctx context.Context) (err error)                                                    // MARKER: PeriodicRefresh
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
	svc.SetDescription(`The Configurator is a core microservice that centralizes the dissemination of configuration values to other microservices.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe( // MARKER: Values
		"Values", svc.doValues,
		sub.At(configuratorapi.Values.Method, configuratorapi.Values.Route),
		sub.Description(`Values returns the values associated with the specified config property names for the caller microservice.`),
		sub.Function(configuratorapi.ValuesIn{}, configuratorapi.ValuesOut{}),
	)
	svc.Subscribe( // MARKER: Refresh
		"Refresh", svc.doRefresh,
		sub.At(configuratorapi.Refresh.Method, configuratorapi.Refresh.Route),
		sub.Description(`Refresh tells all microservices to contact the configurator and refresh their configs.
An error is returned if any of the values sent to the microservices fails validation.`),
		sub.Function(configuratorapi.RefreshIn{}, configuratorapi.RefreshOut{}),
	)
	svc.Subscribe( // MARKER: SyncRepo
		"SyncRepo", svc.doSyncRepo,
		sub.At(configuratorapi.SyncRepo.Method, configuratorapi.SyncRepo.Route),
		sub.Description(`SyncRepo is used to synchronize values among replica peers of the configurator.`),
		sub.Function(configuratorapi.SyncRepoIn{}, configuratorapi.SyncRepoOut{}),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: Values443
		"Values443", svc.doValues443,
		sub.At(configuratorapi.Values443.Method, configuratorapi.Values443.Route),
		sub.Description(`Deprecated.`),
		sub.Function(configuratorapi.Values443In{}, configuratorapi.Values443Out{}),
	)
	svc.Subscribe( // MARKER: Refresh443
		"Refresh443", svc.doRefresh443,
		sub.At(configuratorapi.Refresh443.Method, configuratorapi.Refresh443.Route),
		sub.Description(`Deprecated.`),
		sub.Function(configuratorapi.Refresh443In{}, configuratorapi.Refresh443Out{}),
	)
	svc.Subscribe( // MARKER: Sync443
		"Sync443", svc.doSync443,
		sub.At(configuratorapi.Sync443.Method, configuratorapi.Sync443.Route),
		sub.Description(`Deprecated.`),
		sub.Function(configuratorapi.Sync443In{}, configuratorapi.Sync443Out{}),
	)

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here
	svc.StartTicker("PeriodicRefresh", 20*time.Minute, svc.PeriodicRefresh) // MARKER: PeriodicRefresh

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

// doValues handles marshaling for Values.
func (svc *Intermediate) doValues(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Values
	var in configuratorapi.ValuesIn
	var out configuratorapi.ValuesOut
	err = marshalFunction(w, r, configuratorapi.Values.Route, &in, &out, func(_ any, _ any) error {
		out.Values, err = svc.Values(r.Context(), in.Names)
		return err
	})
	return err // No trace
}

// doRefresh handles marshaling for Refresh.
func (svc *Intermediate) doRefresh(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Refresh
	var in configuratorapi.RefreshIn
	var out configuratorapi.RefreshOut
	err = marshalFunction(w, r, configuratorapi.Refresh.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Refresh(r.Context())
		return err
	})
	return err // No trace
}

// doSyncRepo handles marshaling for SyncRepo.
func (svc *Intermediate) doSyncRepo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SyncRepo
	var in configuratorapi.SyncRepoIn
	var out configuratorapi.SyncRepoOut
	err = marshalFunction(w, r, configuratorapi.SyncRepo.Route, &in, &out, func(_ any, _ any) error {
		err = svc.SyncRepo(r.Context(), in.Timestamp, in.Values)
		return err
	})
	return err // No trace
}

// doValues443 handles marshaling for Values443.
func (svc *Intermediate) doValues443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Values443
	var in configuratorapi.Values443In
	var out configuratorapi.Values443Out
	err = marshalFunction(w, r, configuratorapi.Values443.Route, &in, &out, func(_ any, _ any) error {
		out.Values, err = svc.Values443(r.Context(), in.Names)
		return err
	})
	return err // No trace
}

// doRefresh443 handles marshaling for Refresh443.
func (svc *Intermediate) doRefresh443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Refresh443
	var in configuratorapi.Refresh443In
	var out configuratorapi.Refresh443Out
	err = marshalFunction(w, r, configuratorapi.Refresh443.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Refresh443(r.Context())
		return err
	})
	return err // No trace
}

// doSync443 handles marshaling for Sync443.
func (svc *Intermediate) doSync443(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Sync443
	var in configuratorapi.Sync443In
	var out configuratorapi.Sync443Out
	err = marshalFunction(w, r, configuratorapi.Sync443.Route, &in, &out, func(_ any, _ any) error {
		err = svc.Sync443(r.Context(), in.Timestamp, in.Values)
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

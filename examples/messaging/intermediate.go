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

package messaging

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

	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
	"github.com/microbus-io/fabric/examples/messaging/resources"
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
	_ *workflow.Flow
	_ messagingapi.Client
)

const (
	Hostname = messagingapi.Hostname
	Version  = 230
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Home(w http.ResponseWriter, r *http.Request) (err error)         // MARKER: Home
	NoQueue(w http.ResponseWriter, r *http.Request) (err error)      // MARKER: NoQueue
	DefaultQueue(w http.ResponseWriter, r *http.Request) (err error) // MARKER: DefaultQueue
	CacheLoad(w http.ResponseWriter, r *http.Request) (err error)    // MARKER: CacheLoad
	CacheStore(w http.ResponseWriter, r *http.Request) (err error)   // MARKER: CacheStore
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
	svc.SetDescription(`The Messaging microservice demonstrates service-to-service communication patterns.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// Web endpoints
	svc.Subscribe( // MARKER: Home
		"Home", svc.Home,
		sub.At(messagingapi.Home.Method, messagingapi.Home.Route),
		sub.Description(`Home demonstrates making requests using multicast and unicast request/response patterns.`),
		sub.Web(),
	)
	svc.Subscribe( // MARKER: NoQueue
		"NoQueue", svc.NoQueue,
		sub.At(messagingapi.NoQueue.Method, messagingapi.NoQueue.Route),
		sub.Description(`NoQueue demonstrates how the NoQueue subscription option is used to create
a multicast request/response communication pattern.
All instances of this microservice will respond to each request.`),
		sub.Web(),
		sub.NoQueue(),
	)
	svc.Subscribe( // MARKER: DefaultQueue
		"DefaultQueue", svc.DefaultQueue,
		sub.At(messagingapi.DefaultQueue.Method, messagingapi.DefaultQueue.Route),
		sub.Description(`DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
a unicast request/response communication pattern.
Only one of the instances of this microservice will respond to each request.`),
		sub.Web(),
	)
	svc.Subscribe( // MARKER: CacheLoad
		"CacheLoad", svc.CacheLoad,
		sub.At(messagingapi.CacheLoad.Method, messagingapi.CacheLoad.Route),
		sub.Description(`CacheLoad looks up an element in the distributed cache of the microservice.`),
		sub.Web(),
	)
	svc.Subscribe( // MARKER: CacheStore
		"CacheStore", svc.CacheStore,
		sub.At(messagingapi.CacheStore.Method, messagingapi.CacheStore.Route),
		sub.Description(`CacheStore stores an element in the distributed cache of the microservice.`),
		sub.Web(),
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

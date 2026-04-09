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

package login

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

	"github.com/microbus-io/fabric/examples/login/loginapi"
	"github.com/microbus-io/fabric/examples/login/resources"
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
	_ loginapi.Client
)

const (
	Hostname = loginapi.Hostname
	Version  = 93
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Login(w http.ResponseWriter, r *http.Request) (err error)       // MARKER: Login
	Logout(w http.ResponseWriter, r *http.Request) (err error)      // MARKER: Logout
	Welcome(w http.ResponseWriter, r *http.Request) (err error)     // MARKER: Welcome
	AdminOnly(w http.ResponseWriter, r *http.Request) (err error)   // MARKER: AdminOnly
	ManagerOnly(w http.ResponseWriter, r *http.Request) (err error) // MARKER: ManagerOnly
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
	svc.SetDescription(`The Login microservice demonstrates usage of authentication and authorization.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	svc.Subscribe( // MARKER: Login
		"Login", svc.Login,
		sub.At(loginapi.Login.Method, loginapi.Login.Route),
		sub.Description(`Login renders a simple login screen that authenticates a user.
Known users are hardcoded as "admin", "manager" and "user".
The password is "password".`),
		sub.Web(),
	)
	svc.Subscribe( // MARKER: Logout
		"Logout", svc.Logout,
		sub.At(loginapi.Logout.Method, loginapi.Logout.Route),
		sub.Description(`Logout renders a page that logs out the user.`),
		sub.Web(),
	)
	svc.Subscribe( // MARKER: Welcome
		"Welcome", svc.Welcome,
		sub.At(loginapi.Welcome.Method, loginapi.Welcome.Route),
		sub.Description(`Welcome renders a page that is shown to the user after a successful login.
Rendering is adjusted based on the user's roles.`),
		sub.Web(),
		sub.RequiredClaims(`roles.a || roles.m || roles.u`),
	)
	svc.Subscribe( // MARKER: AdminOnly
		"AdminOnly", svc.AdminOnly,
		sub.At(loginapi.AdminOnly.Method, loginapi.AdminOnly.Route),
		sub.Description(`AdminOnly is only accessible by admins.`),
		sub.Web(),
		sub.RequiredClaims(`roles.a`),
	)
	svc.Subscribe( // MARKER: ManagerOnly
		"ManagerOnly", svc.ManagerOnly,
		sub.At(loginapi.ManagerOnly.Method, loginapi.ManagerOnly.Route),
		sub.Description(`ManagerOnly is only accessible by managers.`),
		sub.Web(),
		sub.RequiredClaims(`roles.m`),
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

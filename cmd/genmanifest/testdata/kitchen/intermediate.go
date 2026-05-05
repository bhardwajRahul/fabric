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

package kitchen

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

	"github.com/microbus-io/fabric/cmd/genmanifest/testdata/kitchen/kitchenapi"
	"github.com/microbus-io/fabric/cmd/genmanifest/testdata/kitchen/resources"
	"github.com/microbus-io/fabric/cmd/genmanifest/testdata/weird/weirdapi"
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
	_ kitchenapi.Client
	_ *workflow.Flow
)

const (
	Hostname = kitchenapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	MyFunc(ctx context.Context, input string) (output string, err error) // MARKER: MyFunc
	SelfPing(ctx context.Context) (err error)                            // MARKER: SelfPing
	AltHostFn(ctx context.Context) (err error)                           // MARKER: AltHostFn
	OnSomething(ctx context.Context, detail string) (ok bool, err error) // MARKER: OnSomething
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
	svc.SetDescription(`Kitchen is a fixture for genmanifest exercising every detected call pattern.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	svc.Subscribe( // MARKER: MyFunc
		"MyFunc", svc.doMyFunc,
		sub.At(kitchenapi.MyFunc.Method, kitchenapi.MyFunc.Route),
		sub.Description(`MyFunc handles the MyFunc endpoint and exercises every call pattern.`),
		sub.Function(kitchenapi.MyFuncIn{}, kitchenapi.MyFuncOut{}),
	)
	svc.Subscribe( // MARKER: SelfPing
		"SelfPing", svc.doSelfPing,
		sub.At(kitchenapi.SelfPing.Method, kitchenapi.SelfPing.Route),
		sub.Description(`SelfPing handles peer-to-peer self-pings.`),
		sub.Function(kitchenapi.SelfPingIn{}, kitchenapi.SelfPingOut{}),
	)
	svc.Subscribe( // MARKER: AltHostFn
		"AltHostFn", svc.doAltHostFn,
		sub.At(kitchenapi.AltHostFn.Method, kitchenapi.AltHostFn.Route),
		sub.Description(`AltHostFn registers on a non-own host via the //host form.`),
		sub.Function(kitchenapi.AltHostFnIn{}, kitchenapi.AltHostFnOut{}),
	)

	weirdapi.NewHook(svc).OnSomething(impl.OnSomething) // MARKER: OnSomething

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel()
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
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

// doMyFunc handles marshaling for MyFunc.
func (svc *Intermediate) doMyFunc(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MyFunc
	var in kitchenapi.MyFuncIn
	var out kitchenapi.MyFuncOut
	err = marshalFunction(w, r, kitchenapi.MyFunc.Route, &in, &out, func(_ any, _ any) error {
		out.Output, err = svc.MyFunc(r.Context(), in.Input)
		return err
	})
	return err
}

// doSelfPing handles marshaling for SelfPing.
func (svc *Intermediate) doSelfPing(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SelfPing
	var in kitchenapi.SelfPingIn
	var out kitchenapi.SelfPingOut
	err = marshalFunction(w, r, kitchenapi.SelfPing.Route, &in, &out, func(_ any, _ any) error {
		err = svc.SelfPing(r.Context())
		return err
	})
	return err
}

// doAltHostFn handles marshaling for AltHostFn.
func (svc *Intermediate) doAltHostFn(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AltHostFn
	var in kitchenapi.AltHostFnIn
	var out kitchenapi.AltHostFnOut
	err = marshalFunction(w, r, kitchenapi.AltHostFn.Route, &in, &out, func(_ any, _ any) error {
		err = svc.AltHostFn(r.Context())
		return err
	})
	return err
}

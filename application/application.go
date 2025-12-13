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

package application

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/testarossa"
)

// Application is a collection of microservices that run in a single process and share the same lifecycle.
type Application struct {
	groups          []group
	sig             chan os.Signal
	plane           string
	deployment      string
	mux             sync.Mutex
	startupTimeout  time.Duration
	shutdownTimeout time.Duration
}

// New creates a new application.
// An application is a collection of microservices that run in a single process and share the same lifecycle.
// A unique plane of communication is used to isolate the app if it is running in a unit test environment.
func New() *Application {
	app := &Application{
		sig:             make(chan os.Signal, 1),
		plane:           env.Get("MICROBUS_PLANE"),
		deployment:      env.Get("MICROBUS_DEPLOYMENT"),
		startupTimeout:  time.Second * 20,
		shutdownTimeout: time.Second * 20,
	}
	return app
}

// NewTesting creates a new application explicitly for running in a unit test environment.
// A random plane of communication is used to isolate the testing app from other apps.
//
// Deprecated: Use [New] with [Application.RunInTest] instead.
func NewTesting() *Application {
	app := &Application{
		sig:            make(chan os.Signal, 1),
		plane:          rand.AlphaNum64(12),
		deployment:     connector.TESTING,
		startupTimeout: time.Second * 8,
	}
	return app
}

/*
Add adds a collection of microservices to be managed by the app.
Added microservices are not started up immediately. An explicit call to [Startup] or [Run] is required.
Microservices that are added together are started in parallel.
Otherwise, microservices are started sequentially in order of inclusion.

In the following example, A is started first, then B1 and B2 in parallel, and finally C1 and C2 in parallel.

	app := application.New()
	app.Add(a)
	app.Add(b1, b2)
	app.Add(c1, c2)
	app.Run()
*/
func (app *Application) Add(services ...service.Service) {
	app.mux.Lock()
	g := group{}
	for _, s := range services {
		s.SetPlane(app.plane)
		s.SetDeployment(app.deployment)
	}
	g = append(g, services...)
	app.groups = append(app.groups, g)
	app.mux.Unlock()
}

// AddAndStartup adds a collection of microservices to the app, and starts them up immediately.
func (app *Application) AddAndStartup(services ...service.Service) (err error) {
	app.mux.Lock()
	g := group{}
	for _, s := range services {
		s.SetPlane(app.plane)
		s.SetDeployment(app.deployment)
	}
	g = append(g, services...)
	app.groups = append(app.groups, g)
	app.mux.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), app.startupTimeout)
	defer cancel()
	err = g.Startup(ctx)
	return errors.Trace(err)
}

// Remove removes the microservices from under management of the app.
// Removed microservices are not shut down automatically and remain running on the same plane of communications.
func (app *Application) Remove(services ...service.Service) {
	toRemove := map[service.Service]bool{}
	for _, s := range services {
		toRemove[s] = true
	}
	app.mux.Lock()
	for gi := range app.groups {
		g := group{}
		for si := range app.groups[gi] {
			s := app.groups[gi][si]
			if !toRemove[s] {
				g = append(g, s)
			}
		}
		if len(app.groups[gi]) != len(g) {
			app.groups[gi] = g
		}
	}
	app.mux.Unlock()
}

// Startup starts all unstarted microservices included in this app.
// Microservices that are included together are started in parallel together.
// Otherwise, microservices are started sequentially in order of inclusion.
// If an error is returned, there is no guarantee as to the state of the microservices:
// some microservices may have been started while others not.
func (app *Application) Startup() error {
	app.mux.Lock()
	defer app.mux.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), app.startupTimeout)
	defer cancel()

	// Start each of the groups sequentially
	for _, g := range app.groups {
		err := g.Startup(ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Shutdown shuts down all started microservices included in this app in the reverse order of their starting up.
// If an error is returned, there is no guarantee as to the state of the microservices:
// some microservices may have been shut down while others not.
func (app *Application) Shutdown() error {
	app.mux.Lock()
	defer app.mux.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
	defer cancel()

	// Stop each of the groups sequentially in reverse order
	for i := len(app.groups) - 1; i >= 0; i-- {
		err := app.groups[i].Shutdown(ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// WaitForInterrupt blocks until an interrupt is received through
// a SIGTERM, SIGINT or a call to interrupt.
func (app *Application) WaitForInterrupt() {
	signal.Notify(app.sig, syscall.SIGINT, syscall.SIGTERM)
	<-app.sig
}

// Interrupt the app.
func (app *Application) Interrupt() {
	app.sig <- syscall.SIGINT
}

// Run starts up all microservices included in this app, waits for interrupt, then shuts them down.
func (app *Application) Run() error {
	err := app.Startup()
	if err != nil {
		return errors.Trace(err)
	}
	app.WaitForInterrupt()
	err = app.Shutdown()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// RunInTest starts up all microservices included in this app, waits for the test to finish, then shuts them down.
// A random plane of communication is used to isolate the testing app from other apps.
// Errors in startup or shutdown will fail the test.
func (app *Application) RunInTest(t testing.TB) error {
	app.plane = rand.AlphaNum64(12)
	app.deployment = connector.TESTING
	app.startupTimeout = time.Second * 8
	for _, g := range app.groups {
		for _, s := range g {
			s.SetPlane(app.plane)
			s.SetDeployment(app.deployment)
		}
	}

	assert := testarossa.For(t)
	t.Cleanup(func() {
		shutdownErr := app.Shutdown()
		assert.NoError(shutdownErr)
	})
	startupErr := app.Startup()
	if !assert.NoError(startupErr) {
		t.FailNow()
		return errors.Trace(startupErr)
	}
	return nil
}

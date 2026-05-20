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
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/testarossa"
)

// Application is a collection of microservices that run in a single process and share the same lifecycle.
type Application struct {
	groups     []group
	sig        chan os.Signal
	plane      string
	deployment string
	mux        sync.Mutex
}

// New creates a new application.
// An application is a collection of microservices that run in a single process and share the same lifecycle.
// A unique plane of communication is used to isolate the app if it is running in a unit test environment.
func New() *Application {
	app := &Application{
		sig:        make(chan os.Signal, 1),
		plane:      env.Get("MICROBUS_PLANE"),
		deployment: env.Get("MICROBUS_DEPLOYMENT"),
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
func (app *Application) AddAndStartup(ctx context.Context, services ...service.Service) (err error) {
	app.mux.Lock()
	g := group{}
	for _, s := range services {
		s.SetPlane(app.plane)
		s.SetDeployment(app.deployment)
	}
	g = append(g, services...)
	app.groups = append(app.groups, g)
	app.mux.Unlock()

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
// The context deadline is used to limit the time allotted to the operation.
func (app *Application) Startup(ctx context.Context) error {
	app.mux.Lock()
	defer app.mux.Unlock()

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
// The context deadline is used to limit the time allotted to the operation.
func (app *Application) Shutdown(ctx context.Context) error {
	app.mux.Lock()
	defer app.mux.Unlock()

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
// Defaults: 120 seconds for startup and 24 seconds for shutdown. The 24-second shutdown budget sits
// under Kubernetes' 30-second default terminationGracePeriodSeconds, leaving ~6 seconds for the
// runtime and orchestrator to finalize before SIGKILL. Kubernetes does not impose a comparable
// hard ceiling on startup (startup probes and progressDeadlineSeconds are operator-tuned and much
// longer), so 120 seconds gives microservices headroom to warm caches, load ML models, or wait on
// slow dependencies. Override either via the env vars MICROBUS_STARTUP_TIME_BUDGET /
// MICROBUS_SHUTDOWN_TIME_BUDGET. The chosen budget is applied as a deadline on the ctx passed to
// each microservice's OnStartup / OnShutdown.
// For programmatic control, use [Application.Startup] and [Application.Shutdown] directly.
func (app *Application) Run() error {
	startupBudget := envDurationOrDefault("MICROBUS_STARTUP_TIME_BUDGET", 120*time.Second)
	shutdownBudget := envDurationOrDefault("MICROBUS_SHUTDOWN_TIME_BUDGET", 24*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), startupBudget)
	err := errors.CatchPanic(func() error {
		return app.Startup(ctx)
	})
	cancel()
	if err != nil {
		return errors.Trace(err)
	}
	app.WaitForInterrupt()

	ctx, cancel = context.WithTimeout(context.Background(), shutdownBudget)
	err = errors.CatchPanic(func() error {
		return app.Shutdown(ctx)
	})
	cancel()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// envDurationOrDefault reads a duration env var via the env package, falling back to def when
// unset or unparseable. Unparseable values are silently treated as unset so a typo never
// reduces the budget to zero.
func envDurationOrDefault(name string, def time.Duration) time.Duration {
	v := env.Get(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// RunInTest starts up all microservices included in this app, waits for the test to finish, then shuts them down.
// A random plane of communication is used to isolate the testing app from other apps.
// Errors in startup or shutdown will fail the test.
func (app *Application) RunInTest(t testing.TB) error {
	app.plane = utils.RandomIdentifier(12)
	app.deployment = connector.TESTING
	for _, g := range app.groups {
		for _, s := range g {
			s.SetPlane(app.plane)
			s.SetDeployment(app.deployment)
		}
	}

	assert := testarossa.For(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*8)
		shutdownErr := errors.CatchPanic(func() error {
			return app.Shutdown(ctx)
		})
		cancel()
		assert.NoError(shutdownErr)
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*8)
	startupErr := errors.CatchPanic(func() error {
		return app.Startup(ctx)
	})
	cancel()
	if !assert.NoError(startupErr) {
		t.FailNow()
	}
	return nil
}

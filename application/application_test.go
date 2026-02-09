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
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/testarossa"
)

func TestApplication_StartStop(t *testing.T) {
	assert := testarossa.For(t)
	ctx := t.Context()

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	alpha := connector.New("alpha.start.stop.application")
	beta := connector.New("beta.start.stop.application")
	app := New()
	app.Add(alpha, beta)

	assert.False(alpha.IsStarted())
	assert.False(beta.IsStarted())

	err := app.Startup(ctx)
	assert.NoError(err)

	assert.True(alpha.IsStarted())
	assert.True(beta.IsStarted())

	err = app.Shutdown(ctx)
	assert.NoError(err)

	assert.False(alpha.IsStarted())
	assert.False(beta.IsStarted())
}

func TestApplication_Interrupt(t *testing.T) {
	assert := testarossa.For(t)
	ctx := t.Context()

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	con := connector.New("interrupt.application")
	app := New()
	app.Add(con)

	ch := make(chan bool)
	go func() {
		err := app.Startup(ctx)
		assert.NoError(err)
		go func() {
			app.WaitForInterrupt()
			err := app.Shutdown(ctx)
			assert.NoError(err)
			ch <- true
		}()
		ch <- true
	}()

	<-ch
	assert.True(con.IsStarted())
	app.Interrupt()
	<-ch
	assert.False(con.IsStarted())
}

func TestApplication_NoConflict(t *testing.T) {
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create first testing app
	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	alpha := connector.New("no.conflict.application")
	alpha.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("alpha"))
		return nil
	})
	appAlpha := New()
	appAlpha.Add(alpha)
	env.Pop("MICROBUS_PLANE")
	env.Pop("MICROBUS_DEPLOYMENT")

	// Create second testing app
	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	beta := connector.New("no.conflict.application")
	beta.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("beta"))
		return nil
	})
	appBeta := New()
	appBeta.Add(beta)
	env.Pop("MICROBUS_PLANE")
	env.Pop("MICROBUS_DEPLOYMENT")

	// Start the apps
	err := appAlpha.Startup(ctx)
	assert.NoError(err)
	defer appAlpha.Shutdown(ctx)
	err = appBeta.Startup(ctx)
	assert.NoError(err)
	defer appBeta.Shutdown(ctx)

	// Assert different planes of communication
	assert.NotEqual(alpha.Plane(), beta.Plane())
	assert.Equal(connector.TESTING, alpha.Deployment())
	assert.Equal(connector.TESTING, beta.Deployment())

	// Alpha should never see beta
	for range 32 {
		response, err := alpha.GET(ctx, "https://no.conflict.application/id")
		assert.NoError(err)
		body, err := io.ReadAll(response.Body)
		assert.NoError(err)
		assert.Equal("alpha", string(body))
	}

	// Beta should never see alpha
	for range 32 {
		response, err := beta.GET(ctx, "https://no.conflict.application/id")
		assert.NoError(err)
		body, err := io.ReadAll(response.Body)
		assert.NoError(err)
		assert.Equal("beta", string(body))
	}
}

func TestApplication_DependencyStart(t *testing.T) {
	assert := testarossa.For(t)
	ctx := t.Context()

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	// Alpha is dependent on beta to start
	failCount := 0
	alpha := connector.New("alpha.dependency.start.application")
	alpha.SetOnStartup(func(ctx context.Context) error {
		_, err := alpha.Request(ctx, pub.GET("https://beta.dependency.start.application/ok"))
		if err != nil {
			failCount++
			return errors.Trace(err)
		}
		return nil
	})

	// Beta takes half a second to start
	beta := connector.New("beta.dependency.start.application")
	beta.Subscribe("GET", "/ok", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	beta.SetOnStartup(func(ctx context.Context) error {
		time.Sleep(time.Millisecond * 500)
		return nil
	})

	app := New()
	app.Add(alpha, beta)
	t0 := time.Now()
	err := app.Startup(ctx)
	dur := time.Since(t0)
	assert.NoError(err)
	assert.NotZero(failCount)
	assert.True(dur >= time.Millisecond*500)

	app.Shutdown(ctx)
}

func TestApplication_FailStart(t *testing.T) {
	assert := testarossa.For(t)
	ctx := t.Context()

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	startupTimeout := time.Second

	// Alpha fails to start
	failCount := 0
	alpha := connector.New("alpha.fail.start.application")
	alpha.SetOnStartup(func(ctx context.Context) error {
		failCount++
		return errors.New("oops")
	})

	// Beta starts without a hitch
	beta := connector.New("beta.fail.start.application")

	app := New()
	app.Add(alpha, beta)
	t0 := time.Now()
	shortCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	err := app.Startup(shortCtx)
	cancel()
	dur := time.Since(t0)
	assert.Error(err)
	assert.True(failCount > 0)
	assert.True(dur >= startupTimeout)
	assert.True(beta.IsStarted())
	assert.False(alpha.IsStarted())

	err = app.Shutdown(ctx)
	assert.NoError(err)
	assert.False(beta.IsStarted())
	assert.False(alpha.IsStarted())
}

func TestApplication_Remove(t *testing.T) {
	assert := testarossa.For(t)
	ctx := t.Context()

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	alpha := connector.New("alpha.remove.application")
	beta := connector.New("beta.remove.application")

	app := New()
	app.AddAndStartup(ctx, alpha, beta)
	assert.True(alpha.IsStarted())
	assert.True(beta.IsStarted())
	assert.Equal(alpha.Plane(), beta.Plane())

	app.Remove(beta)
	assert.True(alpha.IsStarted())
	assert.True(beta.IsStarted())
	assert.Equal(alpha.Plane(), beta.Plane())

	err := app.Shutdown(ctx)
	assert.NoError(err)
	assert.False(alpha.IsStarted())
	assert.True(beta.IsStarted()) // Should remain up because no longer under management of the app

	err = beta.Shutdown(ctx)
	assert.NoError(err)
	assert.False(beta.IsStarted())
}

func TestApplication_Run(t *testing.T) {
	assert := testarossa.For(t)

	env.Push("MICROBUS_PLANE", utils.RandomIdentifier(12))
	env.Push("MICROBUS_DEPLOYMENT", connector.TESTING)
	defer env.Pop("MICROBUS_PLANE")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	con := connector.New("run.application")
	config := configurator.NewService()
	app := New()
	app.Add(config)
	app.Add(con)

	go func() {
		err := app.Run()
		assert.NoError(err)
	}()

	time.Sleep(2 * time.Second)
	assert.True(con.IsStarted())
	assert.True(config.IsStarted())

	app.Interrupt()

	time.Sleep(time.Second)
	assert.False(con.IsStarted())
	assert.False(config.IsStarted())
}

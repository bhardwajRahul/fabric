/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"
)

func TestApplication_StartStop(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	alpha := connector.New("alpha.start.stop.application")
	beta := connector.New("beta.start.stop.application")
	app := NewTesting()
	app.Add(alpha, beta)

	tt.False(alpha.IsStarted())
	tt.False(beta.IsStarted())

	err := app.Startup()
	tt.NoError(err)

	tt.True(alpha.IsStarted())
	tt.True(beta.IsStarted())

	err = app.Shutdown()
	tt.NoError(err)

	tt.False(alpha.IsStarted())
	tt.False(beta.IsStarted())
}

func TestApplication_Interrupt(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := connector.New("interrupt.application")
	app := NewTesting()
	app.Add(con)

	ch := make(chan bool)
	go func() {
		err := app.Startup()
		tt.NoError(err)
		go func() {
			app.WaitForInterrupt()
			err := app.Shutdown()
			tt.NoError(err)
			ch <- true
		}()
		ch <- true
	}()

	<-ch
	tt.True(con.IsStarted())
	app.Interrupt()
	<-ch
	tt.False(con.IsStarted())
}

func TestApplication_NoConflict(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create first testing app
	alpha := connector.New("no.conflict.application")
	alpha.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("alpha"))
		return nil
	})
	appAlpha := NewTesting()
	appAlpha.Add(alpha)

	// Create second testing app
	beta := connector.New("no.conflict.application")
	beta.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("beta"))
		return nil
	})
	appBeta := NewTesting()
	appBeta.Add(beta)

	// Start the apps
	err := appAlpha.Startup()
	tt.NoError(err)
	defer appAlpha.Shutdown()
	err = appBeta.Startup()
	tt.NoError(err)
	defer appBeta.Shutdown()

	// Assert different planes of communication
	tt.NotEqual(alpha.Plane(), beta.Plane())
	tt.Equal(connector.TESTING, alpha.Deployment())
	tt.Equal(connector.TESTING, beta.Deployment())

	// Alpha should never see beta
	for range 32 {
		response, err := alpha.GET(ctx, "https://no.conflict.application/id")
		tt.NoError(err)
		body, err := io.ReadAll(response.Body)
		tt.NoError(err)
		tt.Equal("alpha", string(body))
	}

	// Beta should never see alpha
	for range 32 {
		response, err := beta.GET(ctx, "https://no.conflict.application/id")
		tt.NoError(err)
		body, err := io.ReadAll(response.Body)
		tt.NoError(err)
		tt.Equal("beta", string(body))
	}
}

func TestApplication_DependencyStart(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	startupTimeout := time.Second * 2

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

	// Beta takes a bit of time to start
	beta := connector.New("beta.dependency.start.application")
	beta.Subscribe("GET", "/ok", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	beta.SetOnStartup(func(ctx context.Context) error {
		time.Sleep(startupTimeout / 2)
		return nil
	})

	app := NewTesting()
	app.Add(alpha, beta)
	app.startupTimeout = startupTimeout
	t0 := time.Now()
	err := app.Startup()
	dur := time.Since(t0)
	tt.NoError(err)
	tt.True(failCount > 0)
	tt.True(dur >= startupTimeout/2)

	app.Shutdown()
}

func TestApplication_FailStart(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

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

	app := NewTesting()
	app.Add(alpha, beta)
	app.startupTimeout = startupTimeout
	t0 := time.Now()
	err := app.Startup()
	dur := time.Since(t0)
	tt.Error(err)
	tt.True(failCount > 0)
	tt.True(dur >= startupTimeout)
	tt.True(beta.IsStarted())
	tt.False(alpha.IsStarted())

	err = app.Shutdown()
	tt.NoError(err)
	tt.False(beta.IsStarted())
	tt.False(alpha.IsStarted())
}

func TestApplication_Remove(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	alpha := connector.New("alpha.remove.application")
	beta := connector.New("beta.remove.application")

	app := NewTesting()
	app.AddAndStartup(alpha, beta)
	tt.True(alpha.IsStarted())
	tt.True(beta.IsStarted())
	tt.Equal(alpha.Plane(), beta.Plane())

	app.Remove(beta)
	tt.True(alpha.IsStarted())
	tt.True(beta.IsStarted())
	tt.Equal(alpha.Plane(), beta.Plane())

	err := app.Shutdown()
	tt.NoError(err)
	tt.False(alpha.IsStarted())
	tt.True(beta.IsStarted()) // Should remain up because no longer under management of the app

	err = beta.Shutdown()
	tt.NoError(err)
	tt.False(beta.IsStarted())
}

func TestApplication_Run(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := connector.New("run.application")
	config := configurator.NewService()
	app := NewTesting()
	app.Add(config)
	app.Add(con)

	go func() {
		err := app.Run()
		tt.NoError(err)
	}()

	time.Sleep(2 * time.Second)
	tt.True(con.IsStarted())
	tt.True(config.IsStarted())

	app.Interrupt()

	time.Sleep(time.Second)
	tt.False(con.IsStarted())
	tt.False(config.IsStarted())
}

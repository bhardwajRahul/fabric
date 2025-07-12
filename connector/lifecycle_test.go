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

package connector

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestConnector_StartupShutdown(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var startupCalled, shutdownCalled bool

	con := New("startup.shutdown.connector")
	con.SetOnStartup(func(ctx context.Context) error {
		startupCalled = true
		return nil
	})
	con.SetOnShutdown(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	tt.False(startupCalled)
	tt.False(shutdownCalled)
	tt.False(con.IsStarted())

	err := con.Startup()
	tt.NoError(err)
	tt.True(startupCalled)
	tt.False(shutdownCalled)
	tt.True(con.IsStarted())

	err = con.Shutdown()
	tt.NoError(err)
	tt.True(startupCalled)
	tt.True(shutdownCalled)
	tt.False(con.IsStarted())
}

func TestConnector_StartupError(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var startupCalled, shutdownCalled bool

	con := New("startup.error.connector")
	con.SetOnStartup(func(ctx context.Context) error {
		startupCalled = true
		return errors.New("oops")
	})
	con.SetOnShutdown(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	tt.False(startupCalled)
	tt.False(shutdownCalled)
	tt.False(con.IsStarted())

	err := con.Startup()
	tt.Error(err)
	tt.True(startupCalled)
	tt.True(shutdownCalled)
	tt.False(con.IsStarted())

	err = con.Shutdown()
	tt.Error(err)
	tt.True(startupCalled)
	tt.True(shutdownCalled)
	tt.False(con.IsStarted())
}

func TestConnector_StartupPanic(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("startup.panic.connector")
	con.SetOnStartup(func(ctx context.Context) error {
		panic("really bad")
	})
	err := con.Startup()
	tt.Error(err)
	tt.Equal("really bad", err.Error())
}

func TestConnector_ShutdownPanic(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("shutdown.panic.connector")
	con.SetOnShutdown(func(ctx context.Context) error {
		panic("really bad")
	})
	err := con.Startup()
	tt.NoError(err)
	err = con.Shutdown()
	tt.Error(err)
	tt.Equal("really bad", err.Error())
}

func TestConnector_StartupTimeout(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("startup.timeout.connector")

	done := make(chan bool)
	con.SetOnStartup(func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		<-ctx.Done()
		return ctx.Err()
	})

	go func() {
		err := con.Startup()
		tt.Error(err)
		done <- true
	}()
	time.Sleep(600 * time.Millisecond)
	<-done
	tt.False(con.IsStarted())
}

func TestConnector_ShutdownTimeout(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("shutdown.timeout.connector")

	done := make(chan bool)
	con.SetOnShutdown(func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		<-ctx.Done()
		return ctx.Err()
	})

	err := con.Startup()
	tt.NoError(err)
	tt.True(con.IsStarted())

	go func() {
		err := con.Shutdown()
		tt.Error(err)
		done <- true
	}()
	time.Sleep(600 * time.Millisecond)
	<-done
	tt.False(con.IsStarted())
}

func TestConnector_InitError(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("init.error.connector")
	err := con.DefineConfig("Hundred", cfg.DefaultValue("101"), cfg.Validation("int [1,100]"))
	tt.Error(err)
	err = con.Startup()
	tt.Error(err)

	con = New("init.error.connector")
	err = con.DefineConfig("Hundred", cfg.DefaultValue("1"), cfg.Validation("int [1,100]"))
	tt.NoError(err)
	err = con.SetConfig("Hundred", "101")
	tt.Error(err)
	err = con.Startup()
	tt.Error(err)

	con = New("init.error.connector")
	err = con.Subscribe("GET", ":BAD/path", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	tt.Error(err)
	err = con.Startup()
	tt.Error(err)

	con = New("init.error.connector")
	err = con.StartTicker("ticktock", -time.Minute, func(ctx context.Context) error {
		return nil
	})
	tt.Error(err)
	err = con.Startup()
	tt.Error(err)
}

func TestConnector_Restart(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var startupCalled atomic.Int32
	var shutdownCalled atomic.Int32
	var endpointCalled atomic.Int32
	var tickerCalled atomic.Int32

	// Set up a configurator
	plane := rand.AlphaNum64(12)
	configurator := New("configurator.core")
	configurator.SetDeployment(LAB) // Tickers and configs are disabled in TESTING
	configurator.SetPlane(plane)

	err := configurator.Startup()
	tt.NoError(err)
	defer configurator.Shutdown()

	// Set up the connector
	con := New("restart.connector")
	con.SetDeployment(LAB) // Tickers and configs are disabled in TESTING
	con.SetPlane(plane)
	con.SetOnStartup(func(ctx context.Context) error {
		startupCalled.Add(1)
		return nil
	})
	con.SetOnShutdown(func(ctx context.Context) error {
		shutdownCalled.Add(1)
		return nil
	})
	con.Subscribe("GET", "/endpoint", func(w http.ResponseWriter, r *http.Request) error {
		endpointCalled.Add(1)
		return nil
	})
	con.StartTicker("tick", time.Millisecond*500, func(ctx context.Context) error {
		tickerCalled.Add(1)
		return nil
	})
	con.DefineConfig("config", cfg.DefaultValue("default"))

	tt.Equal("default", con.configs["config"].Value)

	// Startup
	configurator.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte(`{"values":{"config":"overriden"}}`))
		return nil
	})
	err = con.Startup()
	tt.NoError(err)
	tt.Equal(int32(1), startupCalled.Load())
	tt.Zero(shutdownCalled.Load())
	_, err = con.Request(con.lifetimeCtx, pub.GET("https://restart.connector/endpoint"))
	tt.NoError(err)
	tt.Equal(int32(1), endpointCalled.Load())
	time.Sleep(time.Second)
	tt.True(tickerCalled.Load() > 0)
	tt.Equal("overriden", con.Config("config"))

	// Shutdown
	err = con.Shutdown()
	tt.NoError(err)
	tt.Equal(int32(1), shutdownCalled.Load())

	// Restart
	configurator.Unsubscribe("POST", ":888/values")
	configurator.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte(`{}`))
		return nil
	})
	startupCalled.Store(0)
	shutdownCalled.Store(0)
	endpointCalled.Store(0)
	tickerCalled.Store(0)

	err = con.Startup()
	tt.NoError(err)
	tt.Equal(int32(1), startupCalled.Load())
	tt.Zero(shutdownCalled.Load())
	_, err = con.Request(con.lifetimeCtx, pub.GET("https://restart.connector/endpoint"))
	tt.NoError(err)
	tt.Equal(int32(1), endpointCalled.Load())
	time.Sleep(time.Second)
	tt.True(tickerCalled.Load() > 0)
	tt.Equal("default", con.Config("config"))

	// Shutdown
	err = con.Shutdown()
	tt.NoError(err)
	tt.Equal(int32(1), shutdownCalled.Load())
}

func TestConnector_GoGracefulShutdown(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	ctx := context.Background()

	con := New("go.graceful.shutdown.connector")
	err := con.Startup()
	tt.NoError(err)

	done500 := false
	con.Go(ctx, func(ctx context.Context) (err error) {
		time.Sleep(500 * time.Millisecond)
		done500 = true
		return nil
	})
	done300 := false
	con.Go(ctx, func(ctx context.Context) (err error) {
		time.Sleep(400 * time.Millisecond)
		done300 = true
		return nil
	})
	started := time.Now()
	err = con.Shutdown()
	tt.NoError(err)
	dur := time.Since(started)
	tt.True(dur >= 500*time.Millisecond)
	tt.True(done500)
	tt.True(done300)
}

func TestConnector_Parallel(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := New("parallel.connector")
	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	j1 := false
	j2 := false
	j3 := false
	started := time.Now()
	err = con.Parallel(
		func() (err error) {
			time.Sleep(100 * time.Millisecond)
			j1 = true
			return nil
		},
		func() (err error) {
			time.Sleep(200 * time.Millisecond)
			j2 = true
			return nil
		},
		func() (err error) {
			time.Sleep(300 * time.Millisecond)
			j3 = true
			return nil
		},
	)
	dur := time.Since(started)
	tt.True(dur >= 300*time.Millisecond)
	tt.NoError(err)
	tt.True(j1)
	tt.True(j2)
	tt.True(j3)
}

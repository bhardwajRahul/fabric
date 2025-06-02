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
	"testing"

	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestConnector_SetConfig(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	// Mock config service
	mockCfg := New("configurator.core")
	mockCfg.SetDeployment(LAB) // Configs are disabled in TESTING
	mockCfg.SetPlane(plane)
	mockCfg.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
		return nil
	})

	err := mockCfg.Startup()
	tt.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("set.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)

	err = con.DefineConfig("s", cfg.DefaultValue("default"))
	tt.NoError(err)
	tt.Equal("default", con.Config("s"))

	err = con.SetConfig("s", "changed")
	tt.NoError(err)
	tt.Equal("changed", con.Config("s"))

	err = con.ResetConfig("s")
	tt.NoError(err)
	tt.Equal("default", con.Config("s"))

	err = con.SetConfig("s", "changed")
	tt.NoError(err)
	tt.Equal("changed", con.Config("s"))

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal("default", con.Config("s")) // Gets reset after fetching from configurator

	err = con.SetConfig("s", "something")
	tt.Error(err)
	tt.Equal("default", con.Config("s"))

	err = con.ResetConfig("s")
	tt.Error(err)
}

func TestConnector_FetchConfig(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	ctx := context.Background()

	plane := rand.AlphaNum64(12)

	// Mock a config service
	mockCfg := New("configurator.core")
	mockCfg.SetDeployment(LAB) // Configs are disabled in TESTING
	mockCfg.SetPlane(plane)
	fooValue := "baz"
	intValue := "$$$"
	mockCfg.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"values":{"foo":"` + fooValue + `","int":"` + intValue + `"}}`))
		return nil
	})

	err := mockCfg.Startup()
	tt.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("fetch.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)
	err = con.DefineConfig("foo", cfg.DefaultValue("bar"))
	tt.NoError(err)
	err = con.DefineConfig("int", cfg.Validation("int"), cfg.DefaultValue("5"))
	tt.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		tt.True(changed("FOO"))
		tt.True(changed("int"))
		callbackCalled = true
		return nil
	})
	tt.NoError(err)

	tt.Equal("bar", con.Config("foo"))
	tt.Equal("5", con.Config("int"))

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal("baz", con.Config("foo"), "New value should be read from configurator")
	tt.Equal("5", con.Config("int"), "Invalid value should not be accepted")
	tt.False(callbackCalled)

	fooValue = "bam"
	intValue = "8"
	_, err = mockCfg.GET(ctx, "https://fetch.config.connector:888/config-refresh")
	tt.NoError(err)

	tt.Equal("bam", con.Config("foo"))
	tt.Equal("8", con.Config("int"))
	tt.True(callbackCalled)
}

func TestConnector_NoFetchInTestingApp(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	// Mock a config service
	mockCfg := New("configurator.core")
	mockCfg.SetPlane(plane)
	mockCfg.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"values":{"foo":"baz"}}`))
		return nil
	})

	err := mockCfg.Startup()
	tt.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("no.fetch.in.testing.app.config.connector")
	con.SetPlane(plane)
	err = con.DefineConfig("foo", cfg.DefaultValue("bar"))
	tt.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		callbackCalled = true
		return nil
	})
	tt.NoError(err)

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal("bar", con.Config("foo"))
	tt.False(callbackCalled)
}

func TestConnector_CallbackWhenStarted(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Connector
	con := New("callback.when.started.config.connector")
	err := con.DefineConfig("foo", cfg.DefaultValue("bar"))
	tt.NoError(err)
	callbackCalled := 0
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		callbackCalled++
		tt.True(changed("foo"))
		return nil
	})
	tt.NoError(err)

	con.SetConfig("foo", "baz")
	tt.Equal("baz", con.Config("foo"))
	tt.Zero(callbackCalled)

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()
	tt.Zero(callbackCalled)

	con.SetConfig("foo", "bam")
	tt.Equal("bam", con.Config("foo"))
	tt.Equal(1, callbackCalled)
}

func TestConnector_CaseInsensitiveConfig(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	ctx := context.Background()

	plane := rand.AlphaNum64(12)

	// Mock a config service
	mockCfg := New("configurator.core")
	mockCfg.SetDeployment(LAB) // Configs are disabled in TESTING
	mockCfg.SetPlane(plane)
	configValue := "bar"
	mockCfg.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"values":{"foo-config_":"` + configValue + `"}}`))
		return nil
	})

	err := mockCfg.Startup()
	tt.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("case.insensitive.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)
	err = con.DefineConfig("foo-config_", cfg.DefaultValue("bar"))
	tt.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		tt.True(changed("foo-config_"))
		tt.True(changed("foo-CONFIG_"))
		tt.True(changed("FOO-config_"))
		callbackCalled = true
		return nil
	})
	tt.NoError(err)

	tt.Equal(configValue, con.Config("foo-config_"))
	tt.Equal(configValue, con.Config("foo-CONFIG_"))
	tt.Equal(configValue, con.Config("FOO-config_"))

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.False(callbackCalled)

	configValue = "baz"
	_, err = mockCfg.GET(ctx, "https://case.insensitive.config.connector:888/config-refresh")
	tt.NoError(err)

	tt.True(callbackCalled)
	tt.Equal(configValue, con.Config("foo-config_"))
	tt.Equal(configValue, con.Config("foo-CONFIG_"))
	tt.Equal(configValue, con.Config("FOO-config_"))
}

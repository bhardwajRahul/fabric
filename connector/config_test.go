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

package connector

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestConnector_SetConfig(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

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
	assert.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("set.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)

	err = con.DefineConfig("s", cfg.DefaultValue("default"))
	assert.NoError(err)
	assert.Equal("default", con.Config("s"))

	err = con.SetConfig("s", "changed")
	assert.NoError(err)
	assert.Equal("changed", con.Config("s"))

	err = con.ResetConfig("s")
	assert.NoError(err)
	assert.Equal("default", con.Config("s"))

	err = con.SetConfig("s", "changed")
	assert.NoError(err)
	assert.Equal("changed", con.Config("s"))

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Equal("default", con.Config("s")) // Gets reset after fetching from configurator

	err = con.SetConfig("s", "something")
	assert.Error(err)
	assert.Equal("default", con.Config("s"))

	err = con.ResetConfig("s")
	assert.Error(err)
}

func TestConnector_FetchConfig(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
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
	assert.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("fetch.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)
	err = con.DefineConfig("foo", cfg.DefaultValue("bar"))
	assert.NoError(err)
	err = con.DefineConfig("int", cfg.Validation("int"), cfg.DefaultValue("5"))
	assert.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		assert.True(changed("foo"))
		assert.True(changed("int"))
		callbackCalled = true
		return nil
	})
	assert.NoError(err)

	assert.Equal("bar", con.Config("foo"))
	assert.Equal("5", con.Config("int"))

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Equal("baz", con.Config("foo"), "New value should be read from configurator")
	assert.Equal("5", con.Config("int"), "Invalid value should not be accepted")
	assert.False(callbackCalled)

	fooValue = "bam"
	intValue = "8"
	_, err = mockCfg.GET(ctx, "https://fetch.config.connector:888/config-refresh")
	assert.NoError(err)

	assert.Equal("bam", con.Config("foo"))
	assert.Equal("8", con.Config("int"))
	assert.True(callbackCalled)
}

func TestConnector_NoFetchInTestingApp(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

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
	assert.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("no.fetch.in.testing.app.config.connector")
	con.SetPlane(plane)
	err = con.DefineConfig("foo", cfg.DefaultValue("bar"))
	assert.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		callbackCalled = true
		return nil
	})
	assert.NoError(err)

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Equal("bar", con.Config("foo"))
	assert.False(callbackCalled)
}

func TestConnector_CallbackWhenStarted(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Connector
	con := New("callback.when.started.config.connector")
	err := con.DefineConfig("foo", cfg.DefaultValue("bar"))
	assert.NoError(err)
	callbackCalled := 0
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		callbackCalled++
		assert.True(changed("foo"))
		return nil
	})
	assert.NoError(err)

	con.SetConfig("foo", "baz")
	assert.Equal("baz", con.Config("foo"))
	assert.Zero(callbackCalled)

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()
	assert.Zero(callbackCalled)

	con.SetConfig("foo", "bam")
	assert.Equal("bam", con.Config("foo"))
	assert.Equal(1, callbackCalled)
}

func TestConnector_CaseSensitiveConfig(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
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
	assert.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("case.sensitive.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)
	err = con.DefineConfig("foo-config_", cfg.DefaultValue("bar"))
	assert.NoError(err)
	callbackCalled := false
	err = con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		assert.True(changed("foo-config_"))
		assert.False(changed("FOO-CONFIG_"))
		callbackCalled = true
		return nil
	})
	assert.Expect(
		err, nil,
		con.Config("foo-config_"), configValue,
		con.Config("FOO-CONFIG_"), "",
	)

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.False(callbackCalled)

	configValue = "baz"
	_, err = mockCfg.GET(ctx, "https://case.sensitive.config.connector:888/config-refresh")
	assert.Expect(
		err, nil,
		callbackCalled, true,
		con.Config("foo-config_"), configValue,
		con.Config("FOO-CONFIG_"), "",
	)
}

func TestConnector_ReadFromFile(t *testing.T) {
	// No parallel
	assert := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	os.Chdir("testdata/subdir")
	defer os.Chdir("..")
	defer os.Chdir("..")

	// Mock a config service
	mockCfg := New("configurator.core")
	mockCfg.SetDeployment(LAB) // Configs are disabled in TESTING
	mockCfg.SetPlane(plane)
	mockCfg.Subscribe("POST", ":888/values", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"values":{"Provider":"Configurator"}}`))
		return nil
	})

	err := mockCfg.Startup()
	assert.NoError(err)
	defer mockCfg.Shutdown()

	// Connector
	con := New("read.from.file.config.connector")
	con.SetDeployment(LAB) // Configs are disabled in TESTING
	con.SetPlane(plane)
	err = con.DefineConfig("SubDir")
	assert.NoError(err)
	err = con.DefineConfig("case")
	assert.NoError(err)
	err = con.DefineConfig("CASE")
	assert.NoError(err)
	err = con.DefineConfig("Domain")
	assert.NoError(err)
	err = con.DefineConfig("Provider")
	assert.NoError(err)
	err = con.DefineConfig("Empty")
	assert.NoError(err)

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Expect(
		con.Config("SubDir"), "Child Subdomain",
		con.Config("case"), "lowercase",
		con.Config("CASE"), "UPPERCASE",
		con.Config("Domain"), "Subdomain",
		con.Config("Provider"), "Configurator",
		con.Config("Empty"), "",
		con.Config("Undefined"), "",
	)
}

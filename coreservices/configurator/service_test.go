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

package configurator

import (
	"context"
	"sync"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/testarossa"
)

func TestConfigurator_ManyMicroservices(t *testing.T) {
	// No parallel
	env.Push("MICROBUS_PLANE", rand.AlphaNum64(12))
	defer env.Pop("MICROBUS_PLANE")
	env.Push("MICROBUS_DEPLOYMENT", connector.LAB)
	defer env.Pop("MICROBUS_DEPLOYMENT")

	assert := testarossa.For(t)

	configSvc := NewService()
	services := []service.Service{}
	n := 16
	var wg sync.WaitGroup
	for range n {
		con := connector.New("many.microservices.configurator")
		con.DefineConfig("foo", cfg.DefaultValue("bar"))
		con.DefineConfig("moo")
		con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
			con.LogDebug(ctx, "Config changed",
				"foo", con.Config("foo"),
			)
			wg.Done()
			return nil
		})
		services = append(services, con)
	}

	app := application.New()
	app.Add(configSvc)
	app.Add(services...)
	err := app.Startup()
	assert.NoError(err)
	defer app.Shutdown()

	for i := 1; i < len(services); i++ {
		assert.Equal("bar", services[i].(*connector.Connector).Config("foo"))
		assert.Equal("", services[i].(*connector.Connector).Config("moo"))
	}

	// Load new values
	err = configSvc.loadYAML(`
many.microservices.configurator:
  foo: baz
  moo: cow
`)
	assert.NoError(err)

	wg.Add(n)
	err = configSvc.Refresh(configSvc.Lifetime())
	assert.NoError(err)
	wg.Wait()

	for i := range services {
		assert.Equal("baz", services[i].(*connector.Connector).Config("foo"))
		assert.Equal("cow", services[i].(*connector.Connector).Config("moo"))
	}

	// Restore foo to use the default value
	err = configSvc.loadYAML(`
many.microservices.configurator:
  foo:
  moo: cow
`)
	assert.NoError(err)

	wg.Add(n)
	err = configSvc.Refresh(configSvc.Lifetime())
	assert.NoError(err)
	wg.Wait()

	for i := range services {
		assert.Equal("bar", services[i].(*connector.Connector).Config("foo"))
		assert.Equal("cow", services[i].(*connector.Connector).Config("moo"))
	}
}

func TestConfigurator_Callback(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	configSvc := NewService()
	configSvc.SetDeployment(connector.LAB)
	configSvc.SetPlane(plane)

	con := connector.New("callback.configurator")
	con.SetDeployment(connector.LAB)
	con.SetPlane(plane)
	con.DefineConfig("foo", cfg.DefaultValue("bar"))
	var wg sync.WaitGroup
	err := con.SetOnConfigChanged(func(ctx context.Context, changed func(string) bool) error {
		assert.True(changed("foo"))
		wg.Done()
		return nil
	})
	assert.NoError(err)

	err = configSvc.Startup()
	assert.NoError(err)
	defer configSvc.Shutdown()
	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Equal("bar", con.Config("foo"))

	configSvc.loadYAML(`
callback.configurator:
  foo: baz
`)

	// Force a refresh
	wg.Add(1)
	err = configSvc.Refresh(configSvc.Lifetime())
	assert.NoError(err)
	wg.Wait()

	assert.Equal("baz", con.Config("foo"))
}

func TestConfigurator_PeerSync(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	// Start the first peer
	config1 := NewService()
	config1.SetDeployment(connector.LAB)
	config1.SetPlane(plane)
	config1.loadYAML(`
www.example.com:
  Foo: Bar
`)
	err := config1.Startup()
	assert.NoError(err)
	defer config1.Shutdown()

	val, ok := config1.repo.Value("www.example.com", "Foo")
	assert.True(ok)
	assert.Equal("Bar", val)

	// Start the microservice
	con := connector.New("www.example.com")
	con.SetDeployment(connector.LAB)
	con.SetPlane(plane)
	con.DefineConfig("Foo")

	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Equal("Bar", con.Config("Foo"))

	// Start the second peer
	config2 := NewService()
	config2.SetDeployment(connector.LAB)
	config2.SetPlane(plane)
	config2.loadYAML(`
www.example.com:
  Foo: Baz
`)
	err = config2.Startup()
	assert.NoError(err)
	defer config2.Shutdown()

	val, ok = config2.repo.Value("www.example.com", "Foo")
	assert.True(ok)
	assert.Equal("Baz", val)

	val, ok = config1.repo.Value("www.example.com", "Foo")
	assert.True(ok)
	assert.Equal("Baz", val, "First peer should have been updated")

	assert.Equal("Baz", con.Config("Foo"), "Microservice should have been updated")
}

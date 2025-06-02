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
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/testarossa"
)

func TestConfigurator_ManyMicroservices(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	plane := rand.AlphaNum64(12)

	configSvc := NewService()
	configSvc.SetDeployment(connector.LAB)
	configSvc.SetPlane(plane)
	services := []service.Service{}
	n := 16
	var wg sync.WaitGroup
	for range n {
		con := connector.New("many.microservices.configurator")
		con.SetDeployment(connector.LAB)
		con.SetPlane(plane)
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
	tt.NoError(err)
	defer app.Shutdown()

	for i := 1; i < len(services); i++ {
		tt.Equal("bar", services[i].(*connector.Connector).Config("foo"))
		tt.Equal("", services[i].(*connector.Connector).Config("moo"))
	}

	// Load new values
	err = configSvc.loadYAML(`
many.microservices.configurator:
  foo: baz
  moo: cow
`)
	tt.NoError(err)

	wg.Add(n)
	err = configSvc.Refresh(configSvc.Lifetime())
	tt.NoError(err)
	wg.Wait()

	for i := range services {
		tt.Equal("baz", services[i].(*connector.Connector).Config("foo"))
		tt.Equal("cow", services[i].(*connector.Connector).Config("moo"))
	}

	// Restore foo to use the default value
	err = configSvc.loadYAML(`
many.microservices.configurator:
  foo:
  moo: cow
`)
	tt.NoError(err)

	wg.Add(n)
	err = configSvc.Refresh(configSvc.Lifetime())
	tt.NoError(err)
	wg.Wait()

	for i := range services {
		tt.Equal("bar", services[i].(*connector.Connector).Config("foo"))
		tt.Equal("cow", services[i].(*connector.Connector).Config("moo"))
	}
}

func TestConfigurator_Callback(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

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
		tt.True(changed("foo"))
		wg.Done()
		return nil
	})
	tt.NoError(err)

	err = configSvc.Startup()
	tt.NoError(err)
	defer configSvc.Shutdown()
	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal("bar", con.Config("foo"))

	configSvc.loadYAML(`
callback.configurator:
  foo: baz
`)

	// Force a refresh
	wg.Add(1)
	err = configSvc.Refresh(configSvc.Lifetime())
	tt.NoError(err)
	wg.Wait()

	tt.Equal("baz", con.Config("foo"))
}

func TestConfigurator_PeerSync(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

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
	tt.NoError(err)
	defer config1.Shutdown()

	val, ok := config1.repo.Value("www.example.com", "Foo")
	tt.True(ok)
	tt.Equal("Bar", val)

	// Start the microservice
	con := connector.New("www.example.com")
	con.SetDeployment(connector.LAB)
	con.SetPlane(plane)
	con.DefineConfig("Foo")

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal("Bar", con.Config("Foo"))

	// Start the second peer
	config2 := NewService()
	config2.SetDeployment(connector.LAB)
	config2.SetPlane(plane)
	config2.loadYAML(`
www.example.com:
  Foo: Baz
`)
	err = config2.Startup()
	tt.NoError(err)
	defer config2.Shutdown()

	val, ok = config2.repo.Value("www.example.com", "Foo")
	tt.True(ok)
	tt.Equal("Baz", val)

	val, ok = config1.repo.Value("www.example.com", "Foo")
	tt.True(ok)
	tt.Equal("Baz", val, "First peer should have been updated")

	tt.Equal("Baz", con.Config("Foo"), "Microservice should have been updated")
}

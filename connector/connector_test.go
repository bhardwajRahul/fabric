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
	"regexp"
	"testing"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestConnector_HostAndID(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := NewConnector()
	tt.Equal("", con.Hostname())
	tt.NotEqual("", con.ID())
	con.SetHostname("example.com")
	tt.Equal("example.com", con.Hostname())
}

func TestConnector_BadHostname(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	con := NewConnector()
	badHosts := []string{
		"$.example.com",
		"my!example.com",
		"example..com",
		"example.com.",
		".example.com",
		".",
		"",
	}
	for _, s := range badHosts {
		err := con.SetHostname(s)
		tt.Error(err)
	}
}

func TestConnector_Plane(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	randomPlane := rand.AlphaNum64(12)

	// Before starting
	con := New("plane.connector")
	tt.Equal("", con.Plane())
	err := con.SetPlane("bad.plane.name")
	tt.Error(err)
	err = con.SetPlane(randomPlane)
	tt.NoError(err)
	tt.Equal(randomPlane, con.Plane())
	err = con.SetPlane("")
	tt.NoError(err)
	tt.Equal("", con.Plane())

	// After starting
	con = New("plane.connector")
	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()
	tt.NotEqual("", con.Plane())
	tt.NotEqual("microbus", con.Plane())
	tt.True(regexp.MustCompile(`\w{12,}`).MatchString(con.Plane())) // Hash of test name
	err = con.SetPlane("123plane456")
	tt.Error(err)
}

func TestConnector_PlaneEnv(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	// Bad plane name
	env.Push("MICROBUS_PLANE", "bad.plane.name")
	defer env.Pop("MICROBUS_PLANE")

	con := New("plane.env.connector")
	err := con.Startup()
	tt.Error(err)

	// Good plane name
	randomPlane := rand.AlphaNum64(12)
	env.Push("MICROBUS_PLANE", randomPlane)
	defer env.Pop("MICROBUS_PLANE")

	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal(randomPlane, con.Plane())
}

func TestConnector_Deployment(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Before starting
	con := New("deployment.connector")
	tt.Equal("", con.Deployment())
	err := con.SetDeployment("NOGOOD")
	tt.Error(err)
	err = con.SetDeployment("lAb")
	tt.NoError(err)
	tt.Equal(LAB, con.Deployment())
	err = con.SetDeployment("")
	tt.NoError(err)
	tt.Equal("", con.Deployment())

	// After starting
	con = New("deployment.connector")
	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()
	tt.Equal(TESTING, con.Deployment())
	err = con.SetDeployment(LAB)
	tt.Error(err)
}

func TestConnector_DeploymentEnv(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	con := New("deployment.env.connector")

	env.Push("MICROBUS_DEPLOYMENT", "lAb")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	err := con.Startup()
	tt.NoError(err)
	defer con.Shutdown()

	tt.Equal(LAB, con.Deployment())
}

func TestConnector_Version(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Before starting
	con := New("version.connector")
	err := con.SetVersion(-1)
	tt.Error(err)
	err = con.SetVersion(123)
	tt.NoError(err)
	tt.Equal(123, con.Version())
	err = con.SetVersion(0)
	tt.NoError(err)
	tt.Zero(con.Version())

	// After starting
	con = New("version.connector")
	err = con.Startup()
	tt.NoError(err)
	defer con.Shutdown()
	err = con.SetVersion(123)
	tt.Error(err)
}

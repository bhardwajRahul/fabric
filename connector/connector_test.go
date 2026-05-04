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
	"net/http"
	"regexp"
	"testing"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/testarossa"
)

func TestConnector_HostAndID(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := NewConnector()
	assert.Equal("", con.Hostname())
	assert.NotEqual("", con.ID())
	con.SetHostname("example.com")
	assert.Equal("example.com", con.Hostname())
}

func TestConnector_BadHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

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
		assert.Error(err)
	}
}

func TestConnector_Plane(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	randomPlane := utils.RandomIdentifier(12)

	// Before starting
	con := New("plane.connector")
	assert.Equal("", con.Plane())
	err := con.SetPlane("bad.plane.name")
	assert.Error(err)
	err = con.SetPlane(randomPlane)
	assert.NoError(err)
	assert.Equal(randomPlane, con.Plane())
	err = con.SetPlane("")
	assert.NoError(err)
	assert.Equal("", con.Plane())

	// After starting
	con = New("plane.connector")
	err = con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)
	assert.NotEqual("", con.Plane())
	assert.NotEqual("microbus", con.Plane())
	assert.True(regexp.MustCompile(`\w{12,}`).MatchString(con.Plane())) // Hash of test name
	err = con.SetPlane("123plane456")
	assert.Error(err)
}

func TestConnector_PlaneEnv(t *testing.T) {
	// No parallel - Setting envars
	ctx := t.Context()
	assert := testarossa.For(t)

	// Bad plane name
	env.Push("MICROBUS_PLANE", "bad.plane.name")
	defer env.Pop("MICROBUS_PLANE")

	con := New("plane.env.connector")
	err := con.Startup(ctx)
	assert.Error(err)

	// Good plane name
	randomPlane := utils.RandomIdentifier(12)
	env.Push("MICROBUS_PLANE", randomPlane)
	defer env.Pop("MICROBUS_PLANE")

	err = con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	assert.Equal(randomPlane, con.Plane())
}

func TestConnector_Deployment(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Before starting
	con := New("deployment.connector")
	assert.Equal("", con.Deployment())
	err := con.SetDeployment("NOGOOD")
	assert.Error(err)
	err = con.SetDeployment("lAb")
	assert.NoError(err)
	assert.Equal(LAB, con.Deployment())
	err = con.SetDeployment("")
	assert.NoError(err)
	assert.Equal("", con.Deployment())

	// After starting
	con = New("deployment.connector")
	err = con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)
	assert.Equal(TESTING, con.Deployment())
	err = con.SetDeployment(LAB)
	assert.Error(err)
}

func TestConnector_DeploymentEnv(t *testing.T) {
	// No parallel - Setting envars
	ctx := t.Context()
	assert := testarossa.For(t)

	con := New("deployment.env.connector")

	env.Push("MICROBUS_DEPLOYMENT", "lAb")
	defer env.Pop("MICROBUS_DEPLOYMENT")

	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	assert.Equal(LAB, con.Deployment())
}

func TestConnector_Version(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Before starting
	con := New("version.connector")
	err := con.SetVersion(-1)
	assert.Error(err)
	err = con.SetVersion(123)
	assert.NoError(err)
	assert.Equal(123, con.Version())
	err = con.SetVersion(0)
	assert.NoError(err)
	assert.Zero(con.Version())

	// After starting
	con = New("version.connector")
	err = con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)
	err = con.SetVersion(123)
	assert.Error(err)
}

func TestConnector_ExternalizeURL(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := NewConnector()
	con.hostname = "my.service"

	// With full X-Forwarded headers (proto + host + prefix)
	h := http.Header{}
	h.Set("X-Forwarded-Proto", "https")
	h.Set("X-Forwarded-Host", "www.example.com")
	h.Set("X-Forwarded-Prefix", "/app")
	ctx := frame.ContextWithFrameOf(t.Context(), h)

	// URL with scheme - remove scheme and prepend with base URL
	assert.Equal(
		"https://www.example.com/app/other.service/endpoint",
		con.ExternalizeURL(ctx, "https://other.service/endpoint"),
	)
	assert.Equal(
		"https://www.example.com/app/other.service/endpoint",
		con.ExternalizeURL(ctx, "http://other.service/endpoint"),
	)

	// URL starting with // - prepend with base URL
	assert.Equal(
		"https://www.example.com/app/other.service/path",
		con.ExternalizeURL(ctx, "//other.service/path"),
	)

	// URL starting with / - prepend with base URL and hostname
	assert.Equal(
		"https://www.example.com/app/my.service/some/resource",
		con.ExternalizeURL(ctx, "/some/resource"),
	)

	// Other URL - return as-is
	assert.Equal(
		"relative/path",
		con.ExternalizeURL(ctx, "relative/path"),
	)
	assert.Equal(
		"",
		con.ExternalizeURL(ctx, ""),
	)

	// Without X-Forwarded headers - base URL is empty, xf becomes "/"
	ctx2 := frame.ContextWithFrameOf(t.Context(), http.Header{})
	assert.Equal(
		"/other.service/endpoint",
		con.ExternalizeURL(ctx2, "https://other.service/endpoint"),
	)
	assert.Equal(
		"/other.service/endpoint",
		con.ExternalizeURL(ctx2, "//other.service/endpoint"),
	)
	assert.Equal(
		"/my.service/some/resource",
		con.ExternalizeURL(ctx2, "/some/resource"),
	)

	// Without prefix
	h2 := http.Header{}
	h2.Set("X-Forwarded-Proto", "http")
	h2.Set("X-Forwarded-Host", "proxy.local")
	ctx3 := frame.ContextWithFrameOf(t.Context(), h2)
	assert.Equal(
		"http://proxy.local/service.host/api",
		con.ExternalizeURL(ctx3, "https://service.host/api"),
	)
	assert.Equal(
		"http://proxy.local/service.host/api",
		con.ExternalizeURL(ctx3, "//service.host/api"),
	)
	assert.Equal(
		"http://proxy.local/my.service/api",
		con.ExternalizeURL(ctx3, "/api"),
	)

	// Host only, no proto
	h3 := http.Header{}
	h3.Set("X-Forwarded-Host", "proxy.local")
	h3.Set("X-Forwarded-Prefix", "/base")
	ctx4 := frame.ContextWithFrameOf(t.Context(), h3)
	assert.Equal(
		"//proxy.local/base/service.host/api",
		con.ExternalizeURL(ctx4, "https://service.host/api"),
	)
	assert.Equal(
		"//proxy.local/base/service.host/api",
		con.ExternalizeURL(ctx4, "//service.host/api"),
	)
	assert.Equal(
		"//proxy.local/base/my.service/api",
		con.ExternalizeURL(ctx4, "/api"),
	)

	// Prefix only, no host or proto
	h4 := http.Header{}
	h4.Set("X-Forwarded-Prefix", "/base")
	ctx5 := frame.ContextWithFrameOf(t.Context(), h4)
	assert.Equal(
		"/base/service.host/api",
		con.ExternalizeURL(ctx5, "https://service.host/api"),
	)
	assert.Equal(
		"/base/service.host/api",
		con.ExternalizeURL(ctx5, "//service.host/api"),
	)
	assert.Equal(
		"/base/my.service/api",
		con.ExternalizeURL(ctx5, "/api"),
	)
}

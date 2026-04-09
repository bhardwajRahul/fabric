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
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func TestConnector_SummarizeSubscription(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type WebOnly struct{}
	type CapitalIn struct {
		X int    `json:"x,omitzero"`
		Y Point  `json:"y,omitzero"`
		Z string `json:"-"`
	}
	type CapitalOut struct {
		Z map[string]Point `json:"z,omitzero"`
	}
	type CompositeOut struct {
		Pts        []*Point         `json:"pts,omitzero"`
		ByName     map[string]Point `json:"byName,omitzero"`
		PtrToInner *Point           `json:"ptrToInner,omitzero"`
	}
	type ExternalErr struct {
		OnErr *errors.TracedError `json:"onErr,omitzero"`
	}
	type FallbackName struct {
		FooBar int
	}

	cases := []struct {
		name    string
		inputs  any
		outputs any
		want    string
	}{
		{"NoArgs", nil, nil, "NoArgs()"},
		{"WebHandler", WebOnly{}, WebOnly{}, "WebHandler()"},
		{"Compute", CapitalIn{}, CapitalOut{}, "Compute(x int, y Point) (z map[string]Point)"},
		{"InOnly", CapitalIn{}, nil, "InOnly(x int, y Point)"},
		{"OutOnly", nil, CapitalOut{}, "OutOnly() (z map[string]Point)"},
		{"Composite", nil, CompositeOut{}, "Composite() (pts []*Point, byName map[string]Point, ptrToInner *Point)"},
		{"HandleErr", ExternalErr{}, nil, "HandleErr(onErr *TracedError)"},
		{"Fallback", FallbackName{}, nil, "Fallback(fooBar int)"},
	}
	for _, tc := range cases {
		got := summarizeSubscription(&sub.Subscription{
			Name:    tc.name,
			Inputs:  tc.inputs,
			Outputs: tc.outputs,
		})
		assert.Equal(tc.want, got)
	}
}

func TestConnector_OpenAPI(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	ctx := t.Context()

	con := New("openapi.connector")
	con.SetDescription("Test connector for OpenAPI control endpoint")
	con.SetVersion(7)

	noopHandler := func(w http.ResponseWriter, r *http.Request) error { return nil }
	type emptyIn struct{}
	type emptyOut struct{}

	assert.NoError(con.Subscribe("PublicWeb", noopHandler, sub.Web()))
	assert.NoError(con.Subscribe("PublicFunction", noopHandler,
		sub.Function(emptyIn{}, emptyOut{}),
		sub.Description("PublicFunction does something"),
	))
	assert.NoError(con.Subscribe("PublicWorkflow", noopHandler, sub.Workflow(emptyIn{}, emptyOut{})))
	assert.NoError(con.Subscribe("HiddenTask", noopHandler, sub.Task(emptyIn{}, emptyOut{})))
	assert.NoError(con.Subscribe("HiddenInbound", noopHandler,
		sub.InboundEvent(emptyIn{}, emptyOut{}),
		sub.Route("https://other.host:417/hidden-inbound"),
	))
	assert.NoError(con.Subscribe("AdminOnly", noopHandler,
		sub.Web(),
		sub.RequiredClaims(`roles.admin`),
	))

	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// /openapi.json now lives only on :888. The returned doc covers endpoints across all
	// ports - port-based filtering is the consumer's job.
	fetch := func(opts ...pub.Option) map[string]map[string]any {
		opts = append([]pub.Option{pub.GET("https://openapi.connector:888/openapi.json")}, opts...)
		res, err := con.Request(ctx, opts...)
		assert.NoError(err)
		assert.Equal("private, no-store", res.Header.Get("Cache-Control"))
		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		var doc struct {
			Paths map[string]map[string]any `json:"paths"`
		}
		assert.NoError(json.Unmarshal(body, &doc))
		return doc.Paths
	}

	t.Run("anonymous", func(t *testing.T) {
		assert := testarossa.For(t)
		paths := fetch()
		// Function/Web/Workflow appear at their declared ports.
		_, ok := paths["/openapi.connector:443/public-web"]
		assert.True(ok)
		_, ok = paths["/openapi.connector:443/public-function"]
		assert.True(ok)
		_, ok = paths["/openapi.connector:428/public-workflow"]
		assert.True(ok)
		// Task and inbound-event types are still excluded.
		_, ok = paths["/openapi.connector:428/hidden-task"]
		assert.False(ok)
		_, ok = paths["/other.host:417/hidden-inbound"]
		assert.False(ok)
		// Admin-only sub has RequiredClaims and is hidden from anonymous callers.
		_, ok = paths["/openapi.connector:443/admin-only"]
		assert.False(ok)
		// The /openapi.json handler itself is on :888; the //all mirror is filtered.
		_, ok = paths["/openapi.connector:888/openapi.json"]
		assert.True(ok)
		_, ok = paths["/all:888/openapi.json"]
		assert.False(ok)
	})

	t.Run("with_admin_actor", func(t *testing.T) {
		assert := testarossa.For(t)
		// pub.Actor mints an unsigned JWT - accepted in TESTING but claims are still
		// evaluated against each sub's RequiredClaims. The admin-only sub should appear
		// in the doc for this caller.
		paths := fetch(pub.Actor(jwt.MapClaims{
			"roles": map[string]any{"admin": true},
		}))
		_, ok := paths["/openapi.connector:443/admin-only"]
		assert.True(ok)
	})

	t.Run("with_non_matching_actor", func(t *testing.T) {
		assert := testarossa.For(t)
		// A token without the required claim must not surface the gated sub.
		paths := fetch(pub.Actor(jwt.MapClaims{
			"roles": map[string]any{"viewer": true},
		}))
		_, ok := paths["/openapi.connector:443/admin-only"]
		assert.False(ok)
	})
}

func TestConnector_Ping(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("ping.connector")

	// Startup the microservice
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages
	for r := range con.Publish(ctx, pub.GET("https://ping.connector:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://"+con.id+".ping.connector:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://all:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://"+con.id+".all:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
}

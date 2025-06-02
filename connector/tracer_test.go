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
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/testarossa"
)

func TestConnector_TraceRequestAttributes(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.test.request.attributes.connector")

	var span trc.Span
	beta := New("beta.test.request.attributes.connector")
	beta.Subscribe("GET", "handle", func(w http.ResponseWriter, r *http.Request) error {
		span = beta.Span(r.Context())

		// The request attributes should not be added until and unless there's an error
		attributes := span.Attributes()
		tt.Zero(len(attributes["http.method"]))
		tt.Zero(len(attributes["url.scheme"]))
		tt.Zero(len(attributes["server.address"]))
		tt.Zero(len(attributes["server.port"]))
		tt.Zero(len(attributes["url.path"]))

		tt.Equal(0, span.Status())

		if r.URL.Query().Get("err") != "" {
			return errors.New("oops")
		}
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// A request that returns with an error
	_, err = alpha.GET(ctx, "https://beta.test.request.attributes.connector/handle?err=1")
	if tt.Error(err) {
		// The request attributes should be added since there was an error
		attributes := span.Attributes()
		tt.Equal("GET", attributes["http.method"])
		tt.Equal("https", attributes["url.scheme"])
		tt.Equal("beta.test.request.attributes.connector", attributes["server.address"])
		tt.Equal("443", attributes["server.port"])
		tt.Equal("/handle", attributes["url.path"])

		tt.Equal(1, span.Status())
	}

	// A request that returns OK
	_, err = alpha.GET(ctx, "https://beta.test.request.attributes.connector/handle")
	if tt.NoError(err) {
		// The request attributes should not be added since there was no error
		attributes := span.Attributes()
		tt.Zero(len(attributes["http.method"]))
		tt.Zero(len(attributes["url.scheme"]))
		tt.Zero(len(attributes["server.address"]))
		tt.Zero(len(attributes["server.port"]))
		tt.Zero(len(attributes["url.path"]))

		tt.Equal(2, span.Status())
	}
}

func TestConnector_GoTracingSpan(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	alpha := New("go.tracing.span.connector")
	var topSpan trc.Span
	var goSpan trc.Span
	var wg sync.WaitGroup
	wg.Add(1)
	alpha.SetOnStartup(func(ctx context.Context) error {
		topSpan = alpha.Span(ctx)
		alpha.Go(ctx, func(ctx context.Context) (err error) {
			goSpan = alpha.Span(ctx)
			wg.Done()
			return nil
		})
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()

	wg.Wait()
	tt.Equal(topSpan.TraceID(), goSpan.TraceID())
}

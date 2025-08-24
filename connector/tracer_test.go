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
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/testarossa"
)

func TestConnector_TraceRequestAttributes(t *testing.T) {
	// No parallel
	env.Push("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "nil")
	defer env.Pop("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")

	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.test.request.attributes.connector")

	var span trc.Span
	beta := New("beta.test.request.attributes.connector")
	beta.Subscribe("GET", "handle", func(w http.ResponseWriter, r *http.Request) error {
		span = beta.Span(r.Context())

		// The request attributes should not be added until and unless there's an error
		attributes := spanAttributes(span)
		tt.Zero(len(attributes["http.method"]))
		tt.Zero(len(attributes["url.scheme"]))
		tt.Zero(len(attributes["server.address"]))
		tt.Zero(len(attributes["server.port"]))
		tt.Zero(len(attributes["url.path"]))

		tt.Equal(0, spanStatus(span))

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
		attributes := spanAttributes(span)
		tt.Equal("GET", attributes["http.method"])
		tt.Equal("https", attributes["url.scheme"])
		tt.Equal("beta.test.request.attributes.connector", attributes["server.address"])
		tt.Equal("443", attributes["server.port"])
		tt.Equal("/handle", attributes["url.path"])

		tt.Equal(1, spanStatus(span))
	}

	// A request that returns OK
	_, err = alpha.GET(ctx, "https://beta.test.request.attributes.connector/handle")
	if tt.NoError(err) {
		// The request attributes should not be added since there was no error
		attributes := spanAttributes(span)
		tt.Zero(len(attributes["http.method"]))
		tt.Zero(len(attributes["url.scheme"]))
		tt.Zero(len(attributes["server.address"]))
		tt.Zero(len(attributes["server.port"]))
		tt.Zero(len(attributes["url.path"]))

		tt.Equal(2, spanStatus(span))
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

// spanAttributes returns the attributes set on the span.
func spanAttributes(s trc.Span) map[string]string {
	m := map[string]string{}
	attributes := reflect.ValueOf(s).FieldByName("internal").Elem().Elem().FieldByName("attributes")
	for i := range attributes.Len() {
		k := attributes.Index(i).FieldByName("Key").String()
		v := attributes.Index(i).FieldByName("Value").FieldByName("stringly").String()
		if v == "" {
			i := attributes.Index(i).FieldByName("Value").FieldByName("numeric").Uint()
			if i != 0 {
				v = strconv.Itoa(int(i))
			}
		}
		if v == "" {
			slice := attributes.Index(i).FieldByName("Value").FieldByName("slice").Elem()
			if slice.Len() == 1 {
				v = slice.Index(0).String()
			}
		}
		m[k] = v
	}
	return m
}

// Status returns the status of the span: 0=unset; 1=error; 2=OK.
func spanStatus(s trc.Span) int {
	status := reflect.ValueOf(s).FieldByName("internal").Elem().Elem().FieldByName("status")
	return int(status.FieldByName("Code").Uint())
}

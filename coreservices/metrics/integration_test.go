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

package metrics

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
	"github.com/microbus-io/fabric/env"
)

var (
	_ *testing.T
	_ testarossa.TestingT
	_ *metricsapi.Client
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	// Add microservices to the testing app
	err = App.AddAndStartup(
		Svc,
	)
	if err != nil {
		return err
	}
	return nil
}

// Terminate gets called after the testing app shut down.
func Terminate() (err error) {
	return nil
}

func TestMetrics_Collect(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)
	ctx := Context()

	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	defer env.Pop("MICROBUS_PROMETHEUS_EXPORTER")

	Collect_Get(t, ctx, "").
		BodyNotContains("metrics.core"). // metrics.core was started before MICROBUS_PROMETHEUS_EXPORTER was set
		BodyNotContains("one.collect").
		BodyNotContains("two.collect")

	// Join two new services
	con1 := connector.New("one.collect")
	con1.SetOnStartup(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	con1.Subscribe("GET", "/ten", func(w http.ResponseWriter, r *http.Request) error {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("1234567890"))
		return nil
	})
	con2 := connector.New("two.collect")

	err := App.AddAndStartup(con1, con2)
	tt.NoError(err)
	defer con1.Shutdown()
	defer con2.Shutdown()

	// Make a request to the service
	_, err = con1.GET(ctx, "https://one.collect/ten")
	tt.NoError(err)

	// Interact with the cache
	con1.DistribCache().Store(ctx, "A", []byte("1234567890"))
	con1.DistribCache().Load(ctx, "A")
	con1.DistribCache().Load(ctx, "B")

	// Loop until the new services are discovered
	for range 10 {
		tc := Collect_Get(t, ctx, "")
		res, err := tc.Get()
		tt.NoError(err)
		body, err := io.ReadAll(res.Body)
		tt.NoError(err)
		if bytes.Contains(body, []byte("one.collect")) &&
			bytes.Contains(body, []byte("two.collect")) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	re := func(name string, value string, attrs ...string) string {
		var sb strings.Builder
		if name != "" {
			sb.WriteString(regexp.QuoteMeta(name))
			sb.WriteString(regexp.QuoteMeta("{"))
		}
		for i := 0; i < len(attrs); i += 2 {
			sb.WriteString(".*")
			sb.WriteString(regexp.QuoteMeta(attrs[i]))
			sb.WriteString(regexp.QuoteMeta(`="`))
			sb.WriteString(regexp.QuoteMeta(attrs[i+1]))
			sb.WriteString(regexp.QuoteMeta(`"`))
		}
		sb.WriteString(".*")
		if value != "" {
			sb.WriteString(regexp.QuoteMeta("} "))
			sb.WriteString(regexp.QuoteMeta(value))
		}
		return sb.String()
	}

	Collect_Get(t, ctx, "").
		// The two services should be detected
		BodyNotContains("metrics.core"). // metrics.core was started before MICROBUS_PROMETHEUS_EXPORTER was set
		BodyContains("one.collect").
		BodyContains("two.collect").
		// The startup callback should take between 100ms and 500ms
		BodyMatchesRegexp(re("microbus_callback_duration_seconds_bucket", "0",
			"error", "OK",
			"handler", "startup",
			"id", con1.ID(),
			"service", "one.collect",
			"le", "0.1",
		)).
		BodyMatchesRegexp(re("microbus_callback_duration_seconds_bucket", "1",
			"error", "OK",
			"handler", "startup",
			"id", con1.ID(),
			"service", "one.collect",
			"le", "0.5",
		)).
		BodyMatchesRegexp(re("microbus_log_messages_total", "1",
			"id", con1.ID(),
			"message", "Startup",
			"service", "one.collect",
			"severity", "INFO",
		)).
		BodyMatchesRegexp(re("microbus_uptime_duration_seconds", "",
			"id", con1.ID(),
			"service", "one.collect",
		)).
		// Cache should have 1 element of 10 bytes
		BodyMatchesRegexp(re("microbus_cache_memory_bytes", "10",
			"id", con1.ID(),
			"service", "one.collect",
		)).
		BodyMatchesRegexp(re("microbus_cache_elements", "1",
			"id", con1.ID(),
			"service", "one.collect",
		)).
		BodyMatchesRegexp(re("microbus_cache_operations_total", "1",
			"hit", "miss",
			"id", con1.ID(),
			"op", "load",
			"service", "one.collect",
		)).
		BodyMatchesRegexp(re("microbus_cache_operations_total", "1",
			"hit", "local",
			"id", con1.ID(),
			"op", "load",
			"service", "one.collect",
		)).
		BodyMatchesRegexp(re("microbus_server_request_duration_seconds_count", "2",
			"code", "404",
			"error", "OK",
			"handler", "one.collect:888/dcache/all",
			"id", con1.ID(),
			"method", "GET",
			"port", "888",
			"service", "one.collect",
		)).
		// The response size is 10 bytes
		BodyMatchesRegexp(re("microbus_server_response_body_bytes_sum", "10",
			"code", "200",
			"error", "OK",
			"handler", "one.collect:443/ten",
			"id", con1.ID(),
			"method", "GET",
			"port", "443",
			"service", "one.collect",
		)).
		BodyMatchesRegexp(re("microbus_server_response_body_bytes_count", "1",
			"code", "200",
			"error", "OK",
			"handler", "one.collect:443/ten",
			"id", con1.ID(),
			"method", "GET",
			"port", "443",
			"service", "one.collect",
		)).
		// The request should take between 100ms and 500ms
		BodyMatchesRegexp(re("microbus_server_request_duration_seconds_bucket", "0",
			"code", "200",
			"error", "OK",
			"handler", "one.collect:443/ten",
			"id", con1.ID(),
			"method", "GET",
			"port", "443",
			"service", "one.collect",
			"le", "0.1",
		)).
		BodyMatchesRegexp(re("microbus_server_request_duration_seconds_bucket", "1",
			"code", "200",
			"error", "OK",
			"handler", "one.collect:443/ten",
			"id", con1.ID(),
			"method", "GET",
			"port", "443",
			"service", "one.collect",
			"le", "0.5",
		)).
		// Acks should be logged
		BodyContains("microbus_client_ack_roundtrip_latency_seconds_bucket")
}

func TestMetrics_GZip(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	defer env.Pop("MICROBUS_PROMETHEUS_EXPORTER")

	con := connector.New("gzip")
	err := App.AddAndStartup(con)
	tt.NoError(err)
	defer con.Shutdown()

	r, _ := http.NewRequest("GET", "", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	Collect(t, r).
		Assert(func(t *testing.T, res *http.Response, err error) {
			tt.NoError(err)
			tt.Equal("gzip", res.Header.Get("Content-Encoding"))
			unzipper, err := gzip.NewReader(res.Body)
			tt.NoError(err)
			body, err := io.ReadAll(unzipper)
			unzipper.Close()
			tt.NoError(err)
			tt.Contains(string(body), "microbus_log_messages")
		})
}

func TestMetrics_SecretKey(t *testing.T) {
	// No parallel
	ctx := Context()
	Svc.SetSecretKey("secret1234")
	Collect_Get(t, ctx, "").
		Error("incorrect secret key").
		ErrorCode(http.StatusNotFound)
	Svc.SetSecretKey("")
	Collect_Get(t, ctx, "").NoError()
}

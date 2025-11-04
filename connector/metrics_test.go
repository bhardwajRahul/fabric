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
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/testarossa"
)

func TestConnector_DefineMetrics(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("define.metrics.connector")
	assert.False(con.IsStarted())

	// Define all three collector types before starting up
	err := con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	assert.NoError(err)
	err = con.DescribeHistogram(
		"my_histogram",
		"my historgram",
		[]float64{1, 2, 3, 4, 5},
	)
	assert.NoError(err)
	err = con.DescribeGauge(
		"my_gauge",
		"my gauge",
	)
	assert.NoError(err)

	// Duplicate key
	err = con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	assert.Error(err)

	// Startup
	con.initErr = nil
	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	// Describe all three collector types after starting up
	err = con.DescribeCounter(
		"my_counter_2",
		"my counter 2",
	)
	assert.NoError(err)
	err = con.DescribeHistogram(
		"my_histogram_2",
		"my historgram 2",
		[]float64{1, 2, 3, 4, 5},
	)
	assert.NoError(err)
	err = con.DescribeGauge(
		"my_gauge_2",
		"my gauge 2",
	)
	assert.NoError(err)

	// Duplicate key
	err = con.DescribeCounter(
		"my_counter_2",
		"my counter 2",
	)
	assert.Error(err)
}

func TestConnector_ObserveMetrics(t *testing.T) {
	// No parallel
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	defer env.Pop("MICROBUS_PROMETHEUS_EXPORTER")

	assert := testarossa.For(t)
	ctx := t.Context()

	con := New("observe.metrics.connector")
	assert.False(con.IsStarted())

	// Define all three collector types before starting up
	err := con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	assert.NoError(err)
	err = con.DescribeHistogram(
		"my_histogram",
		"my histogram",
		[]float64{1, 2, 3, 4, 5},
	)
	assert.NoError(err)
	err = con.DescribeGauge(
		"my_gauge",
		"my gauge",
	)
	assert.NoError(err)

	// Startup
	con.initErr = nil
	err = con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	// Histogram
	err = con.RecordHistogram(ctx, "my_histogram", 2.5, "a", "1")
	assert.NoError(err)
	err = con.RecordHistogram(ctx, "my_histogram", 0, "a", "1")
	assert.NoError(err)
	err = con.RecordHistogram(ctx, "my_histogram", -2.5, "a", "1")
	assert.NoError(err)

	err = con.AddCounter(ctx, "my_histogram", 1.5, "a", "1")
	assert.Error(err)
	err = con.RecordGauge(ctx, "my_histogram", 1.5, "a", "1")
	assert.Error(err)

	// Gauge
	err = con.RecordGauge(ctx, "my_gauge", 2.5, "a", "1")
	assert.NoError(err)
	err = con.RecordGauge(ctx, "my_gauge", -2.5, "a", "1")
	assert.NoError(err)
	err = con.RecordGauge(ctx, "my_gauge", 0, "a", "1")
	assert.NoError(err)

	err = con.AddCounter(ctx, "my_gauge", 1.5, "a", "1")
	assert.Error(err)
	err = con.RecordHistogram(ctx, "my_gauge", 2.5, "a", "1")
	assert.Error(err)

	// Counter
	err = con.AddCounter(ctx, "my_counter", 1.5, "a", "1")
	assert.NoError(err)
	err = con.AddCounter(ctx, "my_counter", 0, "a", "1")
	assert.NoError(err)
	err = con.AddCounter(ctx, "my_counter", -1.5, "a", "1")
	assert.Error(err)

	err = con.RecordHistogram(ctx, "my_counter", 2.5, "a", "1")
	assert.Error(err)
	err = con.RecordGauge(ctx, "my_counter", 2.5, "a", "1")
	assert.Error(err)
}

func TestConnector_StandardMetrics(t *testing.T) {
	// No parallel
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	defer env.Pop("MICROBUS_PROMETHEUS_EXPORTER")

	assert := testarossa.For(t)

	con := New("standard.metrics.connector")
	assert.Len(con.metricInstruments, 0)

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	assert.Len(con.metricInstruments, 10)
	assert.NotNil(con.metricInstruments["microbus_callback_duration_seconds"])
	assert.NotNil(con.metricInstruments["microbus_server_request_duration_seconds"])
	assert.NotNil(con.metricInstruments["microbus_server_response_body_bytes"])
	assert.NotNil(con.metricInstruments["microbus_client_timeout_requests"])
	assert.NotNil(con.metricInstruments["microbus_client_ack_roundtrip_latency_seconds"])
	assert.NotNil(con.metricInstruments["microbus_log_messages"])
	assert.NotNil(con.metricInstruments["microbus_uptime_duration_seconds"])
	assert.NotNil(con.metricInstruments["microbus_cache_memory_bytes"])
	assert.NotNil(con.metricInstruments["microbus_cache_elements"])
	assert.NotNil(con.metricInstruments["microbus_cache_operations"])
}

func TestConnector_InferUnit(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("infer.unit.connector")
	type testCase struct {
		name string
		desc string
		unit string
	}
	testCases := []testCase{
		{"requests_byte_total", "Requests", "byte"},
		{"requests_byte", "Requests", "byte"},
		{"requests_bytes_total", "Requests", "bytes"},
		{"requests_bytes", "Requests", "bytes"},
		{"requests_megabyte_total", "Requests", "megabyte"},
		{"requests_total", "Requests [byte]", "byte"},
		{"requests_megabyte_total", "Requests [byte]", "byte"},
	}
	for _, tc := range testCases {
		unit := con.inferMetricUnit(tc.name, tc.desc)
		assert.Equal(tc.unit, unit, "Expected %s, got %s, for %s", tc.unit, unit, tc.name)
	}
}

func TestConnector_MetricExporters(t *testing.T) {
	// No parallel
	assert := testarossa.For(t)

	// Create the web server
	var counter atomic.Int32
	httpServer := &http.Server{
		Addr: ":5555",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}),
	}
	go httpServer.ListenAndServe()
	defer httpServer.Close()

	// Both Prometheus and OTel exporters
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	env.Push("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:5555")

	con := New("metric.exporters.connector")
	err := con.Startup()
	assert.NoError(err)
	err = con.Shutdown()
	assert.NoError(err)
	assert.Equal(2, int(counter.Load()))

	// Only Prometheus exporter
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "1")
	env.Push("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	con = New("metric.exporters.connector")
	err = con.Startup()
	assert.NoError(err)
	err = con.Shutdown()
	assert.NoError(err)

	// Only OTel exporter
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "0")
	env.Push("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:5555")

	con = New("metric.exporters.connector")
	err = con.Startup()
	assert.NoError(err)
	err = con.Shutdown()
	assert.NoError(err)
	assert.Equal(4, int(counter.Load()))

	// No exporters
	env.Push("MICROBUS_PROMETHEUS_EXPORTER", "0")
	env.Push("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	con = New("metric.exporters.connector")
	err = con.Startup()
	assert.NoError(err)
	err = con.Shutdown()
	assert.NoError(err)

	for range 4 {
		env.Pop("MICROBUS_PROMETHEUS_EXPORTER")
		env.Pop("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
}

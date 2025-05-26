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
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestConnector_DefineMetrics(t *testing.T) {
	t.Parallel()

	con := New("define.metrics.connector")
	testarossa.False(t, con.IsStarted())

	// Define all three collector types before starting up
	err := con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	testarossa.NoError(t, err)
	err = con.DescribeHistogram(
		"my_histogram",
		"my historgram",
		[]float64{1, 2, 3, 4, 5},
	)
	testarossa.NoError(t, err)
	err = con.DescribeGauge(
		"my_gauge",
		"my gauge",
	)
	testarossa.NoError(t, err)

	// Duplicate key
	err = con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	testarossa.Error(t, err)

	// Startup
	con.initErr = nil
	err = con.Startup()
	testarossa.NoError(t, err)
	defer con.Shutdown()

	// Describe all three collector types after starting up
	err = con.DescribeCounter(
		"my_counter_2",
		"my counter 2",
	)
	testarossa.NoError(t, err)
	err = con.DescribeHistogram(
		"my_histogram_2",
		"my historgram 2",
		[]float64{1, 2, 3, 4, 5},
	)
	testarossa.NoError(t, err)
	err = con.DescribeGauge(
		"my_gauge_2",
		"my gauge 2",
	)
	testarossa.NoError(t, err)

	// Duplicate key
	err = con.DescribeCounter(
		"my_counter_2",
		"my counter 2",
	)
	testarossa.Error(t, err)
}

func TestConnector_ObserveMetrics(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	con := New("observe.metrics.connector")
	testarossa.False(t, con.IsStarted())

	// Define all three collector types before starting up
	err := con.DescribeCounter(
		"my_counter",
		"my counter",
	)
	testarossa.NoError(t, err)
	err = con.DescribeHistogram(
		"my_histogram",
		"my histogram",
		[]float64{1, 2, 3, 4, 5},
	)
	testarossa.NoError(t, err)
	err = con.DescribeGauge(
		"my_gauge",
		"my gauge",
	)
	testarossa.NoError(t, err)

	// Startup
	con.initErr = nil
	err = con.Startup()
	testarossa.NoError(t, err)
	defer con.Shutdown()

	// Histogram
	err = con.RecordHistogram(ctx, "my_histogram", 2.5, "a", "1")
	testarossa.NoError(t, err)
	err = con.RecordHistogram(ctx, "my_histogram", 0, "a", "1")
	testarossa.NoError(t, err)
	err = con.RecordHistogram(ctx, "my_histogram", -2.5, "a", "1")
	testarossa.NoError(t, err)

	err = con.AddCounter(ctx, "my_histogram", 1.5, "a", "1")
	testarossa.Error(t, err)
	err = con.RecordGauge(ctx, "my_histogram", 1.5, "a", "1")
	testarossa.Error(t, err)

	// Gauge
	err = con.RecordGauge(ctx, "my_gauge", 2.5, "a", "1")
	testarossa.NoError(t, err)
	err = con.RecordGauge(ctx, "my_gauge", -2.5, "a", "1")
	testarossa.NoError(t, err)
	err = con.RecordGauge(ctx, "my_gauge", 0, "a", "1")
	testarossa.NoError(t, err)

	err = con.AddCounter(ctx, "my_gauge", 1.5, "a", "1")
	testarossa.Error(t, err)
	err = con.RecordHistogram(ctx, "my_gauge", 2.5, "a", "1")
	testarossa.Error(t, err)

	// Counter
	err = con.AddCounter(ctx, "my_counter", 1.5, "a", "1")
	testarossa.NoError(t, err)
	err = con.AddCounter(ctx, "my_counter", 0, "a", "1")
	testarossa.NoError(t, err)
	err = con.AddCounter(ctx, "my_counter", -1.5, "a", "1")
	testarossa.Error(t, err)

	err = con.RecordHistogram(ctx, "my_counter", 2.5, "a", "1")
	testarossa.Error(t, err)
	err = con.RecordGauge(ctx, "my_counter", 2.5, "a", "1")
	testarossa.Error(t, err)
}

func TestConnector_StandardMetrics(t *testing.T) {
	t.Parallel()

	con := New("standard.metrics.connector")
	testarossa.Len(t, con.metricInstruments, 0)

	err := con.Startup()
	testarossa.NoError(t, err)
	defer con.Shutdown()

	testarossa.Len(t, con.metricInstruments, 10)
	testarossa.NotNil(t, con.metricInstruments["microbus_callback_duration_seconds"])
	testarossa.NotNil(t, con.metricInstruments["microbus_server_request_duration_seconds"])
	testarossa.NotNil(t, con.metricInstruments["microbus_server_response_body_bytes"])
	testarossa.NotNil(t, con.metricInstruments["microbus_client_timeout_requests"])
	testarossa.NotNil(t, con.metricInstruments["microbus_client_ack_roundtrip_latency_seconds"])
	testarossa.NotNil(t, con.metricInstruments["microbus_log_messages"])
	testarossa.NotNil(t, con.metricInstruments["microbus_uptime_duration_seconds"])
	testarossa.NotNil(t, con.metricInstruments["microbus_cache_memory_bytes"])
	testarossa.NotNil(t, con.metricInstruments["microbus_cache_elements"])
	testarossa.NotNil(t, con.metricInstruments["microbus_cache_operations"])
}

func TestConnector_InferUnit(t *testing.T) {
	t.Parallel()

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
		testarossa.Equal(t, tc.unit, unit, "Expected %s, got %s, for %s", tc.unit, unit, tc.name)
	}
}

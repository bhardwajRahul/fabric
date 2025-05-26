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
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/trc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// metricInstrument holds the defined metric instruments.
type metricInstrument struct {
	Counter   metric.Float64Counter
	Gauge     metric.Float64Gauge
	Histogram metric.Float64Histogram

	// Legacy support
	Labels       []string
	GaugeVal     float64
	GaugeValLock sync.Mutex

	// Just-in-time instantiation
	Kind        string // counter, gauge, histogram
	Unit        string
	Description string
	Buckets     []float64
}

// initMeter initializes the OpenTelemetry meter.
func (c *Connector) initMeter(ctx context.Context) (err error) {
	// Use the OTLP endpoint
	// https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/
	// https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/
	// https://opentelemetry.io/docs/specs/otel/protocol/exporter/
	endpoint := env.Get("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if endpoint == "" {
		endpoint = env.Get("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	var exp sdkmetric.Exporter
	if endpoint != "" {
		protocol := env.Get("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")
		if protocol == "" {
			protocol = env.Get("OTEL_EXPORTER_OTLP_PROTOCOL")
		}
		if protocol == "" && strings.Contains(endpoint, ":4317") {
			protocol = "grpc"
		}
		if protocol == "grpc" {
			exp, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
		} else {
			exp, err = otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint))
		}
		if err != nil {
			return errors.Trace(err)
		}
	}
	if exp == nil {
		exp = &nilExporter{}
	}

	// Create a second exporter for Prometheus handler in order to support pull model as well
	c.metricsRegistry = prometheus.NewRegistry()
	promExp, _ := otelprom.New(otelprom.WithRegisterer(c.metricsRegistry))
	c.metricsHandler = promhttp.HandlerFor(c.metricsRegistry, promhttp.HandlerOpts{})

	intervalMillis, _ := strconv.Atoi(env.Get("OTEL_METRIC_EXPORT_INTERVAL"))
	if intervalMillis <= 0 {
		if c.Deployment() == LOCAL {
			intervalMillis = 15000
		} else {
			intervalMillis = 60000
		}
	}

	c.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(time.Duration(intervalMillis)*time.Millisecond), sdkmetric.WithProducer(&jitProducer{c: c})),
		),
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(resource.NewSchemaless(
			// https://opentelemetry.io/docs/specs/semconv/attributes-registry/service/
			attribute.String("service.namespace", c.Plane()),
			attribute.String("service.name", c.Hostname()),
			attribute.Int("service.version", c.Version()),
			attribute.String("service.instance.id", c.ID()),
			attribute.String("deployment.environment", c.Deployment()),
		)),
	)
	c.meter = c.meterProvider.Meter("microbus")

	c.DescribeHistogram(
		"microbus_callback_duration_seconds",
		"Handler processing duration [seconds]",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10},
	)
	c.DescribeHistogram(
		"microbus_server_request_duration_seconds",
		"Request processing duration [seconds]",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10},
	)
	c.DescribeHistogram(
		"microbus_server_response_body_bytes",
		"Response body size [bytes]",
		[]float64{1 << 10, 4 << 10, 16 << 10, 64 << 10, 256 << 10, 1 << 20, 4 << 20, 16 << 20},
	)
	c.DescribeCounter(
		"microbus_client_timeout_requests",
		"Number of requests downstream that timed out",
	)
	c.DescribeHistogram(
		"microbus_client_ack_roundtrip_latency_seconds",
		"Downstream ack roundtrip latency [seconds]",
		[]float64{0.001, 0.0025, 0.005, 0.0075, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5},
	)
	c.DescribeCounter(
		"microbus_log_messages",
		"Number of log messages recorded",
	)
	c.DescribeGauge(
		"microbus_uptime_duration_seconds",
		"Duration since connector was established [seconds]",
	)
	c.DescribeGauge(
		"microbus_cache_memory_bytes",
		"Total size of the elements in the local shard of the distributed cache [bytes]",
	)
	c.DescribeGauge(
		"microbus_cache_elements",
		"Total number of elements in the local shard of the distributed cache",
	)
	c.DescribeCounter(
		"microbus_cache_operations",
		"Number of operations performed on the local shard of the distributed cache",
	)
	return nil
}

// termMeter flushes and shuts down the meter collector.
func (c *Connector) termMeter(ctx context.Context) (err error) {
	if c.meterProvider == nil {
		return nil
	}
	err = c.meterProvider.Shutdown(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	c.meterProvider = nil
	c.meter = nil
	return nil
}

// SetOnObserveMetrics adds a function to be called just before metrics are produced to allow observing them just in time.
// Callbacks are called in the order they were added.
func (c *Connector) SetOnObserveMetrics(handler service.ObserveMetricsHandler) error {
	if c.IsStarted() {
		return c.captureInitErr(errors.New("already started"))
	}
	c.onObserveMetrics = append(c.onObserveMetrics, handler)
	return nil
}

// jitProducer is used to inject a callback before metrics are pushed to the collector.
type jitProducer struct {
	c *Connector
}

func (p *jitProducer) Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error) {
	p.c.observeMetricsJustInTime(ctx)
	return nil, nil
}

// observeMetricsJustInTime observes metrics just before they are shipped to OpenTelemetry.
func (c *Connector) observeMetricsJustInTime(ctx context.Context) error {
	if !c.IsStarted() {
		return nil
	}

	uptime := max(c.Now(ctx).Sub(c.startupTime), 0)
	_ = c.RecordGauge(ctx, "microbus_uptime_duration_seconds", uptime.Seconds())
	_ = c.RecordGauge(ctx, "microbus_cache_elements", float64(c.distribCache.LocalCache().Len()))
	_ = c.RecordGauge(ctx, "microbus_cache_memory_bytes", float64(c.distribCache.LocalCache().Weight()))

	if len(c.onObserveMetrics) == 0 {
		return nil
	}

	// OpenTelemetry: create a span for the callback
	ctx, span := c.StartSpan(c.Lifetime(), "observe-metrics", trc.Internal())
	atomic.AddInt32(&c.pendingOps, 1)
	startTime := time.Now()

	// Call the callback functions
	var err error
	for _, callback := range c.onObserveMetrics {
		callbackErr := errors.CatchPanic(func() error {
			return callback(ctx)
		})
		if callbackErr != nil {
			c.LogError(ctx, "Producing metrics",
				"error", callbackErr,
			)
			err = callbackErr // Remember the last error
		}
	}
	if err != nil {
		// OpenTelemetry: record the error
		span.SetError(err)
		c.ForceTrace(ctx)
	} else {
		span.SetOK(http.StatusOK)
	}
	dur := time.Since(startTime)
	atomic.AddInt32(&c.pendingOps, -1)
	_ = c.RecordHistogram(
		ctx,
		"microbus_callback_duration_seconds",
		dur.Seconds(),
		"handler", "observe-metrics",
		"error", func() string {
			if err != nil {
				return "ERROR"
			}
			return "OK"
		}(),
	)
	span.End()
	return nil
}

// DescribeHistogram defines a new histogram metric.
func (c *Connector) DescribeHistogram(name string, desc string, bucketBounds []float64) (err error) {
	if len(bucketBounds) < 1 {
		return c.captureInitErr(errors.New("empty buckets"))
	}
	sort.Float64s(bucketBounds)
	for i := 0; i < len(bucketBounds)-1; i++ {
		if bucketBounds[i+1] <= bucketBounds[i] {
			return c.captureInitErr(errors.New("buckets must be defined in ascending order"))
		}
	}
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if _, ok := c.metricInstruments[name]; ok {
		return c.captureInitErr(errors.Newf("metric '%s' already defined", name))
	}
	c.metricInstruments[name] = &metricInstrument{
		Kind:        "histogram",
		Unit:        c.inferMetricUnit(name, desc),
		Description: desc,
		Buckets:     bucketBounds,
	}
	return nil
}

// DescribeCounter defines a new counter metric.
func (c *Connector) DescribeCounter(name string, desc string) (err error) {
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if _, ok := c.metricInstruments[name]; ok {
		return c.captureInitErr(errors.Newf("metric '%s' already defined", name))
	}
	c.metricInstruments[name] = &metricInstrument{
		Kind:        "counter",
		Unit:        c.inferMetricUnit(name, desc),
		Description: desc,
	}
	return nil
}

// DescribeGauge defines a new gauge metric.
func (c *Connector) DescribeGauge(name string, desc string) (err error) {
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if _, ok := c.metricInstruments[name]; ok {
		return c.captureInitErr(errors.Newf("metric '%s' already defined", name))
	}
	c.metricInstruments[name] = &metricInstrument{
		Kind:        "gauge",
		Unit:        c.inferMetricUnit(name, desc),
		Description: desc,
	}
	return nil
}

// inferMetricUnit deduces the metric's unit from its name or description.
func (c *Connector) inferMetricUnit(name string, desc string) (unit string) {
	if strings.HasSuffix(desc, "]") {
		p := strings.LastIndex(desc, "[")
		if p > 0 {
			u := desc[p+1 : len(desc)-1]
			if matched, _ := regexp.MatchString("^[a-z]+$", u); matched {
				return u
			}
		}
	}
	common := []string{
		"nanosecond", "microsecond", "millisecond", "second",
		"minute", "hour", "day", "week",
		"byte",
		"kibibyte", "mebibyte", "gibibyte", "tebibyte",
		"kilobyte", "megabyte", "gigabyte", "terabyte",
	}
	for _, u := range common {
		if strings.HasSuffix(name, "_"+u) || strings.Contains(name, "_"+u+"_") {
			return u // Singular
		}
		if strings.HasSuffix(name, "_"+u+"s") || strings.Contains(name, "_"+u+"s_") {
			return u + "s" // Plural
		}
	}
	return ""
}

// AddCounter adds a non-negative value to a counter metric.
// Attributes conform to the standard slog pattern.
func (c *Connector) AddCounter(ctx context.Context, name string, val float64, attributes ...any) (err error) {
	if c.meter == nil {
		return nil
	}
	if val < 0 {
		return errors.Newf("counter '%s' can't be subtracted from", name)
	}
	c.metricLock.RLock()
	m, ok := c.metricInstruments[name]
	if ok && m.Counter == nil && m.Kind == "counter" {
		// Lazy instantiation
		m.Counter, err = c.meter.Float64Counter(
			name,
			metric.WithUnit(m.Unit),
			metric.WithDescription(m.Description),
		)
	}
	c.metricLock.RUnlock()
	if err != nil {
		return errors.Trace(err)
	}
	if !ok {
		return errors.Newf("unknown metric '%s'", name)
	}
	if m.Counter == nil {
		return errors.Newf("metric '%s' is not a counter", name)
	}
	attributes = append(attributes,
		"plane", c.Plane(),
		"service", c.Hostname(),
		"ver", c.Version(),
		"id", c.ID(),
		"deployment", c.Deployment(),
	)
	kvAttributes, err := attributesToKV(attributes)
	if err != nil {
		return errors.Trace(err)
	}
	m.Counter.Add(ctx, val, metric.WithAttributes(kvAttributes...))
	return nil
}

// RecordGauge observes a value for a gauge metric.
// Attributes conform to the standard slog pattern.
func (c *Connector) RecordGauge(ctx context.Context, name string, val float64, attributes ...any) (err error) {
	if c.meter == nil {
		return nil
	}
	c.metricLock.RLock()
	m, ok := c.metricInstruments[name]
	if ok && m.Gauge == nil && m.Kind == "gauge" {
		// Lazy instantiation
		m.Gauge, err = c.meter.Float64Gauge(
			name,
			metric.WithUnit(m.Unit),
			metric.WithDescription(m.Description),
		)
	}
	c.metricLock.RUnlock()
	if err != nil {
		return errors.Trace(err)
	}
	if !ok {
		return errors.Newf("unknown metric '%s'", name)
	}
	if m.Gauge == nil {
		return errors.Newf("metric '%s' is not a gauge", name)
	}
	attributes = append(attributes,
		"plane", c.Plane(),
		"service", c.Hostname(),
		"ver", c.Version(),
		"id", c.ID(),
		"deployment", c.Deployment(),
	)
	kvAttributes, err := attributesToKV(attributes)
	if err != nil {
		return errors.Trace(err)
	}
	m.Gauge.Record(ctx, val, metric.WithAttributes(kvAttributes...))
	return nil
}

// RecordHistogram observes a value for a histogram metric.
// Attributes conform to the standard slog pattern.
func (c *Connector) RecordHistogram(ctx context.Context, name string, val float64, attributes ...any) (err error) {
	if c.meter == nil {
		return nil
	}
	c.metricLock.RLock()
	m, ok := c.metricInstruments[name]
	if ok && m.Histogram == nil && m.Kind == "histogram" {
		// Lazy instantiation
		m.Histogram, err = c.meter.Float64Histogram(
			name,
			metric.WithUnit(m.Unit),
			metric.WithDescription(m.Description),
			metric.WithExplicitBucketBoundaries(m.Buckets...),
		)
	}
	c.metricLock.RUnlock()
	if err != nil {
		return errors.Trace(err)
	}
	if !ok {
		return errors.Newf("unknown metric '%s'", name)
	}
	if m.Histogram == nil {
		return errors.Newf("metric '%s' is not a histogram", name)
	}
	attributes = append(attributes,
		"plane", c.Plane(),
		"service", c.Hostname(),
		"ver", c.Version(),
		"id", c.ID(),
		"deployment", c.Deployment(),
	)
	kvAttributes, err := attributesToKV(attributes)
	if err != nil {
		return errors.Trace(err)
	}
	m.Histogram.Record(ctx, val, metric.WithAttributes(kvAttributes...))
	return nil
}

func attributesToKV(attributes []any) ([]attribute.KeyValue, error) {
	if len(attributes)%2 != 0 {
		return nil, errors.Newf("uneven number of attributes")
	}
	kvAttributes := []attribute.KeyValue{}
	for i := 0; i < len(attributes); i += 2 {
		k, ok := attributes[i].(string)
		if !ok {
			return nil, errors.Newf("expected string for attribute name, found '%s'", attributes[i])
		}
		switch v := attributes[i+1].(type) {
		case string:
			kvAttributes = append(kvAttributes, attribute.String(k, v))
		case int:
			kvAttributes = append(kvAttributes, attribute.Int(k, v))
		case int64:
			kvAttributes = append(kvAttributes, attribute.Int64(k, v))
		case float64:
			kvAttributes = append(kvAttributes, attribute.Float64(k, v))
		case bool:
			kvAttributes = append(kvAttributes, attribute.Bool(k, v))
		case []string:
			kvAttributes = append(kvAttributes, attribute.StringSlice(k, v))
		case []int:
			kvAttributes = append(kvAttributes, attribute.IntSlice(k, v))
		case []int64:
			kvAttributes = append(kvAttributes, attribute.Int64Slice(k, v))
		case []float64:
			kvAttributes = append(kvAttributes, attribute.Float64Slice(k, v))
		case []bool:
			kvAttributes = append(kvAttributes, attribute.BoolSlice(k, v))
		case fmt.Stringer:
			kvAttributes = append(kvAttributes, attribute.Stringer(k, v))
		}
	}
	return kvAttributes, nil
}

type nilExporter struct {
}

func (de *nilExporter) Temporality(ik sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}
func (de *nilExporter) Aggregation(ik sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.AggregationDefault{}
}
func (de *nilExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	return nil
}
func (de *nilExporter) ForceFlush(ctx context.Context) error {
	return nil
}
func (de *nilExporter) Shutdown(ctx context.Context) error {
	return nil
}

// DefineHistogram defines a new histogram metric.
// Histograms can only be observed.
//
// Deprecated: Use DescribeHistogram instead.
func (c *Connector) DefineHistogram(name string, help string, bucketBounds []float64, labels []string) (err error) {
	err = c.DescribeHistogram(name, help, bucketBounds)
	if err != nil {
		return errors.Trace(err)
	}
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if instrument, ok := c.metricInstruments[name]; ok {
		instrument.Labels = labels
	}
	return nil
}

// DefineCounter defines a new counter metric.
// Counters can only be incremented.
//
// Deprecated: Use DescribeCounter instead.
func (c *Connector) DefineCounter(name string, help string, labels []string) (err error) {
	err = c.DescribeCounter(name, help)
	if err != nil {
		return errors.Trace(err)
	}
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if instrument, ok := c.metricInstruments[name]; ok {
		instrument.Labels = labels
	}
	return nil
}

// DefineGauge defines a new gauge metric.
// Gauges can be observed or incremented.
//
// Deprecated: Use DescribeGauge instead.
func (c *Connector) DefineGauge(name string, help string, labels []string) (err error) {
	err = c.DescribeGauge(name, help)
	if err != nil {
		return errors.Trace(err)
	}
	c.metricLock.Lock()
	defer c.metricLock.Unlock()
	if instrument, ok := c.metricInstruments[name]; ok {
		instrument.Labels = labels
	}
	return nil
}

// IncrementMetric adds the given value to a counter or gauge metric.
// The name and labels must match a previously defined metric.
// Gauge metrics support subtraction by use of a negative value.
// Counter metrics only allow addition and a negative value will result in an error.
//
// Warning: Incrementing rather than observing a Gauge is not safe in a distributed environment
// where microservices get out of sync with each other. It is provided for backward
// compatibility only. Avoid using.
//
// Deprecated: Use AddCounter or RecordGauge instead.
func (c *Connector) IncrementMetric(name string, val float64, labels ...string) (err error) {
	if c.meter == nil {
		return nil
	}
	if val == 0 {
		return nil
	}
	c.metricLock.RLock()
	m, ok := c.metricInstruments[name]
	c.metricLock.RUnlock()
	if !ok {
		return errors.Newf("unknown metric '%s'", name)
	}
	attrs := make([]any, 0, len(labels)*2)
	for i := range labels {
		attrs = append(attrs, m.Labels[i], labels[i])
	}
	if m.Counter != nil {
		err = c.AddCounter(c.Lifetime(), name, val, attrs...)
	} else if m.Gauge != nil {
		// Warning: Incrementing rather than observing a Gauge is not safe in a distributed environment
		// where microservices get out of sync with each other.
		// This is provided here for backward compatibility only. Avoid using.
		m.GaugeValLock.Lock()
		newVal := m.GaugeVal + val
		m.GaugeVal = newVal
		m.GaugeValLock.Unlock()
		err = c.RecordGauge(c.Lifetime(), name, newVal, attrs...)
	} else {
		return errors.Newf("metric '%s' cannot be incremented", name)
	}
	return errors.Trace(err)
}

// ObserveMetric observes the given value using a histogram or summary, or sets it as a gauge's value.
// The name and labels must match a previously defined metric.
//
// Deprecated: Use RecordGauge or RecordHistogram instead.
func (c *Connector) ObserveMetric(name string, val float64, labels ...string) (err error) {
	if c.meter == nil {
		return nil
	}
	c.metricLock.RLock()
	m, ok := c.metricInstruments[name]
	c.metricLock.RUnlock()
	if !ok {
		return errors.Newf("unknown metric '%s'", name)
	}
	attrs := make([]any, 0, len(labels)*2)
	for i := range labels {
		attrs = append(attrs, m.Labels[i], labels[i])
	}
	if m.Gauge != nil {
		m.GaugeValLock.Lock()
		m.GaugeVal = val
		m.GaugeValLock.Unlock()
		err = c.RecordGauge(c.Lifetime(), name, val, attrs...)
	} else if m.Histogram != nil {
		err = c.RecordHistogram(c.Lifetime(), name, val, attrs...)
	} else {
		return errors.Newf("metric '%s' cannot be observed", name)
	}
	return errors.Trace(err)
}

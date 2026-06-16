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

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

/*
attributedMeterProvider decorates a metric.MeterProvider so that every instrument it vends stamps a fixed
set of attributes onto each measurement. MeterProvider() hands this decorator (carrying the microservice's
identity - service, deployment, plane, ver, id) to third-party libraries such as sequel and the dwarf
engine, so their "sequel_" and "dwarf_" series carry the same identity labels as the framework's own
"microbus_" metrics and are filterable by service and deployment without a target_info join.

The decorator is needed because the OpenTelemetry resource (which already identifies the microservice) is
not reachable through the metric.MeterProvider interface, so a library given only the provider cannot read
service.name to re-stamp it. The connector knows the identity, so it injects it here instead.

Synchronous instruments are wrapped to append the attribute option on Add/Record. Observable instruments are
passed through unchanged; their values are observed inside a callback registered via RegisterCallback, so the
injection happens by wrapping the metric.Observer handed to that callback. A callback registered through a
WithInt64Callback/WithFloat64Callback construction option (which neither sequel nor dwarf use) is not
intercepted and its observations are emitted without the injected attributes.
*/
type attributedMeterProvider struct {
	embedded.MeterProvider
	inner metric.MeterProvider
	opt   metric.MeasurementOption
}

func (p attributedMeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return attributedMeter{inner: p.inner.Meter(name, opts...), opt: p.opt}
}

// attributedMeter wraps each synchronous instrument it creates and injects the identity attributes into the
// observer of any RegisterCallback. Observable instruments are returned as-is.
type attributedMeter struct {
	embedded.Meter
	inner metric.Meter
	opt   metric.MeasurementOption
}

func (m attributedMeter) Int64Counter(name string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	inst, err := m.inner.Int64Counter(name, opts...)
	return int64Counter{Int64Counter: inst, opt: m.opt}, err
}

func (m attributedMeter) Int64UpDownCounter(name string, opts ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	inst, err := m.inner.Int64UpDownCounter(name, opts...)
	return int64UpDownCounter{Int64UpDownCounter: inst, opt: m.opt}, err
}

func (m attributedMeter) Int64Histogram(name string, opts ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	inst, err := m.inner.Int64Histogram(name, opts...)
	return int64Histogram{Int64Histogram: inst, opt: m.opt}, err
}

func (m attributedMeter) Int64Gauge(name string, opts ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	inst, err := m.inner.Int64Gauge(name, opts...)
	return int64Gauge{Int64Gauge: inst, opt: m.opt}, err
}

func (m attributedMeter) Float64Counter(name string, opts ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	inst, err := m.inner.Float64Counter(name, opts...)
	return float64Counter{Float64Counter: inst, opt: m.opt}, err
}

func (m attributedMeter) Float64UpDownCounter(name string, opts ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	inst, err := m.inner.Float64UpDownCounter(name, opts...)
	return float64UpDownCounter{Float64UpDownCounter: inst, opt: m.opt}, err
}

func (m attributedMeter) Float64Histogram(name string, opts ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	inst, err := m.inner.Float64Histogram(name, opts...)
	return float64Histogram{Float64Histogram: inst, opt: m.opt}, err
}

func (m attributedMeter) Float64Gauge(name string, opts ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	inst, err := m.inner.Float64Gauge(name, opts...)
	return float64Gauge{Float64Gauge: inst, opt: m.opt}, err
}

func (m attributedMeter) Int64ObservableCounter(name string, opts ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	return m.inner.Int64ObservableCounter(name, opts...)
}

func (m attributedMeter) Int64ObservableUpDownCounter(name string, opts ...metric.Int64ObservableUpDownCounterOption) (metric.Int64ObservableUpDownCounter, error) {
	return m.inner.Int64ObservableUpDownCounter(name, opts...)
}

func (m attributedMeter) Int64ObservableGauge(name string, opts ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	return m.inner.Int64ObservableGauge(name, opts...)
}

func (m attributedMeter) Float64ObservableCounter(name string, opts ...metric.Float64ObservableCounterOption) (metric.Float64ObservableCounter, error) {
	return m.inner.Float64ObservableCounter(name, opts...)
}

func (m attributedMeter) Float64ObservableUpDownCounter(name string, opts ...metric.Float64ObservableUpDownCounterOption) (metric.Float64ObservableUpDownCounter, error) {
	return m.inner.Float64ObservableUpDownCounter(name, opts...)
}

func (m attributedMeter) Float64ObservableGauge(name string, opts ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	return m.inner.Float64ObservableGauge(name, opts...)
}

func (m attributedMeter) RegisterCallback(f metric.Callback, instruments ...metric.Observable) (metric.Registration, error) {
	wrapped := func(ctx context.Context, obs metric.Observer) error {
		return f(ctx, attributedObserver{inner: obs, opt: m.opt})
	}
	return m.inner.RegisterCallback(wrapped, instruments...)
}

// attributedObserver appends the identity attributes to every value observed inside a registered callback.
type attributedObserver struct {
	embedded.Observer
	inner metric.Observer
	opt   metric.MeasurementOption
}

func (o attributedObserver) ObserveInt64(inst metric.Int64Observable, value int64, opts ...metric.ObserveOption) {
	o.inner.ObserveInt64(inst, value, append(opts, o.opt)...)
}

func (o attributedObserver) ObserveFloat64(inst metric.Float64Observable, value float64, opts ...metric.ObserveOption) {
	o.inner.ObserveFloat64(inst, value, append(opts, o.opt)...)
}

// The synchronous instrument wrappers embed the real instrument (satisfying the interface and its embedded
// marker) and override only Add/Record to append the identity attributes.

type int64Counter struct {
	metric.Int64Counter
	opt metric.MeasurementOption
}

func (i int64Counter) Add(ctx context.Context, value int64, opts ...metric.AddOption) {
	i.Int64Counter.Add(ctx, value, append(opts, i.opt)...)
}

type int64UpDownCounter struct {
	metric.Int64UpDownCounter
	opt metric.MeasurementOption
}

func (i int64UpDownCounter) Add(ctx context.Context, value int64, opts ...metric.AddOption) {
	i.Int64UpDownCounter.Add(ctx, value, append(opts, i.opt)...)
}

type int64Histogram struct {
	metric.Int64Histogram
	opt metric.MeasurementOption
}

func (i int64Histogram) Record(ctx context.Context, value int64, opts ...metric.RecordOption) {
	i.Int64Histogram.Record(ctx, value, append(opts, i.opt)...)
}

type int64Gauge struct {
	metric.Int64Gauge
	opt metric.MeasurementOption
}

func (i int64Gauge) Record(ctx context.Context, value int64, opts ...metric.RecordOption) {
	i.Int64Gauge.Record(ctx, value, append(opts, i.opt)...)
}

type float64Counter struct {
	metric.Float64Counter
	opt metric.MeasurementOption
}

func (f float64Counter) Add(ctx context.Context, value float64, opts ...metric.AddOption) {
	f.Float64Counter.Add(ctx, value, append(opts, f.opt)...)
}

type float64UpDownCounter struct {
	metric.Float64UpDownCounter
	opt metric.MeasurementOption
}

func (f float64UpDownCounter) Add(ctx context.Context, value float64, opts ...metric.AddOption) {
	f.Float64UpDownCounter.Add(ctx, value, append(opts, f.opt)...)
}

type float64Histogram struct {
	metric.Float64Histogram
	opt metric.MeasurementOption
}

func (f float64Histogram) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	f.Float64Histogram.Record(ctx, value, append(opts, f.opt)...)
}

type float64Gauge struct {
	metric.Float64Gauge
	opt metric.MeasurementOption
}

func (f float64Gauge) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	f.Float64Gauge.Record(ctx, value, append(opts, f.opt)...)
}

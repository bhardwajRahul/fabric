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
	"context"
	"net/url"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/trc"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	prototrace "go.opentelemetry.io/proto/otlp/trace/v1"
)

var nilSpan = trc.NewSpan(nil)

// initTracer initializes an OpenTelemetry tracer
func (c *Connector) initTracer(ctx context.Context) (err error) {
	if c.traceProvider != nil {
		// Already initialized
		return nil
	}

	// Use the OTLP endpoint
	// https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/
	// https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/
	// https://opentelemetry.io/docs/specs/otel/protocol/exporter/
	var exp *otlptrace.Exporter
	endpoint := resolveOTLPEndpoint("TRACES")
	if endpoint == "nil" {
		// The nil client is used for testing of span creation
		exp, err = otlptrace.New(ctx, &nilTraceClient{})
		if err != nil {
			return errors.Trace(err)
		}
	} else if endpoint != "" {
		var conn *otlpConn
		conn, c.traceOTLPKey, err = acquireOTLPConn("TRACES", endpoint)
		if err != nil {
			return errors.Trace(err)
		}
		if conn.protocol() == "grpc" {
			exp, err = otlptracegrpc.New(ctx,
				otlptracegrpc.WithGRPCConn(conn.grpc),
				otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{Enabled: false}),
			)
		} else {
			exp, err = otlptracehttp.New(ctx,
				otlptracehttp.WithEndpointURL(endpoint),
				otlptracehttp.WithHTTPClient(conn.http),
				otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}),
			)
		}
		if err != nil {
			return errors.Trace(err)
		}
	}
	if exp == nil {
		return nil // Disables tracing without overhead
	}

	var sp sdktrace.SpanProcessor
	switch c.deployment {
	case LOCAL, TESTING, LAB:
		sp = sdktrace.NewBatchSpanProcessor(exp)
	default: // PROD
		// Trace only explicitly selected transactions
		c.traceProcessor = newSelectiveProcessor(exp, 8192) // Approx 10MB per microservice
		sp = c.traceProcessor
	}
	c.traceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(newMuffler())),
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(c.otelResource()),
	)
	c.tracer = c.traceProvider.Tracer("")
	return nil
}

// termTracer shuts down the OpenTelemetry tracer
func (c *Connector) termTracer(ctx context.Context) (err error) {
	if c.traceProvider != nil {
		// Flush pending spans through the exporter before releasing the shared connection
		err = c.traceProvider.Shutdown(ctx)
		c.traceProvider = nil
		c.tracer = nil
	}
	releaseOTLPConn(c.traceOTLPKey)
	c.traceOTLPKey = ""
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// StartSpan creates a tracing span and a context containing the newly-created span.
// If the context provided already contains asSpan then the newly-created
// span will be a child of that span, otherwise it will be a root span.
//
// Any Span that is created must also be ended. This is the responsibility of the user.
// Implementations of this API may leak memory or other resources if spans are not ended.
func (c *Connector) StartSpan(ctx context.Context, spanName string, opts ...trc.Option) (context.Context, trc.Span) {
	if c.tracer != nil {
		options := make([]trace.SpanStartOption, len(opts))
		for i := range opts {
			options[i] = opts[i]
		}
		ctx, span := c.tracer.Start(ctx, spanName, options...)
		return ctx, trc.NewSpan(span)
	} else {
		return ctx, nilSpan
	}
}

// Span returns the tracing span stored in the context.
func (c *Connector) Span(ctx context.Context) trc.Span {
	span := trace.SpanFromContext(ctx)
	return trc.NewSpan(span)
}

// TracerProvider returns the OpenTelemetry tracer provider of the microservice, or a no-op provider when tracing is
// not configured. Use it to instrument third-party libraries against the same pipeline and resource as the framework.
func (c *Connector) TracerProvider() trace.TracerProvider {
	if c.traceProvider == nil {
		return tracenoop.NewTracerProvider()
	}
	return c.traceProvider
}

// ForceTrace forces the trace containing the span to be exported
func (c *Connector) ForceTrace(ctx context.Context) {
	if c.traceProcessor != nil {
		traceID := c.Span(ctx).TraceID()
		if traceID != "" {
			if c.traceProcessor.Select(traceID) {
				// Broadcast to all microservices to export all spans belonging to this trace
				c.Go(ctx, func(ctx context.Context) error {
					traceID := c.Span(ctx).TraceID()
					for range c.Publish(ctx, pub.GET("https://all:888/trace?id="+url.QueryEscape(traceID))) {
					}
					return nil
				})
			}
		}
	}
}

type nilTraceClient struct{}

func (nc *nilTraceClient) Start(ctx context.Context) error {
	return nil
}
func (nc *nilTraceClient) Stop(ctx context.Context) error {
	return nil
}
func (nc *nilTraceClient) UploadTraces(ctx context.Context, protoSpans []*prototrace.ResourceSpans) error {
	return nil
}

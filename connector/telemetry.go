/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

package connector

import (
	"context"
	"net/url"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/trc"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type contextKeyType struct{}

var contextKey contextKeyType

// initTracer initializes an OpenTelemetry tracer
func (c *Connector) initTracer(ctx context.Context) (err error) {
	if c.traceProvider != nil {
		// Already initialized
		return nil
	}

	// Use the OTLP HTTP endpoint. Default to the non-secure HTTP protocol
	endpoint := env.Get("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if endpoint == "" {
		endpoint = env.Get("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" && c.deployment == LOCAL {
		endpoint = "http://127.0.0.1:4318"
	}

	var sp sdktrace.SpanProcessor
	var sampler sdktrace.Sampler
	switch c.deployment {
	case LOCAL, TESTINGAPP:
		// Trace all spans
		var exp *otlptrace.Exporter
		if endpoint == "" {
			exp, err = otlptrace.New(ctx, &nilClient{})
		} else {
			exp, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		}
		if err != nil {
			return errors.Trace(err)
		}
		sp = sdktrace.NewBatchSpanProcessor(exp)
		sampler = sdktrace.AlwaysSample()
	default: // PROD, LAB
		if endpoint == "" {
			return nil // Disables tracing without overhead
		}
		// Trace only explicitly selected transactions
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return errors.Trace(err)
		}
		c.traceProcessor = newTraceSelector(exp)
		sp = c.traceProcessor
		sampler = sdktrace.AlwaysSample()
	}
	c.traceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.namespace", c.plane),
			attribute.String("service.name", c.hostName),
			attribute.Int("service.version", c.version),
			attribute.String("service.instance.id", c.id),
		)),
	)
	c.tracer = c.traceProvider.Tracer("")
	return nil
}

// termTracer shuts down the OpenTelemetry tracer
func (c *Connector) termTracer(ctx context.Context) (err error) {
	if c.traceProvider == nil {
		// Not initialized
		return nil
	}
	err = c.traceProvider.Shutdown(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	c.traceProvider = nil
	c.tracer = nil
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
		ctx, span := c.tracer.Start(ctx, spanName, opts...)
		ctx = context.WithValue(ctx, contextKey, span)
		return ctx, trc.NewSpan(span)
	} else {
		return ctx, trc.NewSpan(nil)
	}
}

// Span returns the tracing span stored in the context.
func (c *Connector) Span(ctx context.Context) trc.Span {
	span := ctx.Value(contextKey)
	if span != nil {
		return trc.NewSpan(span.(trace.Span))
	} else {
		return trc.NewSpan(nil)
	}
}

// ForceTrace forces the trace containing the span to be exported
func (c *Connector) ForceTrace(span trc.Span) {
	if c.traceProcessor != nil {
		c.traceProcessor.Select(span.TraceID())
		// Broadcast to all microservices to export all spans belonging to this trace
		c.Go(c.lifetimeCtx, func(ctx context.Context) error {
			for r := range c.Publish(c.lifetimeCtx, pub.GET("https://all:888/trace?id="+url.QueryEscape(span.TraceID()))) {
				_, _ = r.Get()
			}
			return nil
		})
	}
}

type nilClient struct{}

func (nc *nilClient) Start(ctx context.Context) error {
	return nil
}
func (nc *nilClient) Stop(ctx context.Context) error {
	return nil
}
func (nc *nilClient) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	return nil
}

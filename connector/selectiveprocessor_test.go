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
	"strconv"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type exporter struct {
	Callback func(ctx context.Context, spans []sdktrace.ReadOnlySpan) error
}

func (e *exporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e.Callback != nil {
		return e.Callback(ctx, spans)
	}
	return nil
}

func (e *exporter) Shutdown(ctx context.Context) error {
	return nil
}

func TestConnector_TracingExport(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	countExported := 0
	exportedSpans := map[string]bool{}
	ts := newSelectiveProcessor(&exporter{
		Callback: func(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
			countExported += len(spans)
			for _, span := range spans {
				exportedSpans[span.SpanContext().SpanID().String()] = true
			}
			return nil
		},
	}, 16)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	// Nothing traced yet
	tt.Zero(int(ts.insertionPoint.Load()))
	tt.Zero(countExported)
	tt.Zero(len(ts.selected1) + len(ts.selected2))
	tt.Zero(ts.lockCount)

	_, span := tracer.Start(ctx, "1")
	span.End()
	_, span = tracer.Start(ctx, "2")
	span.End()

	subCtx, span := tracer.Start(ctx, "3")
	_, subSpan1 := tracer.Start(subCtx, "3.1")
	tt.Equal(span.SpanContext().TraceID(), subSpan1.SpanContext().TraceID())
	subSpan1.End()

	// The spans should be buffered but not yet exported
	tt.Equal(3, int(ts.insertionPoint.Load()))
	tt.Zero(countExported)
	tt.Zero(ts.lockCount)

	// Select the parent span's trace ID for exporting
	ts.Select(span.SpanContext().TraceID().String())
	ts.ForceFlush(ctx) // Flush the queue
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))

	// The closed subspan should have gotten immediately exported
	tt.Equal(3, int(ts.insertionPoint.Load()))
	tt.Equal(1, countExported)
	tt.True(exportedSpans[subSpan1.SpanContext().SpanID().String()])
	tt.Equal(1, ts.lockCount)

	// Add a second subspan
	_, subSpan2 := tracer.Start(subCtx, "3.2")
	tt.Equal(span.SpanContext().TraceID(), subSpan2.SpanContext().TraceID())
	subSpan2.End()
	ts.ForceFlush(ctx) // Flush the queue

	// The new subspan should have gotten immediately exported and not buffered
	tt.Equal(3, int(ts.insertionPoint.Load()))
	tt.Equal(2, countExported)
	tt.True(exportedSpans[subSpan2.SpanContext().SpanID().String()])
	tt.Equal(2, ts.lockCount)

	span.End()
	ts.ForceFlush(ctx) // Flush the queue

	// The parent span should have gotten immediately exported and not buffered
	tt.Equal(3, int(ts.insertionPoint.Load()))
	tt.Equal(3, countExported)
	tt.True(exportedSpans[span.SpanContext().SpanID().String()])
	tt.Equal(3, ts.lockCount)

	// Select the same trace ID a second time
	ts.Select(span.SpanContext().TraceID().String())
	ts.ForceFlush(ctx) // Flush the queue
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(3, int(ts.insertionPoint.Load()))
	tt.Equal(3, countExported)
	tt.Equal(4, ts.lockCount)
}

func TestConnector_TracingTTLClearMaps(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ts := newSelectiveProcessor(&exporter{}, 16)

	ts.Select("1")
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(1, ts.lockCount)
	ts.Select("2")
	tt.Equal(2, len(ts.selected1)+len(ts.selected2))
	tt.Equal(2, ts.lockCount)
	ts.Select("2")
	tt.Equal(2, len(ts.selected1)+len(ts.selected2))
	tt.Equal(3, ts.lockCount)
	ts.Select("3")
	tt.Equal(3, len(ts.selected1)+len(ts.selected2))
	tt.Equal(4, ts.lockCount)

	// Selection maps should be cleared after TTL
	ts.clockOffset += time.Second * (maxTTLSeconds + 1)
	ts.Select("4")
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(5, ts.lockCount)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	ctx := context.Background()
	_, span := tracer.Start(ctx, "1")
	span.End()

}

func TestConnector_TracingTTLNoLock(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	ts := newSelectiveProcessor(&exporter{}, 16)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	// First span should not lock because the selector maps are empty
	_, span := tracer.Start(ctx, "A")
	span.End()

	tt.Zero(ts.lockCount)

	// Add a random selection
	ts.Select("123")
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(1, ts.lockCount)

	// Span should lock because there's a valid selector
	_, span = tracer.Start(ctx, "B")
	span.End()
	tt.Equal(2, ts.lockCount)

	_, span = tracer.Start(ctx, "C")
	span.End()
	tt.Equal(3, ts.lockCount)

	// After TTL passed, the selectors should be ignored so there should be no lock
	ts.clockOffset += time.Second * (maxTTLSeconds + 1)
	_, span = tracer.Start(ctx, "D")
	span.End()
	tt.Equal(3, ts.lockCount)
}

func TestConnector_TracingSelectorCapacityRollover(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ts := newSelectiveProcessor(&exporter{}, 16)

	for i := range maxSelected / 2 {
		ts.Select(strconv.Itoa(i))
	}
	tt.Len(ts.selected1, maxSelected/2)

	ts.Select(strconv.Itoa(maxSelected / 2))
	tt.Len(ts.selected1, 1)
	tt.Len(ts.selected2, maxSelected/2)

	for i := 1; i < maxSelected/2; i++ {
		ts.Select(strconv.Itoa(maxSelected/2 + i))
	}
	tt.Len(ts.selected1, maxSelected/2)
	tt.Len(ts.selected2, maxSelected/2)

	ts.Select(strconv.Itoa(maxSelected))
	tt.Len(ts.selected1, 1)
	tt.Len(ts.selected2, maxSelected/2)
}

func TestConnector_TracingBufferCapacityRollover(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	n := 16
	ts := newSelectiveProcessor(&exporter{}, n)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	// Fill in the buffer
	tt.Zero(ts.insertionPoint.Load())
	for i := range n {
		tt.Equal(int32(i), ts.insertionPoint.Load())
		tt.Nil(ts.buffer[i].Load())
		_, span := tracer.Start(ctx, "A")
		span.End()
		tt.NotNil(ts.buffer[i].Load())
		tt.Equal(int32(i+1), ts.insertionPoint.Load())
	}

	// Second pass should overwrite in the buffer
	for i := range n {
		if i > 0 {
			tt.Equal(int32(i), ts.insertionPoint.Load())
		}
		before := ts.buffer[i].Load()
		tt.NotNil(before)
		_, span := tracer.Start(ctx, "A")
		span.End()
		after := ts.buffer[i].Load()
		tt.NotNil(after)
		tt.NotEqual(before, after)
		tt.Equal(int32(i+1), ts.insertionPoint.Load())
	}
}

func BenchmarkConnector_TracingOnEnd(b *testing.B) {
	ctx := context.Background()

	ts := newSelectiveProcessor(&exporter{}, 8192)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	arr := make([]trace.Span, b.N)
	for i := range b.N {
		_, arr[i] = tracer.Start(ctx, "A")
	}

	b.ResetTimer()
	for i := range b.N {
		arr[i].End()
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_TracingOnEnd-10    	 5657827	       277.3 ns/op	     432 B/op	       2 allocs/op
}

func TestConnector_DuplicateSelect(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	countExported := 0
	exportedSpans := map[string]bool{}
	ts := newSelectiveProcessor(&exporter{
		Callback: func(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
			countExported += len(spans)
			for _, span := range spans {
				exportedSpans[span.SpanContext().SpanID().String()] = true
			}
			return nil
		},
	}, 16)

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(ts),
	)
	tracer := traceProvider.Tracer("")

	// Nothing traced yet
	tt.Zero(int(ts.insertionPoint.Load()))
	tt.Zero(countExported)
	tt.Zero(len(ts.selected1) + len(ts.selected2))
	tt.Zero(ts.lockCount)

	_, span := tracer.Start(ctx, "1")
	span.End()

	ok := ts.Select(span.SpanContext().TraceID().String())
	tt.True(ok)
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(1, ts.lockCount)
	ts.ForceFlush(ctx) // Flush the queue
	tt.Equal(1, countExported)

	ok = ts.Select(span.SpanContext().TraceID().String())
	tt.False(ok)
	tt.Equal(1, len(ts.selected1)+len(ts.selected2))
	tt.Equal(2, ts.lockCount)
	ts.ForceFlush(ctx) // Flush the queue
	tt.Equal(1, countExported)
}

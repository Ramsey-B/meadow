package tracing

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// SetTracer sets the tracer to be used for tracing.
func SetTracer(t trace.Tracer) {
	tracer = t
}

// GetActiveSpan returns the active span from the context.
func GetActiveSpan(ctx context.Context) trace.Span {
	if tracer == nil {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	// Check if the span is a no-op span (which is what we get when there's no real span)
	if !span.SpanContext().IsValid() {
		return nil
	}
	return span
}

// StartSpan starts a new span with the given name and returns the context and span.
func StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.Start(ctx, spanName)
}

// GetTraceParent returns the trace parent from the context.
func GetTraceParent(ctx context.Context) string {
	span := GetActiveSpan(ctx)
	if span == nil {
		return ""
	}

	tp := propagation.TraceContext{}
	carrier := propagation.MapCarrier{}
	tp.Inject(ctx, carrier)

	return carrier.Get("traceparent")
}

// GetTraceState returns the trace state from the context.
func GetTraceState(ctx context.Context) string {
	span := GetActiveSpan(ctx)
	if span == nil {
		return ""
	}

	tp := propagation.TraceContext{}
	carrier := propagation.MapCarrier{}
	tp.Inject(ctx, carrier)

	return carrier.Get("tracestate")
}

// GetTraceID returns the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	span := GetActiveSpan(ctx)
	if span == nil {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID returns the span ID from the context.
func GetSpanID(ctx context.Context) string {
	span := GetActiveSpan(ctx)
	if span == nil {
		return ""
	}
	return span.SpanContext().SpanID().String()
}


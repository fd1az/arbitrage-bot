package apm

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Tracer interface {
	StartSpanFromContext(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, Span)
	SpanFromContext(ctx context.Context) Span
	GetTracer() trace.Tracer
}

type openTracer struct {
	tracer trace.Tracer
}

func NewTracer(name string) Tracer {
	return &openTracer{
		otel.Tracer(name),
	}
}

func (t *openTracer) StartSpanFromContext(
	ctx context.Context, name string, opts ...trace.SpanStartOption,
) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name, opts...)
	return ctx, NewSpan(span)
}

func (t *openTracer) SpanFromContext(ctx context.Context) Span {
	return NewSpan(trace.SpanFromContext(ctx))
}

func (t *openTracer) GetTracer() trace.Tracer {
	return t.tracer
}

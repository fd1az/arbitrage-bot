package apm

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Span interface {
	SetAttributes(value ...attribute.KeyValue)
	SetAttribute(value attribute.KeyValue)
	End(options ...trace.SpanEndOption)
	NoticeError(err error)
	AddEvent(name string, options ...trace.EventOption)
	IsRecording() bool
	RecordError(err error, options ...trace.EventOption)
	SpanContext() trace.SpanContext
	SetStatus(code codes.Code, description string)
	SetName(name string)
	TracerProvider() trace.TracerProvider
}

type traceSpan struct {
	span trace.Span
}

func NewSpan(span trace.Span) Span {
	return &traceSpan{
		span,
	}
}

func (t *traceSpan) SetAttributes(values ...attribute.KeyValue) {
	for _, value := range values {
		t.span.SetAttributes(value)
	}
}

func (t *traceSpan) SetAttribute(value attribute.KeyValue) {
	t.span.SetAttributes(value)
}

func (t *traceSpan) End(options ...trace.SpanEndOption) {
	t.span.End(options...)
}

func (t *traceSpan) NoticeError(err error) {
	t.span.RecordError(err)
	t.span.SetStatus(codes.Error, err.Error())
}

func (t *traceSpan) AddEvent(name string, options ...trace.EventOption) {
	t.span.AddEvent(name, options...)
}

func (t *traceSpan) IsRecording() bool {
	return t.span.IsRecording()
}

func (t *traceSpan) RecordError(err error, options ...trace.EventOption) {
	t.span.RecordError(err, options...)
}

func (t *traceSpan) SpanContext() trace.SpanContext {
	return t.span.SpanContext()
}

func (t *traceSpan) SetStatus(code codes.Code, description string) {
	t.span.SetStatus(code, description)
}

func (t *traceSpan) SetName(name string) {
	t.span.SetName(name)
}

func (t *traceSpan) TracerProvider() trace.TracerProvider {
	return t.span.TracerProvider()
}

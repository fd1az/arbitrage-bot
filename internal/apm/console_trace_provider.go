package apm

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type ConsoleTraceProvider struct {
	tp *sdktrace.TracerProvider
}

func NewEmptyTraceProvider() TraceProvider {
	return ConsoleTraceProvider{}
}

func NewConsoleTraceProvider() TraceProvider {
	exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	// Set global trace provider
	otel.SetTracerProvider(tp)

	return ConsoleTraceProvider{tp}
}

func (ctp ConsoleTraceProvider) Stop() error {
	return nil
}

package apm

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/fd1az/arbitrage-bot/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

type Provider string

const (
	NewRelicProvider  Provider = "NEWRELIC_PROVIDER"
	ZipkinProvider    Provider = "ZIPKIN_PROVIDER"
	HoneycombProvider Provider = "HONEYCOMB_PROVIDER"
	JaegerProvider    Provider = "JAEGER_PROVIDER"
	ConsoleProvider   Provider = "CONSOLE_PROVIDER"
	EmptyProvider     Provider = "EMPTY_PROVIDER"
)

type TraceProvider interface {
	Stop() error
}

type traceProvider struct {
	tp *sdktrace.TracerProvider
}

type TracerOptions struct {
	exporter           sdktrace.SpanExporter
	tracerProviderName string
	useEmpty           bool
}

type TracerOption func(*TracerOptions)

func WithProvider(provider Provider, log logger.LoggerInterface) TracerOption {
	if provider == NewRelicProvider {
		return useNewRelic(log)
	}

	if provider == ZipkinProvider {
		return useZipkin(log)
	}

	if provider == ConsoleProvider {
		return useConsole(log)
	}

	if provider == HoneycombProvider {
		return useHoneycomb(log)
	}

	log.Warn(context.Background(), "TracerProvider not found, using EmptyProvider")

	return useEmpty()
}

func useEmpty() TracerOption {
	return func(option *TracerOptions) {
		option.useEmpty = true
		option.tracerProviderName = string(EmptyProvider)
	}
}

func useConsole(log logger.LoggerInterface) TracerOption {
	return func(option *TracerOptions) {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			panic(err)
		}

		option.exporter = exp
		option.tracerProviderName = string(ConsoleProvider)
	}
}

func useZipkin(log logger.LoggerInterface) TracerOption {
	return func(option *TracerOptions) {
		url := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		exp, err := zipkin.New(url)
		if err != nil {
			panic(err)
		}

		option.exporter = exp
		option.tracerProviderName = string(ZipkinProvider)
	}
}

func useNewRelic(log logger.LoggerInterface) TracerOption {
	return func(option *TracerOptions) {
		headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS_KEY")
		url := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		exp, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(url),
			otlptracegrpc.WithHeaders(map[string]string{"api-key": headers}),
		)

		if err != nil {
			panic(err)
		}

		option.exporter = exp
		option.tracerProviderName = string(NewRelicProvider)
	}
}

func useHoneycomb(log logger.LoggerInterface) TracerOption {
	return func(option *TracerOptions) {
		headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
		url := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")

		headerKeyValue := strings.Split(headers, "=")
		if len(headerKeyValue) != 2 {
			log.Error(context.Background(), "Invalid OTEL_EXPORTER_OTLP_HEADERS format, expected key=value")
			panic("Invalid OTEL_EXPORTER_OTLP_HEADERS format")
		}

		// Use HTTP or gRPC based on protocol
		var exp sdktrace.SpanExporter
		var err error

		if protocol == "http/protobuf" {
			log.Info(context.Background(), "Initializing Honeycomb with HTTP/Protobuf exporter", "endpoint", url)
			exp, err = useHoneycombHTTP(url, headerKeyValue)
		} else {
			log.Info(context.Background(), "Initializing Honeycomb with gRPC exporter", "endpoint", url)
			exp, err = useHoneycombGRPC(url, headerKeyValue)
		}

		if err != nil {
			log.Error(context.Background(), "Error initializing Honeycomb exporter", "error", err)
			panic(err)
		}

		option.exporter = exp
		option.tracerProviderName = string(HoneycombProvider)
	}
}

// useHoneycombHTTP creates an HTTP OTLP exporter for Honeycomb
func useHoneycombHTTP(url string, headerKeyValue []string) (sdktrace.SpanExporter, error) {
	return otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpointURL(url),
		otlptracehttp.WithHeaders(map[string]string{
			headerKeyValue[0]: headerKeyValue[1], // x-honeycomb-team header
		}),
	)
}

// useHoneycombGRPC creates a gRPC OTLP exporter for Honeycomb
func useHoneycombGRPC(url string, headerKeyValue []string) (sdktrace.SpanExporter, error) {
	return otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpointURL(url),
		otlptracegrpc.WithHeaders(map[string]string{
			headerKeyValue[0]: headerKeyValue[1], // x-honeycomb-team header
		}),
	)
}

func NewTraceProvider(log logger.LoggerInterface, options ...TracerOption) TraceProvider {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")

	if len(options) == 0 {
		options = []TracerOption{useHoneycomb(log)}
	}

	opts := &TracerOptions{}

	for _, opt := range options {
		opt(opts)
	}

	if opts.useEmpty {
		return NewEmptyTraceProvider()
	}

	exp := opts.exporter

	rsrc, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("otel.provider", opts.tracerProviderName),
		))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(rsrc),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set trace propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

	return &traceProvider{
		tp,
	}
}

func (o *traceProvider) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5) //nolint:gomnd
	defer cancel()

	if err := o.tp.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

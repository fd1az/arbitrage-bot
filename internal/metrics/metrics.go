package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metric2 "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

type MetricProvider interface {
	Meter(name string, options ...metric.MeterOption) metric.Meter
	Shutdown(ctx context.Context) error
}

func getReaders(ctx context.Context, cfg Config, opt []otlpmetricgrpc.Option) []metric2.Reader {
	var readers []metric2.Reader

	for _, provider := range cfg.Provider {
		switch provider.Provider {
		case PrometheusProvider:
			promExporter, err := prometheus.New()
			if err != nil {
				panic(err)
			}

			readers = append(readers, promExporter)
		case OtelCollector:
			cfg := []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithEndpointURL(provider.Endpoint),
				otlpmetricgrpc.WithHeaders(provider.Headers),
			}

			if provider.Insecure {
				cfg = append(cfg, otlpmetricgrpc.WithInsecure())
			}

			exp, err := otlpmetricgrpc.New(ctx, cfg...)
			if err != nil {
				panic(err)
			}

			readers = append(readers, metric2.NewPeriodicReader(exp))
		}
	}

	if len(cfg.Provider) == 0 {
		exp, err := otlpmetricgrpc.New(ctx, opt...)
		if err != nil {
			panic(err)
		}

		readers = append(readers, metric2.NewPeriodicReader(exp))
	}

	return readers
}

func NewMetricProvider(options ...OptionFn) MetricProvider {
	ctx := context.Background()

	var cfg Config

	for _, opt := range options {
		cfg = opt(cfg)
	}

	var opt []otlpmetricgrpc.Option

	readers := getReaders(ctx, cfg, opt)

	var metricsOps []metric2.Option

	for _, reader := range readers {
		metricsOps = append(metricsOps, metric2.WithReader(reader))
	}

	if cfg.ServiceName != "" {
		metricsOps = append(metricsOps, metric2.WithResource(
			resource.NewSchemaless(semconv.ServiceNameKey.String(cfg.ServiceName)),
		))
	} else {
		serviceName := os.Getenv("OTEL_SERVICE_NAME")

		metricsOps = append(metricsOps, metric2.WithResource(
			resource.NewSchemaless(semconv.ServiceNameKey.String(serviceName)),
		))
	}

	meterProvider := metric2.NewMeterProvider(metricsOps...)

	otel.SetMeterProvider(meterProvider)

	return meterProvider
}

func ServePrometheusMetrics(opt ...PromOptionFn) {
	var cfg PromServerConfig
	var port = "2223"

	for _, o := range opt {
		cfg = o(cfg)
	}

	if cfg.port != "" {
		port = cfg.port
	}

	log.Printf("serving metrics at localhost:2223/metrics")
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil) //nolint:gosec // Ignoring G114: Use of net/http serve function that has no support for setting timeouts.
	if err != nil {
		fmt.Printf("error serving http: %v", err)
		return
	}
}

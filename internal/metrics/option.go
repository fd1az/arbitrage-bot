package metrics

import (
	"fmt"
	"os"
)

type Provider string

const (
	PrometheusProvider Provider = "prometheus"
	OtelCollector      Provider = "customOtelCollector"
	InsecureOtel                = false
	SecureOtel                  = true
)

func NewHoneycombConfig() ProviderCfg {
	otelKey := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS_KEY")
	url := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	serviceName := os.Getenv("OTEL_SERVICE_NAME")

	headers := map[string]string{
		"x-honeycomb-team":    otelKey,
		"x-honeycomb-dataset": fmt.Sprintf("%s_metrics", serviceName),
	}

	return ProviderCfg{
		Provider: OtelCollector,
		Endpoint: url,
		Headers:  headers,
	}
}

func NewOtelCollectorConfig(url string, headers map[string]string, insecure bool) ProviderCfg {
	provider := ProviderCfg{
		Provider: OtelCollector,
		Endpoint: url,
		Headers:  headers,
		Insecure: insecure,
	}

	return provider
}

type Config struct {
	ServiceName string
	Provider    []ProviderCfg
}

type ProviderCfg struct {
	Provider Provider
	Endpoint string
	Headers  map[string]string
	Insecure bool
}

type OptionFn func(config Config) Config

func WithProviderConfig(provider ProviderCfg) OptionFn {
	return func(config Config) Config {
		config.Provider = append(config.Provider, provider)

		return config
	}
}

type PromServerConfig struct {
	port string
}

type PromOptionFn func(config PromServerConfig) PromServerConfig

func WithPort(port string) PromOptionFn {
	return func(config PromServerConfig) PromServerConfig {
		config.port = port
		return config
	}
}

func WithServiceName(serviceName string) OptionFn {
	return func(config Config) Config {
		config.ServiceName = serviceName

		return config
	}
}

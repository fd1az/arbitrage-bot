package httpclient

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Default connection pool settings
	defaultDialKeepAlive         = 10 * time.Second
	defaultRequestTimeout        = 10 * time.Second
	defaultMaxIdleConns          = 0
	defaultMaxConnsPerHost       = 5
	defaultIdleConnTimeout       = 2 * time.Minute
	defaultExpectContinueTimeout = 100 * time.Millisecond

	// Metric names
	metricRequestCounter = "http_client_requests_total"
)

// Client is the interface for making HTTP requests.
type Client interface {
	// NewRequest creates a new request with default options.
	NewRequest() Request
	// NewRequestWithOptions creates a new request with custom options.
	NewRequestWithOptions(opts ...RequestOption) Request
	// Do executes a request and returns the response.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

// InstrumentedClient wraps http.Client with OTEL instrumentation.
type InstrumentedClient struct {
	client         *http.Client
	requestCounter metric.Int64Counter
	providerName   string
	tracer         trace.Tracer
	baseURL        string
	defaultHeaders map[string]string
	logRequest     bool
	logResponse    bool
}

// NewInstrumentedClient creates a new instrumented HTTP client.
func NewInstrumentedClient(opts ...ClientOption) (Client, error) {
	options := NewClientOptions(opts...)

	// Create or use provided http.Client
	httpClient := options.client
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultRequestTimeout,
		}
	}

	// Configure transport
	if options.roundTripper != nil {
		httpClient.Transport = options.roundTripper
	} else if httpClient.Transport == nil {
		httpClient.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				KeepAlive: defaultDialKeepAlive,
			}).DialContext,
			MaxIdleConns:          defaultMaxIdleConns,
			MaxConnsPerHost:       defaultMaxConnsPerHost,
			IdleConnTimeout:       defaultIdleConnTimeout,
			ExpectContinueTimeout: defaultExpectContinueTimeout,
			DisableKeepAlives:     false,
		}
	}

	// Set timeout if specified
	if options.requestTimeout != nil {
		httpClient.Timeout = *options.requestTimeout
	}

	// Wrap transport with OTEL instrumentation
	httpClient.Transport = otelhttp.NewTransport(
		httpClient.Transport,
		otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
			return otelhttptrace.NewClientTrace(ctx)
		}),
	)

	// Set provider name
	providerName := options.providerName
	if providerName == "" {
		providerName = "default"
	}

	// Get or create meter provider
	meterProvider := options.meterProvider
	if meterProvider == nil {
		meterProvider = otel.GetMeterProvider()
	}

	// Create meter and counter
	meter := meterProvider.Meter(
		"instrumented_http_client",
		metric.WithInstrumentationAttributes(attribute.String("provider", providerName)),
	)

	requestCounter, err := meter.Int64Counter(
		metricRequestCounter,
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	// Get tracer
	tracer := options.tracer
	if tracer == nil {
		tracer = otel.GetTracerProvider().Tracer("instrumented_http_client")
	}

	return &InstrumentedClient{
		client:         httpClient,
		requestCounter: requestCounter,
		providerName:   providerName,
		tracer:         tracer,
		baseURL:        options.baseURL,
		defaultHeaders: options.headers,
		logRequest:     options.logRequest,
		logResponse:    options.logResponse,
	}, nil
}

// NewRequest creates a new request builder with default options.
func (c *InstrumentedClient) NewRequest() Request {
	return c.NewRequestWithOptions()
}

// NewRequestWithOptions creates a new request builder with custom options.
func (c *InstrumentedClient) NewRequestWithOptions(opts ...RequestOption) Request {
	reqOpts := NewRequestOptions(opts...)

	// Default error handler: nil means no custom handling
	errorHandler := reqOpts.responseErrorHandler

	return &requestBuilder{
		client:           c.client,
		requestCounter:   c.requestCounter,
		providerName:     c.providerName,
		tracer:           c.tracer,
		baseURL:          c.baseURL,
		headers:          copyHeaders(c.defaultHeaders),
		errorHandler:     errorHandler,
		labels:           reqOpts.labels,
		excludeHeaders:   reqOpts.excludeHeaders,
		enableLogHeaders: reqOpts.enableLogHeaders,
		logRequest:       c.logRequest,
		logResponse:      c.logResponse,
	}
}

// Do executes an http.Request directly.
func (c *InstrumentedClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.client.Do(req.WithContext(ctx))
}

// copyHeaders creates a copy of a headers map.
func copyHeaders(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ReadBody reads and returns the response body, or empty if error.
func ReadBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Package httpclient provides an instrumented HTTP client with OTEL tracing and metrics.
package httpclient

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// TraceOption specifies what to log in traces.
type TraceOption string

const (
	TraceRequest  TraceOption = "request"
	TraceResponse TraceOption = "response"
)

// ClientOptions holds configuration for the instrumented HTTP client.
type ClientOptions struct {
	client         *http.Client
	meterProvider  metric.MeterProvider
	providerName   string
	roundTripper   http.RoundTripper
	requestTimeout *time.Duration
	headers        map[string]string
	baseURL        string
	logRequest     bool
	logResponse    bool
	tracer         trace.Tracer
}

// ClientOption is a function that configures ClientOptions.
type ClientOption func(*ClientOptions)

// NewClientOptions creates ClientOptions from variadic options.
func NewClientOptions(opts ...ClientOption) *ClientOptions {
	options := &ClientOptions{}
	for _, o := range opts {
		o(options)
	}
	return options
}

// WithMeterProvider sets the OTEL meter provider.
func WithMeterProvider(mp metric.MeterProvider) ClientOption {
	return func(o *ClientOptions) {
		o.meterProvider = mp
	}
}

// WithProviderName sets the provider name for metrics and traces.
func WithProviderName(name string) ClientOption {
	return func(o *ClientOptions) {
		o.providerName = name
	}
}

// WithRoundTripper sets a custom HTTP transport.
func WithRoundTripper(rt http.RoundTripper) ClientOption {
	return func(o *ClientOptions) {
		o.roundTripper = rt
	}
}

// WithRequestTimeout sets the request timeout.
func WithRequestTimeout(timeout time.Duration) ClientOption {
	return func(o *ClientOptions) {
		o.requestTimeout = &timeout
	}
}

// WithHeaders sets default headers for all requests.
func WithHeaders(headers map[string]string) ClientOption {
	return func(o *ClientOptions) {
		o.headers = headers
	}
}

// WithBaseURL sets the base URL for all requests.
func WithBaseURL(url string) ClientOption {
	return func(o *ClientOptions) {
		o.baseURL = url
	}
}

// WithTraceOptions enables request/response body logging to traces.
func WithTraceOptions(tracer trace.Tracer, opts ...TraceOption) ClientOption {
	return func(o *ClientOptions) {
		o.tracer = tracer
		for _, opt := range opts {
			switch opt {
			case TraceRequest:
				o.logRequest = true
			case TraceResponse:
				o.logResponse = true
			}
		}
	}
}

// RequestOptions holds per-request configuration.
type RequestOptions struct {
	responseErrorHandler ResponseErrorHandler
	labels               []*Label
	excludeHeaders       []string
	enableLogHeaders     bool
}

// RequestOption configures a single request.
type RequestOption func(*RequestOptions)

// NewRequestOptions creates RequestOptions from variadic options.
func NewRequestOptions(opts ...RequestOption) *RequestOptions {
	options := &RequestOptions{}
	for _, o := range opts {
		o(options)
	}
	if options.labels == nil {
		options.labels = make([]*Label, 0)
	}
	return options
}

// ResponseErrorHandler is a function that determines if a response is an error.
type ResponseErrorHandler func(statusCode int, body []byte) error

// WithResponseErrorHandler sets a custom error handler for responses.
func WithResponseErrorHandler(handler ResponseErrorHandler) RequestOption {
	return func(o *RequestOptions) {
		o.responseErrorHandler = handler
	}
}

// Label is a key-value pair for metrics/traces.
type Label struct {
	Key   string
	Value string
}

// NewLabel creates a new label.
func NewLabel(key, value string) *Label {
	return &Label{Key: key, Value: value}
}

// WithLabels sets labels for the request.
func WithLabels(labels ...*Label) RequestOption {
	return func(o *RequestOptions) {
		o.labels = labels
	}
}

// WithHeadersLogConfig configures header logging.
func WithHeadersLogConfig(enable bool, exclude ...string) RequestOption {
	return func(o *RequestOptions) {
		o.enableLogHeaders = enable
		o.excludeHeaders = exclude
	}
}

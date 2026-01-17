package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Request is the interface for building and executing HTTP requests.
type Request interface {
	// HTTP methods
	Get(ctx context.Context, url string) (*Response, error)
	Post(ctx context.Context, url string) (*Response, error)
	Put(ctx context.Context, url string) (*Response, error)
	Patch(ctx context.Context, url string) (*Response, error)
	Delete(ctx context.Context, url string) (*Response, error)

	// Configuration
	SetBody(body interface{}) Request
	SetHeader(key, value string) Request
	SetHeaders(headers map[string]string) Request
	SetQueryParam(key, value string) Request
	SetQueryParams(params map[string]string) Request
	SetResult(result interface{}) Request
}

// Response wraps http.Response with additional helpers.
type Response struct {
	*http.Response
	body   []byte
	result interface{}
}

// Body returns the response body as bytes.
func (r *Response) Body() []byte {
	return r.body
}

// String returns the response body as string.
func (r *Response) String() string {
	return string(r.body)
}

// IsError returns true if the status code indicates an error (>= 400).
func (r *Response) IsError() bool {
	return r.StatusCode >= 400
}

// IsSuccess returns true if the status code indicates success (< 400).
func (r *Response) IsSuccess() bool {
	return r.StatusCode < 400
}

// Result returns the unmarshaled result.
func (r *Response) Result() interface{} {
	return r.result
}

// requestBuilder implements Request.
type requestBuilder struct {
	client           *http.Client
	requestCounter   metric.Int64Counter
	providerName     string
	tracer           trace.Tracer
	baseURL          string
	headers          map[string]string
	queryParams      map[string]string
	body             interface{}
	result           interface{}
	errorHandler     ResponseErrorHandler
	labels           []*Label
	excludeHeaders   []string
	enableLogHeaders bool
	logRequest       bool
	logResponse      bool
}

// Get executes a GET request.
func (r *requestBuilder) Get(ctx context.Context, url string) (*Response, error) {
	return r.execute(ctx, http.MethodGet, url)
}

// Post executes a POST request.
func (r *requestBuilder) Post(ctx context.Context, url string) (*Response, error) {
	return r.execute(ctx, http.MethodPost, url)
}

// Put executes a PUT request.
func (r *requestBuilder) Put(ctx context.Context, url string) (*Response, error) {
	return r.execute(ctx, http.MethodPut, url)
}

// Patch executes a PATCH request.
func (r *requestBuilder) Patch(ctx context.Context, url string) (*Response, error) {
	return r.execute(ctx, http.MethodPatch, url)
}

// Delete executes a DELETE request.
func (r *requestBuilder) Delete(ctx context.Context, url string) (*Response, error) {
	return r.execute(ctx, http.MethodDelete, url)
}

// SetBody sets the request body (will be JSON encoded if struct/map).
func (r *requestBuilder) SetBody(body interface{}) Request {
	r.body = body
	return r
}

// SetHeader sets a single header.
func (r *requestBuilder) SetHeader(key, value string) Request {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}
	r.headers[key] = value
	return r
}

// SetHeaders sets multiple headers.
func (r *requestBuilder) SetHeaders(headers map[string]string) Request {
	for k, v := range headers {
		r.SetHeader(k, v)
	}
	return r
}

// SetQueryParam sets a single query parameter.
func (r *requestBuilder) SetQueryParam(key, value string) Request {
	if r.queryParams == nil {
		r.queryParams = make(map[string]string)
	}
	r.queryParams[key] = value
	return r
}

// SetQueryParams sets multiple query parameters.
func (r *requestBuilder) SetQueryParams(params map[string]string) Request {
	for k, v := range params {
		r.SetQueryParam(k, v)
	}
	return r
}

// SetResult sets the result struct for JSON unmarshaling.
func (r *requestBuilder) SetResult(result interface{}) Request {
	r.result = result
	return r
}

// execute performs the HTTP request with instrumentation.
func (r *requestBuilder) execute(ctx context.Context, method, url string) (*Response, error) {
	// Start span
	ctx, span := r.tracer.Start(ctx, "http.request",
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", url),
			attribute.String("provider", r.providerName),
		),
	)
	defer span.End()

	// Build full URL
	fullURL := url
	if r.baseURL != "" && !strings.HasPrefix(url, "http") {
		fullURL = strings.TrimSuffix(r.baseURL, "/") + "/" + strings.TrimPrefix(url, "/")
	}

	// Add query params
	if len(r.queryParams) > 0 {
		params := make([]string, 0, len(r.queryParams))
		for k, v := range r.queryParams {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		separator := "?"
		if strings.Contains(fullURL, "?") {
			separator = "&"
		}
		fullURL = fullURL + separator + strings.Join(params, "&")
	}

	// Build request body
	var bodyReader io.Reader
	if r.body != nil {
		switch b := r.body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(b)
		case string:
			bodyReader = strings.NewReader(b)
		case io.Reader:
			bodyReader = b
		default:
			// JSON encode
			jsonBody, err := json.Marshal(b)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to marshal body")
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
			if r.headers == nil {
				r.headers = make(map[string]string)
			}
			if _, ok := r.headers["Content-Type"]; !ok {
				r.headers["Content-Type"] = "application/json"
			}
		}

		// Log request body to trace
		if r.logRequest {
			if bodyBytes, ok := r.body.([]byte); ok {
				span.AddEvent("request.body", trace.WithAttributes(
					attribute.String("http.request_body", string(bodyBytes)),
				))
			} else if bodyStr, ok := r.body.(string); ok {
				span.AddEvent("request.body", trace.WithAttributes(
					attribute.String("http.request_body", bodyStr),
				))
			}
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	// Log headers to trace
	if r.enableLogHeaders {
		r.logHeaders(span, req.Header)
	}

	// Execute request
	resp, err := r.client.Do(req)
	if err != nil {
		r.recordError(ctx, span, err)
		return nil, err
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response body to trace
	if r.logResponse {
		span.AddEvent("response.body", trace.WithAttributes(
			attribute.String("http.response_body", string(body)),
		))
	}

	// Build response
	response := &Response{
		Response: resp,
		body:     body,
	}

	// Unmarshal result if set
	if r.result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, r.result); err != nil {
			span.RecordError(err)
			// Don't fail the request, just log the error
		} else {
			response.result = r.result
		}
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.String("http.error.status", resp.Status),
		)
	}

	// Run custom error handler
	if r.errorHandler != nil {
		if handlerErr := r.errorHandler(resp.StatusCode, body); handlerErr != nil {
			r.recordMetrics(ctx, false)
			span.SetStatus(codes.Error, handlerErr.Error())
			return response, handlerErr
		}
	}

	// Record success metrics
	r.recordMetrics(ctx, !response.IsError())

	return response, nil
}

// recordError logs network errors to the span.
func (r *requestBuilder) recordError(ctx context.Context, span trace.Span, err error) {
	span.RecordError(err)

	var netErr net.Error
	if errors.Is(err, context.Canceled) {
		span.SetAttributes(attribute.Bool("context.cancelled", true))
	}
	if errors.As(err, &netErr) && netErr.Timeout() {
		span.SetAttributes(attribute.Bool("request.timeout", true))
	}

	span.SetStatus(codes.Error, err.Error())
	r.recordMetrics(ctx, false)
}

// recordMetrics increments the request counter.
func (r *requestBuilder) recordMetrics(ctx context.Context, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("provider", r.providerName),
		attribute.Bool("success", success),
	}

	// Add custom labels
	for _, label := range r.labels {
		attrs = append(attrs, attribute.String(label.Key, label.Value))
	}

	r.requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// logHeaders adds request headers to the trace span.
func (r *requestBuilder) logHeaders(span trace.Span, headers http.Header) {
	excludeMap := make(map[string]bool)
	for _, h := range r.excludeHeaders {
		excludeMap[strings.ToLower(h)] = true
	}

	attrs := make([]attribute.KeyValue, 0)
	for k, values := range headers {
		key := strings.ToLower(k)
		headerKey := fmt.Sprintf("http.request.header.%s", key)
		headerVal := ""
		if len(values) > 0 {
			headerVal = values[0]
		}

		if excludeMap[key] {
			attrs = append(attrs, attribute.String(headerKey, "*****"))
		} else {
			attrs = append(attrs, attribute.String(headerKey, headerVal))
		}
	}

	if len(attrs) > 0 {
		span.AddEvent("request.headers", trace.WithAttributes(attrs...))
	}
}

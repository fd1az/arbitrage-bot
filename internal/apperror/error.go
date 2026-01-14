package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// AppError implements the error interface and provides structured error handling
type AppError struct {
	Code       Code      `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"statusCode"`
	Context    string    `json:"context,omitempty"`
	TraceID    string    `json:"traceId,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	cause      error     // unexported to maintain encapsulation
	stack      []uintptr // stack trace
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %s (code: %s, context: %s)", e.Code, e.Message, e.Code, e.Context)
	}
	return fmt.Sprintf("%s: %s (code: %s)", e.Code, e.Message, e.Code)
}

// Unwrap implements the errors.Unwrap interface
func (e *AppError) Unwrap() error {
	return e.cause
}

// Is implements errors.Is interface for error comparison
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithTraceID sets the trace ID for distributed tracing
func (e *AppError) WithTraceID(traceID string) *AppError {
	e.TraceID = traceID
	return e
}

// ToResponse serializes the error for HTTP response
func (e *AppError) ToResponse() map[string]interface{} {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":      e.Code,
			"message":   e.Message,
			"timestamp": e.Timestamp.Format(time.RFC3339),
		},
	}

	if e.Context != "" {
		resp["error"].(map[string]interface{})["context"] = e.Context
	}

	if e.TraceID != "" {
		resp["error"].(map[string]interface{})["traceId"] = e.TraceID
	}

	return resp
}

// ToLog serializes the error for logging with stack trace
func (e *AppError) ToLog() map[string]interface{} {
	log := map[string]interface{}{
		"code":       e.Code,
		"message":    e.Message,
		"statusCode": e.StatusCode,
		"timestamp":  e.Timestamp.Format(time.RFC3339),
	}

	if e.Context != "" {
		log["context"] = e.Context
	}

	if e.TraceID != "" {
		log["traceId"] = e.TraceID
	}

	if e.cause != nil {
		log["cause"] = e.cause.Error()
	}

	if len(e.stack) > 0 {
		log["stack"] = e.formatStack()
	}

	return log
}

// formatStack formats the stack trace
func (e *AppError) formatStack() string {
	var sb strings.Builder
	frames := runtime.CallersFrames(e.stack)
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") {
			sb.WriteString(fmt.Sprintf("\n\t%s:%d %s", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}
	return sb.String()
}

// captureStack captures the current stack trace
func captureStack() []uintptr {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	return pcs[:n]
}

// New creates a new AppError with the given code and options
func New(code Code, opts ...Option) *AppError {
	err := &AppError{
		Code:       code,
		Message:    messages[code],
		StatusCode: getDefaultStatusCode(code),
		Timestamp:  time.Now(),
		stack:      captureStack(),
	}

	// Apply options
	for _, opt := range opts {
		opt(err)
	}

	// If message wasn't set by options and isn't in messages map, use code as message
	if err.Message == "" {
		err.Message = string(code)
	}

	return err
}

// Option is a functional option for AppError
type Option func(*AppError)

// WithMessage sets a custom message
func WithMessage(message string) Option {
	return func(e *AppError) {
		e.Message = message
	}
}

// WithContext adds context information
func WithContext(context string) Option {
	return func(e *AppError) {
		e.Context = context
	}
}

// WithStatusCode sets a custom HTTP status code
func WithStatusCode(statusCode int) Option {
	return func(e *AppError) {
		e.StatusCode = statusCode
	}
}

// WithCause wraps an underlying error
func WithCause(cause error) Option {
	return func(e *AppError) {
		e.cause = cause
	}
}

// Factory methods for common error types

// NotFound creates a not found error
func NotFound(code Code, context string) *AppError {
	return New(code, WithContext(context), WithStatusCode(http.StatusNotFound))
}

// Validation creates a validation error
func Validation(code Code, context string) *AppError {
	return New(code, WithContext(context), WithStatusCode(http.StatusBadRequest))
}

// Unauthorized creates an unauthorized error
func Unauthorized(code Code, context string) *AppError {
	return New(code, WithContext(context), WithStatusCode(http.StatusUnauthorized))
}

// Forbidden creates a forbidden error
func Forbidden(code Code, context string) *AppError {
	return New(code, WithContext(context), WithStatusCode(http.StatusForbidden))
}

// Conflict creates a conflict error
func Conflict(code Code, context string) *AppError {
	return New(code, WithContext(context), WithStatusCode(http.StatusConflict))
}

// Internal creates an internal server error
func Internal(code Code, context string, cause error) *AppError {
	return New(code, WithContext(context), WithCause(cause), WithStatusCode(http.StatusInternalServerError))
}

// External creates an external service error
func External(code Code, context string, cause error) *AppError {
	return New(code, WithContext(context), WithCause(cause), WithStatusCode(http.StatusServiceUnavailable))
}

// Wrap wraps a standard error into AppError
func Wrap(err error, code Code, context string) *AppError {
	if err == nil {
		return nil
	}

	// If it's already an AppError, return it
	var appErr *AppError
	if errors.As(err, &appErr) {
		if context != "" && appErr.Context == "" {
			appErr.Context = context
		}
		return appErr
	}

	// Create new AppError wrapping the original
	return Internal(code, context, err)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetCode extracts the error code from an error
func GetCode(err error) Code {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeUnknownError
}

// getDefaultStatusCode determines the HTTP status code based on the error code
func getDefaultStatusCode(code Code) int {
	switch {
	// Authentication & Authorization
	case strings.Contains(string(code), "UNAUTHORIZED"):
		return http.StatusUnauthorized

	// Not Found errors
	case strings.Contains(string(code), "NOT_FOUND"):
		return http.StatusNotFound

	// Validation errors
	case strings.Contains(string(code), "INVALID"):
		return http.StatusBadRequest

	// Connection errors
	case strings.Contains(string(code), "CONNECTION"),
		strings.Contains(string(code), "TIMEOUT"):
		return http.StatusServiceUnavailable

	// Rate limit
	case code == CodeRateLimitExceeded:
		return http.StatusTooManyRequests

	default:
		return http.StatusInternalServerError
	}
}

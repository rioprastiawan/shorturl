package httpx

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is a failure with a stable machine-readable code. Handlers return
// these; the middleware renders them.
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string][]string

	// cause is logged but never sent to the client.
	cause error
}

func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *APIError) Unwrap() error { return e.cause }

// WithCause attaches an internal error for logging without changing what the
// client sees.
func (e *APIError) WithCause(err error) *APIError {
	clone := *e
	clone.cause = err
	return &clone
}

// AsAPIError converts any error into something renderable, defaulting to an
// opaque 500.
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "An unexpected error occurred",
		cause:   err,
	}
}

func newError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// Common failures. Handlers should prefer these over inventing new codes so
// clients can switch on a small, documented set.
var (
	ErrUnauthorized = newError(http.StatusUnauthorized, "unauthorized", "Authentication required")
	ErrForbidden    = newError(http.StatusForbidden, "forbidden", "You do not have access to this resource")
	ErrNotFound     = newError(http.StatusNotFound, "not_found", "Resource not found")
	ErrConflict     = newError(http.StatusConflict, "conflict", "The resource already exists")
	ErrRateLimited  = newError(http.StatusTooManyRequests, "rate_limited", "Too many requests")
	ErrSetupDone    = newError(http.StatusConflict, "setup_completed", "Initial setup has already been completed")
)

// BadRequest reports a malformed request that never reached validation.
func BadRequest(message string) *APIError {
	return newError(http.StatusBadRequest, "bad_request", message)
}

// Invalid reports field-level validation failures.
func Invalid(fields map[string][]string) *APIError {
	return &APIError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_error",
		Message: "The request is invalid",
		Fields:  fields,
	}
}

// Forbiddenf reports an authorization failure with a specific reason.
func Forbiddenf(format string, args ...any) *APIError {
	return newError(http.StatusForbidden, "forbidden", fmt.Sprintf(format, args...))
}

// Conflictf reports a uniqueness or state conflict.
func Conflictf(code, format string, args ...any) *APIError {
	return newError(http.StatusConflict, code, fmt.Sprintf(format, args...))
}

// Internal wraps an unexpected failure.
func Internal(err error) *APIError {
	return (&APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "An unexpected error occurred",
	}).WithCause(err)
}

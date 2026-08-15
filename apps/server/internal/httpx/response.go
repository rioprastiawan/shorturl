// Package httpx holds the shared HTTP envelope: how every handler writes a
// success, a list, or an error. Keeping it in one place is what makes the
// public API contract in §14.9 of the plan actually consistent.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Meta carries pagination information alongside a list response.
type Meta struct {
	NextCursor *string `json:"next_cursor"`
	Total      *int64  `json:"total,omitempty"`
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type listEnvelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the shape every failure takes.
type ErrorBody struct {
	Code      string              `json:"code"`
	Message   string              `json:"message"`
	Fields    map[string][]string `json:"fields,omitempty"`
	RequestID string              `json:"request_id,omitempty"`
}

// Data writes a single-resource response: {"data": {...}}.
func Data(w http.ResponseWriter, status int, v any) {
	write(w, status, dataEnvelope{Data: v})
}

// List writes a collection response with pagination metadata. A nil slice is
// still rendered as [] so clients never have to handle null.
func List(w http.ResponseWriter, v any, meta Meta) {
	if v == nil {
		v = []any{}
	}
	write(w, http.StatusOK, listEnvelope{Data: v, Meta: meta})
}

// NoContent ends a request that has nothing to return.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error renders an APIError. Anything that is not already an APIError is
// treated as an internal failure: the real cause is logged, and the client is
// told nothing beyond a request ID, so stack traces and SQL never leak.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)
	requestID := middleware.GetReqID(r.Context())

	if apiErr.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			slog.String("request_id", requestID),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
	}

	write(w, apiErr.Status, errorEnvelope{Error: ErrorBody{
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		Fields:    apiErr.Fields,
		RequestID: requestID,
	}})
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already sent, so this can only be logged.
		slog.Error("encoding response body", slog.String("error", err.Error()))
	}
}

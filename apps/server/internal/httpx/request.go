package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// MaxBodyBytes caps request bodies. Link metadata is documented as 8 KB, and
// nothing else the API accepts is large, so 1 MB is generous.
const MaxBodyBytes = 1 << 20

// Decode reads a JSON body into v, rejecting unknown fields so a typo in an
// integration's payload surfaces immediately instead of being ignored.
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mediaType, _, _ := strings.Cut(ct, ";"); strings.TrimSpace(mediaType) != "application/json" {
			return BadRequest("Content-Type must be application/json")
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError

		switch {
		case errors.As(err, &maxErr):
			return BadRequest("Request body is too large")
		case errors.Is(err, io.EOF):
			return BadRequest("Request body is empty")
		case errors.As(err, &syntaxErr):
			return BadRequest("Request body is not valid JSON")
		case errors.As(err, &typeErr):
			return Invalid(map[string][]string{
				typeErr.Field: {"must be a " + typeErr.Type.String()},
			})
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return Invalid(map[string][]string{field: {"unknown field"}})
		default:
			return BadRequest("Request body could not be read")
		}
	}

	// A second value in the stream means the caller sent concatenated JSON.
	if dec.More() {
		return BadRequest("Request body must contain a single JSON object")
	}
	return nil
}

// UUIDParam parses a path parameter, returning a 404 rather than a 400 for a
// malformed ID: to the caller, an unparseable ID and a missing resource are
// the same thing, and distinguishing them leaks whether IDs exist.
func UUIDParam(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	return id, nil
}

// QueryInt reads a bounded integer query parameter.
func QueryInt(r *http.Request, key string, fallback, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

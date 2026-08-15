// Package validate accumulates field-level errors into the shape the API
// error envelope expects.
package validate

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Errors collects per-field messages.
type Errors struct {
	fields map[string][]string
}

// New returns an empty collector.
func New() *Errors { return &Errors{fields: make(map[string][]string)} }

// Add records a message against a field.
func (e *Errors) Add(field, format string, args ...any) {
	e.fields[field] = append(e.fields[field], fmt.Sprintf(format, args...))
}

// Check records the message when ok is false. Reads naturally at the call
// site: v.Check(len(name) > 0, "name", "is required").
func (e *Errors) Check(ok bool, field, format string, args ...any) {
	if !ok {
		e.Add(field, format, args...)
	}
}

// HasErrors reports whether anything was recorded.
func (e *Errors) HasErrors() bool { return len(e.fields) > 0 }

// Fields returns the collected messages, or nil when there are none.
func (e *Errors) Fields() map[string][]string {
	if len(e.fields) == 0 {
		return nil
	}
	return e.fields
}

// Required records an error when the trimmed value is empty, and returns the
// trimmed value so callers can use it directly.
func (e *Errors) Required(field, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		e.Add(field, "is required")
	}
	return trimmed
}

// Length checks a rune count, not a byte count, so a name in a non-Latin
// script is not rejected for being "too long".
func (e *Errors) Length(field, value string, min, max int) {
	n := utf8.RuneCountInString(value)
	if n < min {
		e.Add(field, "must be at least %d characters", min)
	}
	if n > max {
		e.Add(field, "must be at most %d characters", max)
	}
}

// Email checks a syntactically valid address and returns it lowercased.
func (e *Errors) Email(field, value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		e.Add(field, "is required")
		return trimmed
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		e.Add(field, "must be a valid email address")
	}
	return trimmed
}

// Password enforces a length floor only.
//
// Composition rules ("one uppercase, one symbol") push people toward
// predictable substitutions and are not what NIST recommends; length is what
// actually resists guessing.
func (e *Errors) Password(field, value string) {
	if n := utf8.RuneCountInString(value); n < 10 {
		e.Add(field, "must be at least 10 characters")
	} else if n > 256 {
		e.Add(field, "must be at most 256 characters")
	}
}

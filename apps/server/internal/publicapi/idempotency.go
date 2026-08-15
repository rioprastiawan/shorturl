package publicapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IdempotencyHeader is the header an integration sends to make a retry safe
// (plan §14.3).
const IdempotencyHeader = "Idempotency-Key"

// MaxIdempotencyKeyLength matches the idempotency_keys.idempotency_key column.
const MaxIdempotencyKeyLength = 255

// IdempotencyTTL is the retention window from plan §14.3. Long enough to cover
// any sane retry schedule, short enough that the table stays small.
const IdempotencyTTL = 24 * time.Hour

// Decision is what a POST /links should do given the stored idempotency state.
type Decision int

const (
	// DecisionCreate: no live record for this key — do the work.
	DecisionCreate Decision = iota
	// DecisionReplay: same key, same request — return the original link with 200.
	DecisionReplay
	// DecisionConflict: same key, different request — refuse.
	DecisionConflict
	// DecisionLinkGone: the record survives but the link it points at does not.
	DecisionLinkGone
)

func (d Decision) String() string {
	switch d {
	case DecisionCreate:
		return "create"
	case DecisionReplay:
		return "replay"
	case DecisionConflict:
		return "conflict"
	case DecisionLinkGone:
		return "link_gone"
	default:
		return "unknown"
	}
}

// Record is the stored idempotency state in the shape the decision needs. It
// is deliberately not store.IdempotencyKey, so Decide stays testable without a
// database and without a not-found sentinel error.
type Record struct {
	Found       bool
	RequestHash string
	LinkID      *uuid.UUID
}

// Decide is the whole idempotency rule, as a pure function.
func Decide(rec Record, requestHash string) Decision {
	if !rec.Found {
		return DecisionCreate
	}
	if rec.RequestHash != requestHash {
		return DecisionConflict
	}
	if rec.LinkID == nil {
		return DecisionLinkGone
	}
	return DecisionReplay
}

// CanonicalHash fingerprints a request body so that a retry which differs only
// in formatting is recognised as the same request.
//
// Round-tripping through map[string]any is what does the work: json.Marshal
// emits map keys in sorted order, so key order and whitespace both disappear,
// at every level of nesting.
func CanonicalHash(body []byte) (string, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("canonicalise request body: %w", err)
	}

	canonical, err := json.Marshal(decoded)
	if err != nil {
		return "", fmt.Errorf("canonicalise request body: %w", err)
	}

	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// ValidIdempotencyKey reports whether a key fits the column. An over-long key
// is the caller's bug and must not become a 500 from the database.
func ValidIdempotencyKey(key string) bool {
	return len(key) <= MaxIdempotencyKeyLength
}

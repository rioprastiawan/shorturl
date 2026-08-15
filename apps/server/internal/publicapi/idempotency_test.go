package publicapi

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDecide(t *testing.T) {
	linkID := uuid.New()
	const hash = "a1b2c3"

	tests := []struct {
		name        string
		record      Record
		requestHash string
		want        Decision
	}{
		{
			name:        "no record creates",
			record:      Record{},
			requestHash: hash,
			want:        DecisionCreate,
		},
		{
			name:        "matching hash replays",
			record:      Record{Found: true, RequestHash: hash, LinkID: &linkID},
			requestHash: hash,
			want:        DecisionReplay,
		},
		{
			name:        "different hash conflicts",
			record:      Record{Found: true, RequestHash: "different", LinkID: &linkID},
			requestHash: hash,
			want:        DecisionConflict,
		},
		{
			name:        "matching hash without a link is gone",
			record:      Record{Found: true, RequestHash: hash},
			requestHash: hash,
			want:        DecisionLinkGone,
		},
		{
			name:        "a conflicting record without a link still conflicts",
			record:      Record{Found: true, RequestHash: "different"},
			requestHash: hash,
			want:        DecisionConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.record, tc.requestHash); got != tc.want {
				t.Errorf("Decide() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCanonicalHashIgnoresFormatting(t *testing.T) {
	// Every one of these is the same request as far as an integration is
	// concerned, so a retry that differs only in serialisation must replay
	// rather than conflict.
	equivalent := []string{
		`{"a":1,"b":2}`,
		`{"b":2,"a":1}`,
		"{\n  \"a\" : 1,\n  \"b\" : 2\n}",
		`  {"b": 2,   "a": 1}  `,
	}

	want, err := CanonicalHash([]byte(equivalent[0]))
	if err != nil {
		t.Fatalf("CanonicalHash: %v", err)
	}

	for _, body := range equivalent[1:] {
		got, err := CanonicalHash([]byte(body))
		if err != nil {
			t.Fatalf("CanonicalHash(%q): %v", body, err)
		}
		if got != want {
			t.Errorf("CanonicalHash(%q) = %s, want %s", body, got, want)
		}
	}
}

func TestCanonicalHashNestedOrder(t *testing.T) {
	// metadata is a caller-owned object, and Go's map iteration means an
	// integration has no control over the order its own encoder emits.
	a := `{"destination_url":"https://x.test/a","metadata":{"source":"erp","id":"12345"}}`
	b := `{"metadata":{"id":"12345","source":"erp"},"destination_url":"https://x.test/a"}`

	hashA, err := CanonicalHash([]byte(a))
	if err != nil {
		t.Fatalf("CanonicalHash: %v", err)
	}
	hashB, err := CanonicalHash([]byte(b))
	if err != nil {
		t.Fatalf("CanonicalHash: %v", err)
	}
	if hashA != hashB {
		t.Errorf("nested key order changed the hash: %s != %s", hashA, hashB)
	}
}

func TestCanonicalHashDistinguishesRequests(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"different value", `{"a":1}`, `{"a":2}`},
		{"different key", `{"a":1}`, `{"b":1}`},
		{"extra field", `{"a":1}`, `{"a":1,"b":2}`},
		{"string versus number", `{"a":"1"}`, `{"a":1}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hashA, err := CanonicalHash([]byte(tc.a))
			if err != nil {
				t.Fatalf("CanonicalHash: %v", err)
			}
			hashB, err := CanonicalHash([]byte(tc.b))
			if err != nil {
				t.Fatalf("CanonicalHash: %v", err)
			}
			if hashA == hashB {
				t.Errorf("%q and %q hash identically", tc.a, tc.b)
			}
		})
	}
}

func TestCanonicalHashRejectsNonObjects(t *testing.T) {
	for _, body := range []string{"", "[1,2]", `"text"`, "not json"} {
		if _, err := CanonicalHash([]byte(body)); err == nil {
			t.Errorf("CanonicalHash(%q) succeeded, want an error", body)
		}
	}
}

func TestValidIdempotencyKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"typical", "invoice-8f4e2b8a", true},
		{"empty", "", true},
		{"at the limit", strings.Repeat("k", MaxIdempotencyKeyLength), true},
		{"over the limit", strings.Repeat("k", MaxIdempotencyKeyLength+1), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidIdempotencyKey(tc.key); got != tc.want {
				t.Errorf("ValidIdempotencyKey(len %d) = %v, want %v", len(tc.key), got, tc.want)
			}
		})
	}
}

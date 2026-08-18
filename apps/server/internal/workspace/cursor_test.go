package workspace

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWorkspaceCursorRoundTrip(t *testing.T) {
	wantTime := time.Date(2026, time.August, 18, 4, 30, 12, 987654321, time.FixedZone("WIB", 7*60*60))
	wantID := uuid.New()

	gotTime, gotID, err := decodeWorkspaceCursor(encodeWorkspaceCursor(wantTime, wantID))
	if err != nil {
		t.Fatalf("decodeWorkspaceCursor() error = %v", err)
	}
	if !gotTime.Equal(wantTime) {
		t.Fatalf("decoded time = %v, want %v", gotTime, wantTime)
	}
	if gotID != wantID {
		t.Fatalf("decoded id = %v, want %v", gotID, wantID)
	}
}

func TestWorkspaceCursorRejectsInvalidValues(t *testing.T) {
	for _, cursor := range []string{"not-base64!", "bm90LWEtY3Vyc29y"} {
		if _, _, err := decodeWorkspaceCursor(cursor); err == nil {
			t.Fatalf("decodeWorkspaceCursor(%q) unexpectedly succeeded", cursor)
		}
	}
}

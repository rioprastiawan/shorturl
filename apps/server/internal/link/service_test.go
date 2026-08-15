package link

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	want := time.Date(2026, 8, 15, 5, 30, 0, 123456789, time.UTC)
	wantID := uuid.MustParse("0198c7e2-1111-4222-8333-444455556666")

	gotTime, gotID, err := decodeCursor(encodeCursor(want, wantID))
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}
	if !gotTime.Equal(want) {
		t.Errorf("time = %v, want %v", gotTime, want)
	}
	if gotID != wantID {
		t.Errorf("id = %v, want %v", gotID, wantID)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for name, cursor := range map[string]string{
		"not base64":    "!!!!",
		"no separator":  "dGltZXN0YW1w",
		"bad timestamp": encodeRaw("nonsense|0198c7e2-1111-4222-8333-444455556666"),
		"bad uuid":      encodeRaw("2026-08-15T05:30:00Z|not-a-uuid"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeCursor(cursor); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestIsValidRedirectType(t *testing.T) {
	valid := []int16{301, 302, 307, 308}
	for _, code := range valid {
		if !isValidRedirectType(code) {
			t.Errorf("isValidRedirectType(%d) = false, want true", code)
		}
	}
	for _, code := range []int16{0, 200, 303, 304, 400, 500} {
		if isValidRedirectType(code) {
			t.Errorf("isValidRedirectType(%d) = true, want false", code)
		}
	}
}

func TestIsValidStatus(t *testing.T) {
	for _, s := range []string{"active", "disabled", "archived"} {
		if !isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "ACTIVE", "deleted", "enabled"} {
		if isValidStatus(s) {
			t.Errorf("isValidStatus(%q) = true, want false", s)
		}
	}
}

func TestViewShortURL(t *testing.T) {
	v := View{Hostname: "go.example.com"}
	v.Link.Slug = "aB7kP2"
	if got, want := v.ShortURL(), "https://go.example.com/aB7kP2"; got != want {
		t.Errorf("ShortURL() = %q, want %q", got, want)
	}
}

func TestToDTOHidesPasswordAndReportsFlag(t *testing.T) {
	hash := "$argon2id$whatever"
	v := View{Hostname: "go.example.com"}
	v.Link.PasswordHash = &hash

	dto := ToDTO(v)
	if !dto.HasPassword {
		t.Error("HasPassword = false, want true when a hash is stored")
	}

	// The DTO type must not carry the hash at all; this is a compile-time
	// guarantee, but assert the flag flips correctly when absent too.
	v.Link.PasswordHash = nil
	if ToDTO(v).HasPassword {
		t.Error("HasPassword = true, want false when no hash is stored")
	}

	empty := ""
	v.Link.PasswordHash = &empty
	if ToDTO(v).HasPassword {
		t.Error("HasPassword = true for an empty hash, want false")
	}
}

func encodeRaw(s string) string {
	return encodeCursorRaw(s)
}

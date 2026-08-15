package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
)

func testConfig(secure bool) config.Config {
	return config.Config{
		SessionCookie: "shorturl_session",
		CookieSecure:  secure,
		SessionTTL:    30 * 24 * time.Hour,
	}
}

func readSetCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	cookies := (&http.Response{Header: rec.Header()}).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d Set-Cookie headers, want 1", len(cookies))
	}
	return cookies[0]
}

func TestSetSessionCookie(t *testing.T) {
	tests := []struct {
		name       string
		secure     bool
		wantSecure bool
	}{
		{"development is not secure-only", false, false},
		{"production is secure-only", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			expiresAt := time.Now().Add(time.Hour)
			SetSessionCookie(rec, testConfig(tt.secure), "token-value", expiresAt)

			cookie := readSetCookie(t, rec)
			switch {
			case cookie.Name != "shorturl_session":
				t.Errorf("Name = %q, want %q", cookie.Name, "shorturl_session")
			case cookie.Value != "token-value":
				t.Errorf("Value = %q, want %q", cookie.Value, "token-value")
			case cookie.Path != "/":
				t.Errorf("Path = %q, want %q", cookie.Path, "/")
			case !cookie.HttpOnly:
				t.Error("HttpOnly = false, want true: the token must be unreadable from JavaScript")
			case cookie.Secure != tt.wantSecure:
				t.Errorf("Secure = %v, want %v", cookie.Secure, tt.wantSecure)
			case cookie.SameSite != http.SameSiteLaxMode:
				t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
			case cookie.MaxAge <= 0:
				t.Errorf("MaxAge = %d, want a positive lifetime", cookie.MaxAge)
			}
		})
	}
}

func TestSetSessionCookieRejectsNegativeLifetime(t *testing.T) {
	rec := httptest.NewRecorder()
	// An already-expired session must not be sent as MaxAge<0, which browsers
	// read as "delete this cookie" rather than "this cookie is stale".
	SetSessionCookie(rec, testConfig(false), "token-value", time.Now().Add(-time.Hour))

	if got := readSetCookie(t, rec).MaxAge; got < 0 {
		t.Errorf("MaxAge = %d, want 0 or more", got)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, testConfig(true))

	cookie := readSetCookie(t, rec)
	switch {
	case cookie.Value != "":
		t.Errorf("Value = %q, want empty", cookie.Value)
	case cookie.MaxAge >= 0:
		t.Errorf("MaxAge = %d, want a negative value so the browser drops it", cookie.MaxAge)
	// The attributes must match the ones the cookie was set with, or the
	// browser treats this as a different cookie and keeps the live one.
	case cookie.Path != "/":
		t.Errorf("Path = %q, want %q", cookie.Path, "/")
	case !cookie.HttpOnly:
		t.Error("HttpOnly = false, want true")
	case !cookie.Secure:
		t.Error("Secure = false, want true")
	case cookie.SameSite != http.SameSiteLaxMode:
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
}

func TestReadCookie(t *testing.T) {
	svc := NewService(nil, testConfig(false))

	tests := []struct {
		name   string
		cookie *http.Cookie
		want   string
	}{
		{"present", &http.Cookie{Name: "shorturl_session", Value: "token-value"}, "token-value"},
		{"absent", nil, ""},
		{"different name", &http.Cookie{Name: "other", Value: "token-value"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}
			if got := svc.ReadCookie(r); got != tt.want {
				t.Errorf("ReadCookie() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

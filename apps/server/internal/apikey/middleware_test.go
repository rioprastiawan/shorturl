package apikey

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
)

func TestRequireScope(t *testing.T) {
	tests := []struct {
		name       string
		granted    []string
		required   string
		anonymous  bool
		wantStatus int
		wantCode   string
	}{
		{
			name:       "read key reading",
			granted:    []string{authctx.ScopeLinksRead},
			required:   authctx.ScopeLinksRead,
			wantStatus: http.StatusOK,
		},
		{
			name:       "read key writing",
			granted:    []string{authctx.ScopeLinksRead},
			required:   authctx.ScopeLinksWrite,
			wantStatus: http.StatusForbidden,
			wantCode:   "insufficient_scope",
		},
		{
			name:       "write key writing",
			granted:    []string{authctx.ScopeLinksWrite},
			required:   authctx.ScopeLinksWrite,
			wantStatus: http.StatusOK,
		},
		{
			name:       "write key reading",
			granted:    []string{authctx.ScopeLinksWrite},
			required:   authctx.ScopeLinksRead,
			wantStatus: http.StatusForbidden,
			wantCode:   "insufficient_scope",
		},
		{
			name:       "default key reading",
			granted:    DefaultScopes,
			required:   authctx.ScopeLinksRead,
			wantStatus: http.StatusOK,
		},
		{
			name:       "default key writing",
			granted:    DefaultScopes,
			required:   authctx.ScopeLinksWrite,
			wantStatus: http.StatusOK,
		},
		{
			name:       "default key has no analytics",
			granted:    DefaultScopes,
			required:   authctx.ScopeAnalyticsRead,
			wantStatus: http.StatusForbidden,
			wantCode:   "insufficient_scope",
		},
		{
			name:       "analytics key reading analytics",
			granted:    []string{authctx.ScopeAnalyticsRead},
			required:   authctx.ScopeAnalyticsRead,
			wantStatus: http.StatusOK,
		},
		{
			name:       "no scopes at all",
			granted:    nil,
			required:   authctx.ScopeLinksRead,
			wantStatus: http.StatusForbidden,
			wantCode:   "insufficient_scope",
		},
		{
			name:       "no principal on the context",
			anonymous:  true,
			required:   authctx.ScopeLinksRead,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			handler := RequireScope(tc.required)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			}))

			r := httptest.NewRequest(http.MethodGet, "/links", nil)
			if !tc.anonymous {
				r = r.WithContext(authctx.WithAPIKey(r.Context(), authctx.APIKeyPrincipal{
					ID:          uuid.New(),
					WorkspaceID: uuid.New(),
					Scopes:      tc.granted,
				}))
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if reached != (tc.wantStatus == http.StatusOK) {
				t.Errorf("handler reached = %v, want %v", reached, tc.wantStatus == http.StatusOK)
			}
			if tc.wantCode != "" && errorCode(t, w.Body.Bytes()) != tc.wantCode {
				t.Errorf("error code = %q, want %q", errorCode(t, w.Body.Bytes()), tc.wantCode)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		wantOK bool
	}{
		{"standard", "Bearer shr_live_abc", "shr_live_abc", true},
		{"lowercase scheme", "bearer shr_live_abc", "shr_live_abc", true},
		{"missing header", "", "", false},
		{"no scheme", "shr_live_abc", "", false},
		{"wrong scheme", "Basic shr_live_abc", "", false},
		{"empty token", "Bearer ", "", false},
		{"padded token", "Bearer   shr_live_abc  ", "shr_live_abc", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/links", nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}

			got, ok := BearerToken(r)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("BearerToken(%q) = %q, %v; want %q, %v", tc.header, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decoding error body %q: %v", body, err)
	}
	return envelope.Error.Code
}

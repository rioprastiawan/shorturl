package publicapi

import (
	"net/http"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRoutes pins the public contract from plan §14. Renaming a path here is a
// breaking change for every integration already deployed against it.
func TestRoutes(t *testing.T) {
	r := chi.NewRouter()
	NewHandler(nil, nil).Routes(r)

	var registered []string
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered = append(registered, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("walking routes: %v", err)
	}

	want := []string{
		"DELETE /{linkId}",
		"GET /",
		"GET /by-reference/{externalReference}",
		"GET /{linkId}",
		"PATCH /{linkId}",
		"POST /",
	}
	for _, route := range want {
		if !slices.Contains(registered, route) {
			t.Errorf("route %q is not registered; got %v", route, registered)
		}
	}
	if len(registered) != len(want) {
		t.Errorf("registered %d routes, want %d: %v", len(registered), len(want), registered)
	}
}

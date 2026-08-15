package workspace

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

// RequireWorkspace resolves {workspaceId} to the caller's membership and puts
// it on the context, so every handler below reads authorization from one place
// and scopes its queries to a workspace the caller demonstrably belongs to.
//
// A non-member gets 404, not 403: a 403 would confirm that the workspace
// exists, which lets an outsider enumerate IDs.
//
// Must be mounted inside the /{workspaceId} route, not on its parent — chi
// only fills URL parameters once the segment has been matched.
func RequireWorkspace(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID, err := httpx.UUIDParam(chi.URLParam(r, "workspaceId"))
			if err != nil {
				httpx.Error(w, r, err)
				return
			}

			ctx := r.Context()
			user := authctx.MustUser(ctx)

			role, err := svc.MemberRole(ctx, workspaceID, user.ID)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}

			ctx = authctx.WithMembership(ctx, authctx.Membership{
				WorkspaceID: workspaceID,
				UserID:      user.ID,
				Role:        role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

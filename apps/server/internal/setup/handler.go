package setup

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/auth"
	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
	"github.com/rioprastiawan/shorturl/apps/server/internal/workspace"
)

type completeRequest struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	WorkspaceName  string `json:"workspace_name"`
	DeploymentMode string `json:"deployment_mode"`
}

type statusDTO struct {
	Completed           bool   `json:"completed"`
	DeploymentMode      string `json:"deployment_mode"`
	RegistrationEnabled bool   `json:"registration_enabled"`
}

type indexingDTO struct {
	Enabled bool `json:"enabled"`
}

type completeDTO struct {
	User      auth.UserDTO  `json:"user"`
	Workspace workspace.DTO `json:"workspace"`
}

// Handler is the HTTP layer over Service.
type Handler struct {
	svc *Service
}

// NewHandler builds the setup handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers the wizard endpoints. Both paths are absolute within the
// API mount point, so the caller registers them alongside the other feature
// routes rather than under a /setup subtree.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/setup/status", h.status)
	r.Post("/setup", h.complete)
	r.Get("/system/indexing", h.indexing)
}

func (h *Handler) AdminRoutes(r chi.Router) {
	r.Put("/indexing", h.updateIndexing)
}

func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.svc.SearchIndexing(r.Context())
	if err != nil {
		http.Error(w, "could not read crawler policy", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if enabled {
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
}

func (h *Handler) indexing(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.svc.SearchIndexing(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, indexingDTO{Enabled: enabled})
}

func (h *Handler) updateIndexing(w http.ResponseWriter, r *http.Request) {
	if !authctx.MustUser(r.Context()).IsAdmin {
		httpx.Error(w, r, httpx.ErrForbidden)
		return
	}
	var req indexingDTO
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.SetSearchIndexing(r.Context(), req.Enabled); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, req)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	completed, err := h.svc.Status(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	mode, err := h.svc.DeploymentMode(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, statusDTO{Completed: completed, DeploymentMode: mode, RegistrationEnabled: mode == ModePublic})
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	v := validate.New()
	name, email := auth.ValidateAccount(v, req.Name, req.Email, req.Password)
	workspaceName := v.Required("workspace_name", req.WorkspaceName)
	if workspaceName != "" {
		v.Length("workspace_name", workspaceName, workspace.NameMinLength, workspace.NameMaxLength)
	}
	if req.DeploymentMode != ModeInternal && req.DeploymentMode != ModePublic {
		v.Add("deployment_mode", "must be one of: internal, public")
	}
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}

	user, ws, token, expiresAt, err := h.svc.Complete(r.Context(), name, email, req.Password, workspaceName, req.DeploymentMode)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	auth.SetSessionCookie(w, h.svc.cfg, token, expiresAt)
	httpx.Data(w, http.StatusCreated, completeDTO{
		User:      auth.NewUserDTO(user),
		Workspace: workspace.NewDTO(ws, authctx.RoleOwner),
	})
}

package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// Handler is the HTTP layer over Service.
type Handler struct {
	svc *Service
}

// NewHandler builds the authentication handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes registers the authentication endpoints relative to the mount point.
// GET /me expects RequireAuth to be applied by the caller.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)
	r.Get("/me", h.me)
}

// Mount registers the same endpoints with RequireAuth already applied to the
// ones that need it. Use either this or Routes, never both: chi panics on a
// duplicate pattern.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(h.svc))
		r.Get("/me", h.me)
	})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	v := validate.New()
	name, email := ValidateAccount(v, req.Name, req.Email, req.Password)
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}

	ctx := r.Context()
	user, err := h.svc.Register(ctx, name, email, req.Password, req.InvitationToken)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	token, expiresAt, err := h.svc.StartSession(ctx, user.ID, r.UserAgent(), h.clientIPHash(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.svc.SetCookie(w, token, expiresAt)
	httpx.Data(w, http.StatusCreated, NewUserDTO(user))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	// Missing credentials are rejected as bad credentials, not as a validation
	// error, so probing with an empty password reveals nothing extra.
	v := validate.New()
	email := v.Email("email", req.Email)
	if v.HasErrors() || req.Password == "" {
		httpx.Error(w, r, errInvalidCredentials)
		return
	}

	user, token, expiresAt, err := h.svc.Login(r.Context(), email, req.Password, r.UserAgent(), h.clientIPHash(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.svc.SetCookie(w, token, expiresAt)
	httpx.Data(w, http.StatusOK, NewUserDTO(user))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), h.svc.ReadCookie(r)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.svc.ClearCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.User(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	httpx.Data(w, http.StatusOK, NewUserDTO(user))
}

// clientIPHash anonymises the caller's address for the sessions row, so the
// device list can distinguish sessions without storing raw IPs.
func (h *Handler) clientIPHash(r *http.Request) string {
	return security.HashIP(h.svc.cfg.IPHashSecret, middleware.ClientIP(r))
}

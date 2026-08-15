package auth

import (
	"net/http"
	"time"

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
		r.Patch("/me", h.updateProfile)
		r.Patch("/preferences", h.updatePreferences)
		r.Put("/password", h.changePassword)
		r.Get("/2fa", h.twoFactorStatus)
		r.Post("/2fa/setup", h.twoFactorSetup)
		r.Post("/2fa/enable", h.twoFactorEnable)
		r.Delete("/2fa", h.twoFactorDisable)
	})
}

func (h *Handler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())
	var req updatePreferencesRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v := validate.New()
	if req.Language != "en" && req.Language != "id" {
		v.Add("language", "must be English or Bahasa Indonesia")
	}
	if req.Timezone == "" {
		v.Add("timezone", "is required")
	} else if _, err := time.LoadLocation(req.Timezone); err != nil {
		v.Add("timezone", "must be a valid IANA timezone")
	}
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}
	updated, err := h.svc.UpdatePreferences(r.Context(), user.ID, req.Language, req.Timezone)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, NewUserDTO(updated))
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

	user, token, expiresAt, err := h.svc.Login(r.Context(), email, req.Password, req.Code, r.UserAgent(), h.clientIPHash(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.svc.SetCookie(w, token, expiresAt)
	httpx.Data(w, http.StatusOK, NewUserDTO(user))
}

func (h *Handler) twoFactorStatus(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.svc.TwoFactorStatus(r.Context(), authctx.MustUser(r.Context()).ID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, twoFactorStatusDTO{Enabled: enabled})
}

func (h *Handler) twoFactorSetup(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())
	var req twoFactorSetupRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	setup, err := h.svc.BeginTwoFactor(r.Context(), user, req.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, setup)
}

func (h *Handler) twoFactorEnable(w http.ResponseWriter, r *http.Request) {
	var req twoFactorCodeRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.EnableTwoFactor(r.Context(), authctx.MustUser(r.Context()).ID, req.Code); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) twoFactorDisable(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())
	var req twoFactorDisableRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.DisableTwoFactor(r.Context(), user, req.Password, req.Code); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
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

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())
	var req updateProfileRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v := validate.New()
	name := v.Required("name", req.Name)
	if name != "" {
		v.Length("name", name, NameMinLength, NameMaxLength)
	}
	email := v.Email("email", req.Email)
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}
	updated, err := h.svc.UpdateProfile(r.Context(), user.ID, name, email)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, NewUserDTO(updated))
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	user := authctx.MustUser(r.Context())
	var req changePasswordRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	v := validate.New()
	if req.CurrentPassword == "" {
		v.Add("current_password", "is required")
	}
	v.Password("new_password", req.NewPassword)
	if v.HasErrors() {
		httpx.Error(w, r, httpx.Invalid(v.Fields()))
		return
	}
	if err := h.svc.ChangePassword(r.Context(), user, req.CurrentPassword, req.NewPassword, h.svc.ReadCookie(r)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// clientIPHash anonymises the caller's address for the sessions row, so the
// device list can distinguish sessions without storing raw IPs.
func (h *Handler) clientIPHash(r *http.Request) string {
	return security.HashIP(h.svc.cfg.IPHashSecret, middleware.ClientIP(r))
}

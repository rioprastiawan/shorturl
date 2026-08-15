package branding

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

const maxAssetBytes = 2 << 20

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/branding", h.get)
	r.Get("/branding/assets/{kind}", h.asset)
}

func (h *Handler) AdminRoutes(r chi.Router) {
	r.Put("/branding", h.update)
	r.Post("/branding/assets/{kind}", h.upload)
	r.Delete("/branding/assets/{kind}", h.deleteAsset)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, cfg)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var cfg Config
	if err := httpx.Decode(w, r, &cfg); err != nil {
		httpx.Error(w, r, err)
		return
	}
	updated, err := h.svc.Save(r.Context(), cfg)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, updated)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAssetBytes+(64<<10))
	if err := r.ParseMultipartForm(maxAssetBytes); err != nil {
		httpx.Error(w, r, httpx.BadRequest("File must be 2 MB or smaller"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, httpx.BadRequest("file is required"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAssetBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAssetBytes {
		httpx.Error(w, r, httpx.BadRequest("File must be between 1 byte and 2 MB"))
		return
	}
	contentType := http.DetectContentType(data)
	allowed := contentType == "image/png" || contentType == "image/jpeg" || contentType == "image/webp" || contentType == "image/x-icon" || contentType == "image/vnd.microsoft.icon"
	if !allowed {
		httpx.Error(w, r, httpx.BadRequest("Use PNG, JPEG, WebP, or ICO"))
		return
	}
	if err := h.svc.SaveAsset(r.Context(), chi.URLParam(r, "kind"), contentType, data); err != nil {
		httpx.Error(w, r, err)
		return
	}
	cfg, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, cfg)
}

func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := h.svc.DeleteAsset(r.Context(), chi.URLParam(r, "kind")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	contentType, data, err := h.svc.Asset(r.Context(), chi.URLParam(r, "kind"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !authctx.MustUser(r.Context()).IsAdmin {
		httpx.Error(w, r, httpx.ErrForbidden)
		return false
	}
	return true
}

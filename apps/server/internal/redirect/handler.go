// Package redirect serves the public short-link hot path.
//
// Everything here is written for latency: on a Redis hit a request performs
// one cache read, one bounded channel send for analytics, and writes a
// redirect header. No database call, no template rendering, no allocation
// larger than the response itself.
package redirect

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/analytics"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
	"github.com/rioprastiawan/shorturl/apps/server/internal/middleware"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/slug"
	"github.com/rioprastiawan/shorturl/apps/server/internal/urlx"
)

// Handler serves redirects for every host that is not the application domain.
type Handler struct {
	links    *link.Service
	producer *analytics.Producer
	cfg      config.Config
	logger   *slog.Logger
}

// NewHandler builds the redirect handler.
func NewHandler(links *link.Service, producer *analytics.Producer, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{links: links, producer: producer, cfg: cfg, logger: logger}
}

// ServeHTTP resolves host + slug and responds.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := urlx.NormalizeHostname(r.Host)
	path := slug.Normalize(r.URL.Path)

	if path == "" {
		// The bare custom domain has nothing to serve. Sending it to the
		// dashboard is friendlier than a 404 for someone who typed the host.
		http.Redirect(w, r, h.cfg.AppURL, http.StatusFound)
		return
	}

	// A slug is a single path segment. Anything deeper was never a short link.
	if strings.Contains(path, "/") {
		h.writeNotFound(w, r)
		return
	}

	res := h.links.Resolve(r.Context(), hostname, path)

	switch res.Status {
	case link.StatusRedirect:
		h.recordClick(r, res)
		h.writeRedirect(w, r, res)

	case link.StatusPassword:
		h.servePasswordGate(w, r, res)

	case link.StatusGone:
		h.writeGone(w, r)

	default:
		h.writeNotFound(w, r)
	}
}

func (h *Handler) writeRedirect(w http.ResponseWriter, r *http.Request, res link.Resolution) {
	code := res.Code
	if code == 0 {
		code = http.StatusFound
	}

	// A short link's target can change at any time, so intermediaries must not
	// cache the redirect even when the status code is normally cacheable.
	// Without this, a 301 would be remembered by browsers effectively forever
	// and editing the destination would appear to do nothing.
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "unsafe-url")
	http.Redirect(w, r, res.URL, code)
}

// recordClick hands the event to the analytics producer. The producer never
// blocks and never returns an error, so this cannot slow or fail a redirect.
func (h *Handler) recordClick(r *http.Request, res link.Resolution) {
	if h.producer == nil {
		return
	}

	query := r.URL.Query()
	country := strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
	if len(country) != 2 || country == "XX" {
		country = ""
	}
	h.producer.Enqueue(r.Context(), analytics.ClickEvent{
		LinkID:      res.LinkID,
		WorkspaceID: res.WorkspaceID,
		ClickedAt:   time.Now().UTC(),
		IPHash:      security.HashIP(h.cfg.IPHashSecret, middleware.ClientIP(r)),
		Country:     country,
		UserAgent:   r.UserAgent(),
		Referrer:    r.Referer(),
		UTMSource:   query.Get("utm_source"),
		UTMMedium:   query.Get("utm_medium"),
		UTMCampaign: query.Get("utm_campaign"),
	})
}

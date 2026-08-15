package redirect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
)

// unlockTTL is how long one successful password entry lasts. Short enough that
// a shared machine does not stay unlocked, long enough that a reload or a
// redirect back does not re-prompt.
const unlockTTL = 30 * time.Minute

// servePasswordGate renders the prompt on GET and checks the password on POST.
func (h *Handler) servePasswordGate(w http.ResponseWriter, r *http.Request, res link.Resolution) {
	if h.hasValidUnlock(r, res.LinkID) {
		h.recordClick(r, res)
		h.writeRedirect(w, r, res)
		return
	}

	if r.Method != http.MethodPost {
		h.renderPasswordForm(w, r, "")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderPasswordForm(w, r, "Could not read the submitted form.")
		return
	}

	ok, err := h.links.VerifyPasswordFor(r.Context(), res.LinkID, r.PostFormValue("password"))
	if err != nil {
		h.logger.Error("verify link password", "link_id", res.LinkID.String(), "error", err)
		h.renderPasswordForm(w, r, "Something went wrong. Try again.")
		return
	}
	if !ok {
		h.renderPasswordForm(w, r, "Incorrect password.")
		return
	}

	h.setUnlockCookie(w, r, res.LinkID)
	h.recordClick(r, res)
	h.writeRedirect(w, r, res)
}

func (h *Handler) renderPasswordForm(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	// 401 on a failed attempt, 200 on the first prompt: a monitoring tool
	// should be able to tell the difference.
	status := http.StatusOK
	if message != "" {
		status = http.StatusUnauthorized
	}
	w.WriteHeader(status)

	if err := passwordPage.Execute(w, struct {
		Error string
		Brand brandData
	}{Error: message, Brand: h.publicBrand(r)}); err != nil {
		h.logger.Warn("render password page", "error", err)
	}
}

// Unlock cookies are signed rather than stored: the redirect path must not
// gain a database or Redis write just to remember one browser, and the value
// carries no secret worth protecting beyond its own integrity.
func (h *Handler) unlockCookieName(linkID uuid.UUID) string {
	return "shorturl_unlock_" + linkID.String()[:8]
}

func (h *Handler) signUnlock(linkID uuid.UUID, expires int64) string {
	mac := hmac.New(sha256.New, h.cfg.SessionSecret)
	mac.Write([]byte(linkID.String()))
	mac.Write([]byte{'|'})
	mac.Write([]byte(strconv.FormatInt(expires, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *Handler) setUnlockCookie(w http.ResponseWriter, r *http.Request, linkID uuid.UUID) {
	expires := time.Now().Add(unlockTTL).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     h.unlockCookieName(linkID),
		Value:    strconv.FormatInt(expires, 10) + "." + h.signUnlock(linkID, expires),
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(expires, 0),
	})
}

func (h *Handler) hasValidUnlock(r *http.Request, linkID uuid.UUID) bool {
	cookie, err := r.Cookie(h.unlockCookieName(linkID))
	if err != nil {
		return false
	}

	rawExpires, signature, found := strings.Cut(cookie.Value, ".")
	if !found {
		return false
	}
	expires, err := strconv.ParseInt(rawExpires, 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return false
	}

	return hmac.Equal([]byte(signature), []byte(h.signUnlock(linkID, expires)))
}

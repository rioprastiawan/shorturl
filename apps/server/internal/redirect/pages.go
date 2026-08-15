package redirect

import (
	"html/template"
	"net/http"
	"strings"
)

// These pages are served on customer-facing short domains, so they are
// deliberately plain: no branding, no framework, no external requests, and no
// detail about why a link is unavailable. Distinguishing "never existed" from
// "was disabled" would let anyone enumerate a workspace's history.
var (
	statusPage   = template.Must(template.New("status").Parse(statusHTML))
	passwordPage = template.Must(template.New("password").Parse(passwordHTML))
)

const baseCSS = `
:root { color-scheme: light dark; }
* { box-sizing: border-box; }
body {
  margin: 0; min-height: 100vh; display: grid; place-items: center;
  font: 16px/1.5 system-ui, -apple-system, "Segoe UI", sans-serif;
  background: #fff; color: #0f172a; padding: 24px;
}
@media (prefers-color-scheme: dark) { body { background: #020617; color: #e2e8f0; } }
main { max-width: 24rem; width: 100%; text-align: center; }
h1 { font-size: 1.25rem; margin: 0 0 .5rem; font-weight: 600; }
p { margin: 0; color: #64748b; }
form { margin-top: 1.5rem; display: flex; flex-direction: column; gap: .75rem; }
input {
  width: 100%; padding: .625rem .75rem; font: inherit; border-radius: .5rem;
  border: 1px solid #cbd5e1; background: transparent; color: inherit;
}
@media (prefers-color-scheme: dark) { input { border-color: #334155; } }
input:focus { outline: 2px solid #2563eb; outline-offset: 1px; }
button {
  padding: .625rem .75rem; font: inherit; font-weight: 500; cursor: pointer;
  border: 0; border-radius: .5rem; background: #2563eb; color: #fff;
}
button:hover { background: #1d4ed8; }
.error { color: #dc2626; font-size: .875rem; margin-top: .25rem; }
`

const statusHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>{{.Title}}</title><style>` + baseCSS + `</style>
</head><body><main>
<h1>{{.Title}}</h1>
<p>{{.Message}}</p>
</main></body></html>`

const passwordHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>Password required</title><style>` + baseCSS + `</style>
</head><body><main>
<h1>Password required</h1>
<p>This link is protected. Enter the password to continue.</p>
<form method="post" autocomplete="off">
  <input type="password" name="password" placeholder="Password" autofocus required
         aria-label="Password"{{if .Error}} aria-invalid="true"{{end}}>
  {{if .Error}}<div class="error" role="alert">{{.Error}}</div>{{end}}
  <button type="submit">Continue</button>
</form>
</main></body></html>`

type statusData struct {
	Title   string
	Message string
}

func (h *Handler) writeNotFound(w http.ResponseWriter, r *http.Request) {
	h.writeStatus(w, r, http.StatusNotFound, statusData{
		Title:   "Link not found",
		Message: "This short link does not exist, or the address was mistyped.",
	})
}

func (h *Handler) writeGone(w http.ResponseWriter, r *http.Request) {
	h.writeStatus(w, r, http.StatusGone, statusData{
		Title:   "Link unavailable",
		Message: "This short link is no longer active.",
	})
}

func (h *Handler) writeStatus(w http.ResponseWriter, r *http.Request, code int, data statusData) {
	// A crawler or API client gets a bare status; only browsers need the page.
	if !acceptsHTML(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(data.Title + "\n"))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(code)
	if err := statusPage.Execute(w, data); err != nil {
		h.logger.Warn("render status page", "error", err)
	}
}

func acceptsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

package branding

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

const SettingKey = "system_branding"

type Config struct {
	AppName          string `json:"app_name"`
	OrganizationName string `json:"organization_name"`
	Tagline          string `json:"tagline"`
	LoginHeading     string `json:"login_heading"`
	LoginDescription string `json:"login_description"`
	FooterText       string `json:"footer_text"`
	SupportEmail     string `json:"support_email"`
	SupportURL       string `json:"support_url"`
	DocumentationURL string `json:"documentation_url"`
	PrivacyURL       string `json:"privacy_url"`
	TermsURL         string `json:"terms_url"`
	PrimaryColor     string `json:"primary_color"`
	ShellColor       string `json:"shell_color"`
	ShowPoweredBy    bool   `json:"show_powered_by"`
	LogoLightURL     string `json:"logo_light_url"`
	LogoDarkURL      string `json:"logo_dark_url"`
	LogoCompactURL   string `json:"logo_compact_url"`
	FaviconURL       string `json:"favicon_url"`
	QRForeground     string `json:"qr_foreground"`
	QRBackground     string `json:"qr_background"`
	QRStyle          string `json:"qr_style"`
	QRCornerStyle    string `json:"qr_corner_style"`
	QRMargin         int    `json:"qr_margin"`
	QRSize           int    `json:"qr_size"`
	QRUseLogo        bool   `json:"qr_use_logo"`
}

func Default() Config {
	return Config{AppName: "ShortURL", Tagline: "Simple links. Clear insights.",
		LoginHeading:     "Make every link easier to share and understand.",
		LoginDescription: "Create branded short links, understand your audience, and keep your whole team working in one place.",
		FooterText:       "Secure, self-hosted, and built for your team.", PrimaryColor: "#16a34a",
		ShellColor: "#172033", ShowPoweredBy: true, QRForeground: "#172033", QRBackground: "#ffffff",
		QRStyle: "rounded", QRCornerStyle: "rounded", QRMargin: 2, QRSize: 1024, QRUseLogo: true}
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool, q *store.Queries) *Service { return &Service{pool: pool, q: q} }

func (s *Service) Get(ctx context.Context) (Config, error) {
	cfg := Default()
	row, err := s.q.GetSetting(ctx, SettingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.withAssets(ctx, cfg), nil
	}
	if err != nil {
		return cfg, httpx.Internal(err)
	}
	if err := json.Unmarshal(row.Value, &cfg); err != nil {
		return cfg, httpx.Internal(err)
	}
	return s.withAssets(ctx, cfg), nil
}

func (s *Service) withAssets(ctx context.Context, cfg Config) Config {
	rows, err := s.pool.Query(ctx, `SELECT kind, extract(epoch from updated_at)::bigint FROM system_branding_assets`)
	if err != nil {
		return cfg
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var version int64
		if rows.Scan(&kind, &version) != nil {
			continue
		}
		url := "/api/v1/system/branding/assets/" + kind + "?v=" + strconv.FormatInt(version, 10)
		switch kind {
		case "logo_light":
			cfg.LogoLightURL = url
		case "logo_dark":
			cfg.LogoDarkURL = url
		case "logo_compact":
			cfg.LogoCompactURL = url
		case "favicon":
			cfg.FaviconURL = url
		}
	}
	return cfg
}

func (s *Service) Save(ctx context.Context, cfg Config) (Config, error) {
	cfg.AppName = strings.TrimSpace(cfg.AppName)
	if cfg.AppName == "" || len(cfg.AppName) > 80 {
		return cfg, httpx.Invalid(map[string][]string{"app_name": {"must be between 1 and 80 characters"}})
	}
	for field, value := range map[string]string{"organization_name": cfg.OrganizationName, "tagline": cfg.Tagline, "login_heading": cfg.LoginHeading, "login_description": cfg.LoginDescription, "footer_text": cfg.FooterText, "support_email": cfg.SupportEmail, "support_url": cfg.SupportURL, "documentation_url": cfg.DocumentationURL, "privacy_url": cfg.PrivacyURL, "terms_url": cfg.TermsURL} {
		if len(value) > 500 {
			return cfg, httpx.Invalid(map[string][]string{field: {"must be at most 500 characters"}})
		}
	}
	if !validHex(cfg.PrimaryColor) || !validHex(cfg.ShellColor) {
		return cfg, httpx.Invalid(map[string][]string{"colors": {"must use six-digit hex colors"}})
	}
	if !validHex(cfg.QRForeground) || !validHex(cfg.QRBackground) {
		return cfg, httpx.Invalid(map[string][]string{"qr_colors": {"must use six-digit hex colors"}})
	}
	validQRStyles := map[string]bool{"square": true, "rounded": true, "dots": true, "extra-rounded": true, "diamond": true, "classy": true, "classy-rounded": true, "soft": true, "star": true}
	if !validQRStyles[cfg.QRStyle] {
		return cfg, httpx.Invalid(map[string][]string{"qr_style": {"is not supported"}})
	}
	validCornerStyles := map[string]bool{"square": true, "rounded": true, "circle": true, "dot": true, "leaf": true}
	if !validCornerStyles[cfg.QRCornerStyle] {
		return cfg, httpx.Invalid(map[string][]string{"qr_corner_style": {"is not supported"}})
	}
	if cfg.QRMargin < 1 || cfg.QRMargin > 6 {
		return cfg, httpx.Invalid(map[string][]string{"qr_margin": {"must be between 1 and 6"}})
	}
	if cfg.QRSize != 512 && cfg.QRSize != 1024 && cfg.QRSize != 2048 {
		return cfg, httpx.Invalid(map[string][]string{"qr_size": {"must be 512, 1024, or 2048"}})
	}
	// Asset URLs are derived server-side and never persisted from client input.
	cfg.LogoLightURL, cfg.LogoDarkURL, cfg.LogoCompactURL, cfg.FaviconURL = "", "", "", ""
	raw, _ := json.Marshal(cfg)
	if _, err := s.q.SetSetting(ctx, store.SetSettingParams{Key: SettingKey, Value: raw}); err != nil {
		return cfg, httpx.Internal(err)
	}
	return s.Get(ctx)
}

func validHex(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, c := range value[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func (s *Service) SaveAsset(ctx context.Context, kind, contentType string, data []byte) error {
	if !validKind(kind) {
		return httpx.ErrNotFound
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO system_branding_assets (kind, content_type, data) VALUES ($1,$2,$3)
		ON CONFLICT (kind) DO UPDATE SET content_type=EXCLUDED.content_type, data=EXCLUDED.data, updated_at=now()`, kind, contentType, data); err != nil {
		return httpx.Internal(err)
	}
	return nil
}

func (s *Service) Asset(ctx context.Context, kind string) (string, []byte, error) {
	if !validKind(kind) {
		return "", nil, httpx.ErrNotFound
	}
	var contentType string
	var data []byte
	err := s.pool.QueryRow(ctx, `SELECT content_type, data FROM system_branding_assets WHERE kind=$1`, kind).Scan(&contentType, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, httpx.ErrNotFound
	}
	if err != nil {
		return "", nil, httpx.Internal(err)
	}
	return contentType, data, nil
}

func (s *Service) DeleteAsset(ctx context.Context, kind string) error {
	if !validKind(kind) {
		return httpx.ErrNotFound
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM system_branding_assets WHERE kind=$1`, kind)
	if err != nil {
		return httpx.Internal(err)
	}
	return nil
}

func validKind(kind string) bool {
	return kind == "logo_light" || kind == "logo_dark" || kind == "logo_compact" || kind == "favicon"
}

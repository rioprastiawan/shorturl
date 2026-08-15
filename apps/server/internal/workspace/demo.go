package workspace

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/authctx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// DemoSize controls how much synthetic data is created. Large presets are
// intentionally admin-only at the HTTP boundary because they are load-testing
// tools as much as onboarding examples.
type DemoSize string

const (
	DemoStarter  DemoSize = "starter"
	DemoBusy     DemoSize = "busy"
	DemoFiveYear DemoSize = "five_year"
)

type demoPreset struct {
	links          int
	days           int
	analyticsLinks int
}

func demoPresetFor(size DemoSize) (demoPreset, bool) {
	switch size {
	case DemoStarter:
		return demoPreset{links: 15, days: 90, analyticsLinks: 8}, true
	case DemoBusy:
		return demoPreset{links: 500, days: 365, analyticsLinks: 25}, true
	case DemoFiveYear:
		return demoPreset{links: 2500, days: 5 * 365, analyticsLinks: 30}, true
	default:
		return demoPreset{}, false
	}
}

// CreateDemo creates a complete, isolated example workspace in one
// transaction. Rollups are generated directly because replaying synthetic raw
// events through Redis would be slow and would make creation non-atomic.
func (s *Service) CreateDemo(ctx context.Context, userID uuid.UUID, size DemoSize) (*store.Workspace, error) {
	preset, ok := demoPresetFor(size)
	if !ok {
		return nil, httpx.BadRequest("size must be one of starter, busy, five_year")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, httpx.Internal(err)
	}
	defer tx.Rollback(ctx)
	qtx := store.New(tx)

	slug, err := UniqueSlug(ctx, qtx, "Demo Workspace")
	if err != nil {
		return nil, err
	}
	ws, err := qtx.CreateWorkspace(ctx, store.CreateWorkspaceParams{Name: "Demo Workspace", Slug: slug, OwnerUserID: userID})
	if err != nil {
		return nil, httpx.Internal(err)
	}
	if _, err := qtx.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: userID, Role: string(authctx.RoleOwner)}); err != nil {
		return nil, httpx.Internal(err)
	}

	domainID := uuid.New()
	hostname := fmt.Sprintf("demo-%s.example.test", ws.ID.String()[:12])
	if _, err := tx.Exec(ctx, `INSERT INTO domains
		(id, workspace_id, hostname, status, verification_token, verification_method,
		 ssl_status, is_default, verified_at, last_checked_at)
		VALUES ($1, $2, $3, 'active', $4, 'dns_txt', 'active', true, now(), now())`,
		domainID, ws.ID, hostname, uuid.NewString()); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO links
		(workspace_id, domain_id, slug, destination_url, title, status, redirect_type,
		 click_count, metadata, created_by, created_via, created_at, updated_at)
		SELECT $1, $2,
			CASE WHEN i <= 6 THEN (ARRAY['welcome','product-launch','pricing','documentation','summer-campaign','customer-story'])[i]
			     ELSE 'demo-link-' || lpad(i::text, 4, '0') END,
			CASE i % 5 WHEN 0 THEN 'https://example.com/pricing'
			     WHEN 1 THEN 'https://example.com/products'
			     WHEN 2 THEN 'https://example.com/docs/getting-started'
			     WHEN 3 THEN 'https://example.com/blog/launch'
			     ELSE 'https://example.com/contact' END,
			CASE i % 5 WHEN 0 THEN 'Pricing page' WHEN 1 THEN 'Product launch'
			     WHEN 2 THEN 'Getting started guide' WHEN 3 THEN 'Launch announcement'
			     ELSE 'Contact the team' END || CASE WHEN i > 6 THEN ' #' || i ELSE '' END,
			CASE WHEN i % 29 = 0 THEN 'archived' WHEN i % 17 = 0 THEN 'disabled' ELSE 'active' END,
			CASE WHEN i % 11 = 0 THEN 301 ELSE 302 END, 0,
			jsonb_build_object('demo', true, 'campaign', CASE i % 3 WHEN 0 THEN 'organic' WHEN 1 THEN 'email' ELSE 'social' END),
			$3, CASE WHEN i % 7 = 0 THEN 'api' ELSE 'dashboard' END,
			now() - make_interval(days => (i % $5)), now() - make_interval(days => (i % $5))
		FROM generate_series(1, $4::integer) AS i`, ws.ID, domainID, userID, preset.links, preset.days); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `WITH demo_links AS (
		SELECT id FROM links WHERE workspace_id = $1 ORDER BY created_at, id LIMIT $2
	), buckets AS (
		SELECT generate_series(date_trunc('day', now()) - make_interval(days => $3 - 1),
		                       date_trunc('day', now()), interval '1 day') + interval '12 hours' AS bucket
	)
	INSERT INTO click_hourly (workspace_id, link_id, bucket, clicks)
	SELECT $1, link.id, bucket,
		(3 + mod(abs(hashtext(link.id::text || bucket::text)::bigint), 120))::bigint
	FROM demo_links link CROSS JOIN buckets`, ws.ID, preset.analyticsLinks, preset.days); err != nil {
		return nil, httpx.Internal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO click_hourly (workspace_id, link_id, bucket, clicks)
		SELECT $1, id, date_trunc('day', created_at) + interval '13 hours',
			(10 + mod(abs(hashtext(id::text)::bigint), 490))::bigint
		FROM links WHERE workspace_id = $1
		ON CONFLICT (link_id, bucket) DO NOTHING`, ws.ID); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `WITH days AS (
		SELECT generate_series(current_date - ($2::integer - 1), current_date, interval '1 day')::date AS day
	), values(dimension, value, weight) AS (VALUES
		('device','Desktop',45), ('device','Mobile',38), ('device','Tablet',7),
		('browser','Chrome',48), ('browser','Safari',27), ('browser','Firefox',13), ('browser','Edge',8),
		('os','Windows',36), ('os','macOS',25), ('os','Android',22), ('os','iOS',15),
		('referrer','google.com',42), ('referrer','linkedin.com',18), ('referrer','newsletter',15), ('referrer','direct',25),
		('utm_source','google',35), ('utm_source','newsletter',28), ('utm_source','linkedin',20), ('utm_source','partner',12)
		,('utm_medium','cpc',34), ('utm_medium','email',29), ('utm_medium','social',22), ('utm_medium','referral',11)
		,('utm_campaign','product-launch',37), ('utm_campaign','summer-sale',26), ('utm_campaign','weekly-digest',21)
		,('country','ID',41), ('country','US',24), ('country','SG',16), ('country','AU',9)
	)
	INSERT INTO click_dimension_daily (workspace_id, day, dimension, value, clicks)
	SELECT $1, day, dimension, value,
		greatest(1, weight + mod(abs(hashtext(day::text || dimension || value)::bigint), 25) - 12)
	FROM days CROSS JOIN values`, ws.ID, preset.days); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO click_dimension_daily (workspace_id, day, dimension, value, clicks)
		SELECT $1, day::date, 'visitor', 'demo-visitor-' || visitor, 1
		FROM generate_series(current_date - (least($2::integer, 365) - 1), current_date, interval '1 day') day
		CROSS JOIN generate_series(1, 20) visitor`, ws.ID, preset.days); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `UPDATE links l SET click_count = totals.clicks
		FROM (SELECT link_id, sum(clicks)::bigint clicks FROM click_hourly WHERE workspace_id = $1 GROUP BY link_id) totals
		WHERE l.id = totals.link_id`, ws.ID); err != nil {
		return nil, httpx.Internal(err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO click_events
		(link_id, workspace_id, clicked_at, country, city, device, os, browser, referrer_host, utm_source, utm_medium, utm_campaign)
		SELECT link.id, $1, now() - make_interval(hours => event_no * 2),
			CASE event_no % 4 WHEN 0 THEN 'ID' WHEN 1 THEN 'US' WHEN 2 THEN 'SG' ELSE 'AU' END,
			CASE event_no % 4 WHEN 0 THEN 'Jakarta' WHEN 1 THEN 'New York' WHEN 2 THEN 'Singapore' ELSE 'Sydney' END,
			CASE event_no % 3 WHEN 0 THEN 'desktop' WHEN 1 THEN 'mobile' ELSE 'tablet' END,
			CASE event_no % 3 WHEN 0 THEN 'Windows' WHEN 1 THEN 'Android' ELSE 'iOS' END,
			CASE event_no % 3 WHEN 0 THEN 'Chrome' WHEN 1 THEN 'Safari' ELSE 'Firefox' END,
			CASE event_no % 3 WHEN 0 THEN 'google.com' WHEN 1 THEN 'linkedin.com' ELSE NULL END,
			CASE event_no % 3 WHEN 0 THEN 'google' WHEN 1 THEN 'linkedin' ELSE 'newsletter' END,
			'campaign', 'demo-launch'
		FROM (SELECT id FROM links WHERE workspace_id = $1 ORDER BY click_count DESC LIMIT 10) link
		CROSS JOIN generate_series(1, 5) event_no`, ws.ID); err != nil {
		return nil, httpx.Internal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httpx.Internal(err)
	}
	return &ws, nil
}

// DemoEstimate is returned to the UI through static preset descriptions and is
// kept here to make performance expectations explicit in tests.
func DemoEstimate(size DemoSize) (links, days int, ok bool) {
	preset, ok := demoPresetFor(size)
	return preset.links, preset.days, ok
}

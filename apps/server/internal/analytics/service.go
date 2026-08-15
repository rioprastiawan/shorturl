package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// Granularities accepted by date_trunc in the rollup queries. Nothing else may
// ever reach the query: the value is interpolated by PostgreSQL as an
// identifier-like string, so it is chosen here and never taken from input.
const (
	GranularityHour = "hour"
	GranularityDay  = "day"
	GranularityWeek = "week"
)

// dimensionLimit caps every breakdown. Ten rows is what the dashboard renders;
// a longer tail is noise.
const dimensionLimit = 10

// topLinkLimit caps the top-links table.
const topLinkLimit = 10

// recentLinkLimit matches the overview card.
const recentLinkLimit = 5

// maxCustomRange bounds a custom window. Rollups make a wide range cheap, but
// not unbounded, and a year of daily buckets is already more than any chart
// can render.
const maxCustomRange = 366 * 24 * time.Hour

// Range is a resolved reporting window plus the bucket size to report it at.
type Range struct {
	Label       string
	From        time.Time
	To          time.Time
	Granularity string
}

// Days returns the window as whole UTC days, which is how
// click_dimension_daily is keyed.
func (r Range) Days() (from, to time.Time) {
	return truncateDay(r.From), truncateDay(r.To)
}

func truncateDay(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseRange resolves the range query parameters against a reference time.
//
// Granularity follows from the window rather than being a separate knob: an
// hourly chart over 90 days is 2,160 unreadable points, and a weekly chart
// over one day is a single bar.
func ParseRange(label, from, to string, now time.Time) (Range, error) {
	now = now.UTC()
	if label == "" {
		label = "7d"
	}

	var start, end time.Time
	switch label {
	case "24h":
		start, end = now.Add(-24*time.Hour), now
	case "7d":
		start, end = now.AddDate(0, 0, -7), now
	case "30d":
		start, end = now.AddDate(0, 0, -30), now
	case "90d":
		start, end = now.AddDate(0, 0, -90), now
	case "custom":
		var err error
		if start, err = parseRFC3339("from", from); err != nil {
			return Range{}, err
		}
		if end, err = parseRFC3339("to", to); err != nil {
			return Range{}, err
		}
		if !start.Before(end) {
			return Range{}, httpx.BadRequest("from must be earlier than to")
		}
		if end.Sub(start) > maxCustomRange {
			return Range{}, httpx.BadRequest("A custom range may span at most 366 days")
		}
	default:
		return Range{}, httpx.BadRequest("range must be one of 24h, 7d, 30d, 90d, custom")
	}

	return Range{
		Label:       label,
		From:        start.UTC(),
		To:          end.UTC(),
		Granularity: granularityFor(end.Sub(start)),
	}, nil
}

func granularityFor(span time.Duration) string {
	switch {
	case span <= 24*time.Hour:
		return GranularityHour
	case span <= 30*24*time.Hour:
		return GranularityDay
	default:
		return GranularityWeek
	}
}

func parseRFC3339(field, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, httpx.BadRequest(field + " is required when range=custom")
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, httpx.BadRequest(field + " must be an RFC3339 timestamp")
	}
	return t.UTC(), nil
}

// Service answers reporting questions. Every query it issues reads a rollup
// table; none of them touch click_events.
type Service struct {
	q   *store.Queries
	cfg config.Config
}

// NewService wires the reporting side of the analytics feature.
func NewService(q *store.Queries, cfg config.Config) *Service {
	return &Service{q: q, cfg: cfg}
}

// Overview returns the workspace header counters and the most recent links.
func (s *Service) Overview(ctx context.Context, workspaceID uuid.UUID) (OverviewResponse, error) {
	counts, err := s.q.CountLinksInWorkspace(ctx, workspaceID)
	if err != nil {
		return OverviewResponse{}, httpx.Internal(err)
	}

	totals, err := s.q.WorkspaceClickTotals(ctx, workspaceID)
	if err != nil {
		return OverviewResponse{}, httpx.Internal(err)
	}

	domains, err := s.q.CountActiveDomains(ctx, workspaceID)
	if err != nil {
		return OverviewResponse{}, httpx.Internal(err)
	}

	recent, err := s.q.RecentLinks(ctx, store.RecentLinksParams{
		WorkspaceID: workspaceID,
		Limit:       recentLinkLimit,
	})
	if err != nil {
		return OverviewResponse{}, httpx.Internal(err)
	}

	links := make([]RecentLink, 0, len(recent))
	for _, row := range recent {
		links = append(links, RecentLink{
			ID:             row.Link.ID.String(),
			Slug:           row.Link.Slug,
			Title:          row.Link.Title,
			ShortURL:       shortURL(row.Hostname, row.Link.Slug),
			DestinationURL: row.Link.DestinationUrl,
			Status:         row.Link.Status,
			Clicks:         row.Link.ClickCount,
			CreatedAt:      row.Link.CreatedAt,
		})
	}

	return OverviewResponse{
		TotalLinks:    counts.Total,
		ActiveLinks:   counts.Active,
		TotalClicks:   totals.TotalClicks,
		ClicksToday:   totals.ClicksToday,
		ActiveDomains: domains,
		RecentLinks:   links,
	}, nil
}

// Clicks returns the time series plus every breakdown the analytics page shows.
func (s *Service) Clicks(ctx context.Context, workspaceID uuid.UUID, rng Range) (ClicksResponse, error) {
	series, err := s.q.ClicksOverTime(ctx, store.ClicksOverTimeParams{
		Granularity: rng.Granularity,
		WorkspaceID: workspaceID,
		FromTime:    rng.From,
		ToTime:      rng.To,
	})
	if err != nil {
		return ClicksResponse{}, httpx.Internal(err)
	}

	points := make([]SeriesPoint, 0, len(series))
	for _, row := range series {
		points = append(points, SeriesPoint{Period: row.Period, Clicks: row.Clicks})
	}

	top, err := s.q.TopLinks(ctx, store.TopLinksParams{
		WorkspaceID: workspaceID,
		FromTime:    rng.From,
		ToTime:      rng.To,
		RowLimit:    topLinkLimit,
	})
	if err != nil {
		return ClicksResponse{}, httpx.Internal(err)
	}

	links := make([]TopLink, 0, len(top))
	for _, row := range top {
		links = append(links, TopLink{
			ID:       row.ID.String(),
			Slug:     row.Slug,
			Title:    row.Title,
			ShortURL: shortURL(row.Hostname, row.Slug),
			Clicks:   row.Clicks,
		})
	}

	out := ClicksResponse{
		Range:       rng.Label,
		Granularity: rng.Granularity,
		From:        rng.From,
		To:          rng.To,
		Series:      points,
		TopLinks:    links,
	}

	for _, breakdown := range []struct {
		dimension string
		target    *[]DimensionCount
	}{
		{DimensionReferrer, &out.Referrers},
		{DimensionUTMSource, &out.UTMSources},
		{DimensionDevice, &out.Devices},
		{DimensionBrowser, &out.Browsers},
		{DimensionOS, &out.OS},
	} {
		values, err := s.dimension(ctx, workspaceID, breakdown.dimension, rng)
		if err != nil {
			return ClicksResponse{}, err
		}
		*breakdown.target = values
	}

	return out, nil
}

// LinkAnalytics returns the time series for one link. The workspace ID comes
// from the caller's membership, so a link belonging to another tenant simply
// does not resolve.
func (s *Service) LinkAnalytics(ctx context.Context, workspaceID, linkID uuid.UUID, rng Range) (LinkAnalyticsResponse, error) {
	link, err := s.q.GetLinkInWorkspace(ctx, store.GetLinkInWorkspaceParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LinkAnalyticsResponse{}, httpx.ErrNotFound
		}
		return LinkAnalyticsResponse{}, httpx.Internal(err)
	}

	totals, err := s.q.LinkClickTotals(ctx, linkID)
	if err != nil {
		return LinkAnalyticsResponse{}, httpx.Internal(err)
	}

	series, err := s.q.ClicksOverTimeForLink(ctx, store.ClicksOverTimeForLinkParams{
		Granularity: rng.Granularity,
		LinkID:      linkID,
		FromTime:    rng.From,
		ToTime:      rng.To,
	})
	if err != nil {
		return LinkAnalyticsResponse{}, httpx.Internal(err)
	}

	points := make([]SeriesPoint, 0, len(series))
	for _, row := range series {
		points = append(points, SeriesPoint{Period: row.Period, Clicks: row.Clicks})
	}

	return LinkAnalyticsResponse{
		Range:       rng.Label,
		Granularity: rng.Granularity,
		From:        rng.From,
		To:          rng.To,
		Link: TopLink{
			ID:       link.Link.ID.String(),
			Slug:     link.Link.Slug,
			Title:    link.Link.Title,
			ShortURL: shortURL(link.Hostname, link.Link.Slug),
			Clicks:   totals.TotalClicks,
		},
		TotalClicks: totals.TotalClicks,
		ClicksToday: totals.ClicksToday,
		Series:      points,
	}, nil
}

func (s *Service) dimension(ctx context.Context, workspaceID uuid.UUID, dimension string, rng Range) ([]DimensionCount, error) {
	fromDay, toDay := rng.Days()
	rows, err := s.q.TopDimensionValues(ctx, store.TopDimensionValuesParams{
		WorkspaceID: workspaceID,
		Dimension:   dimension,
		FromDay:     fromDay,
		ToDay:       toDay,
		RowLimit:    dimensionLimit,
	})
	if err != nil {
		return nil, httpx.Internal(err)
	}

	out := make([]DimensionCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, DimensionCount{Value: row.Value, Clicks: row.Clicks})
	}
	return out, nil
}

// shortURL renders the public form of a link. Custom domains always terminate
// TLS at Traefik, so the scheme is not configurable.
func shortURL(hostname, slug string) string {
	return "https://" + hostname + "/" + slug
}

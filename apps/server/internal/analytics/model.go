package analytics

import "time"

// OverviewResponse backs the dashboard header (§16 of the plan).
type OverviewResponse struct {
	TotalLinks    int64        `json:"total_links"`
	ActiveLinks   int64        `json:"active_links"`
	TotalClicks   int64        `json:"total_clicks"`
	ClicksToday   int64        `json:"clicks_today"`
	ActiveDomains int64        `json:"active_domains"`
	RecentLinks   []RecentLink `json:"recent_links"`
}

// RecentLink is the compact link shape the overview embeds. The links feature
// owns the full representation; this one exists so the overview is a single
// request.
type RecentLink struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Title          *string   `json:"title"`
	ShortURL       string    `json:"short_url"`
	DestinationURL string    `json:"destination_url"`
	Status         string    `json:"status"`
	Clicks         int64     `json:"clicks"`
	CreatedAt      time.Time `json:"created_at"`
}

// SeriesPoint is one bucket of the clicks-over-time chart.
type SeriesPoint struct {
	Period time.Time `json:"period"`
	Clicks int64     `json:"clicks"`
}

// TopLink is one row of the top-links table.
type TopLink struct {
	ID       string  `json:"id"`
	Slug     string  `json:"slug"`
	Title    *string `json:"title"`
	ShortURL string  `json:"short_url"`
	Clicks   int64   `json:"clicks"`
}

// DimensionCount is one row of any breakdown (referrer, device, browser, ...).
type DimensionCount struct {
	Value  string `json:"value"`
	Clicks int64  `json:"clicks"`
}

// ClicksResponse is the full analytics page payload.
type ClicksResponse struct {
	Range       string           `json:"range"`
	Granularity string           `json:"granularity"`
	From        time.Time        `json:"from"`
	To          time.Time        `json:"to"`
	Series      []SeriesPoint    `json:"series"`
	TopLinks    []TopLink        `json:"top_links"`
	Referrers   []DimensionCount `json:"referrers"`
	UTMSources  []DimensionCount `json:"utm_sources"`
	Devices     []DimensionCount `json:"devices"`
	Browsers    []DimensionCount `json:"browsers"`
	OS          []DimensionCount `json:"os"`
}

// LinkAnalyticsResponse is the per-link view.
//
// It carries no dimension breakdowns: click_dimension_daily is keyed by
// workspace, not by link, so a per-link referrer chart would have to scan
// click_events — exactly the query this design exists to avoid.
type LinkAnalyticsResponse struct {
	Range       string        `json:"range"`
	Granularity string        `json:"granularity"`
	From        time.Time     `json:"from"`
	To          time.Time     `json:"to"`
	Link        TopLink       `json:"link"`
	TotalClicks int64         `json:"total_clicks"`
	ClicksToday int64         `json:"clicks_today"`
	Series      []SeriesPoint `json:"series"`
}

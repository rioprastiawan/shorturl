package analytics

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
)

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 8, 15, 13, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		label       string
		from        string
		to          string
		wantLabel   string
		wantFrom    time.Time
		wantTo      time.Time
		granularity string
	}{
		{
			name:        "empty defaults to 7d",
			wantLabel:   "7d",
			wantFrom:    now.AddDate(0, 0, -7),
			wantTo:      now,
			granularity: GranularityDay,
		},
		{
			name:        "24h buckets by hour",
			label:       "24h",
			wantLabel:   "24h",
			wantFrom:    now.Add(-24 * time.Hour),
			wantTo:      now,
			granularity: GranularityHour,
		},
		{
			name:        "30d still buckets by day",
			label:       "30d",
			wantLabel:   "30d",
			wantFrom:    now.AddDate(0, 0, -30),
			wantTo:      now,
			granularity: GranularityDay,
		},
		{
			name:        "90d buckets by week",
			label:       "90d",
			wantLabel:   "90d",
			wantFrom:    now.AddDate(0, 0, -90),
			wantTo:      now,
			granularity: GranularityWeek,
		},
		{
			name:        "1y buckets by week",
			label:       "1y",
			wantLabel:   "1y",
			wantFrom:    now.AddDate(-1, 0, 0),
			wantTo:      now,
			granularity: GranularityWeek,
		},
		{
			name:        "5y buckets by month",
			label:       "5y",
			wantLabel:   "5y",
			wantFrom:    now.AddDate(-5, 0, 0),
			wantTo:      now,
			granularity: GranularityMonth,
		},
		{
			name:        "custom under a day buckets by hour",
			label:       "custom",
			from:        "2026-08-15T00:00:00Z",
			to:          "2026-08-15T18:00:00Z",
			wantLabel:   "custom",
			wantFrom:    time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			wantTo:      time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC),
			granularity: GranularityHour,
		},
		{
			name:        "custom over a month buckets by week",
			label:       "custom",
			from:        "2026-01-01T00:00:00Z",
			to:          "2026-06-01T00:00:00Z",
			wantLabel:   "custom",
			wantFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantTo:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			granularity: GranularityWeek,
		},
		{
			name:        "custom with an offset is normalised to utc",
			label:       "custom",
			from:        "2026-08-15T07:00:00+07:00",
			to:          "2026-08-15T17:00:00+07:00",
			wantLabel:   "custom",
			wantFrom:    time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
			wantTo:      time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC),
			granularity: GranularityHour,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRange(tc.label, tc.from, tc.to, now)
			if err != nil {
				t.Fatalf("ParseRange: %v", err)
			}
			if got.Label != tc.wantLabel {
				t.Errorf("label: got %q want %q", got.Label, tc.wantLabel)
			}
			if !got.From.Equal(tc.wantFrom) {
				t.Errorf("from: got %s want %s", got.From, tc.wantFrom)
			}
			if !got.To.Equal(tc.wantTo) {
				t.Errorf("to: got %s want %s", got.To, tc.wantTo)
			}
			if got.Granularity != tc.granularity {
				t.Errorf("granularity: got %q want %q", got.Granularity, tc.granularity)
			}
		})
	}
}

func TestParseRangeRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 8, 15, 13, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		label string
		from  string
		to    string
	}{
		{"unknown label", "10y", "", ""},
		{"a number is not a label", "7", "", ""},
		{"days are case sensitive", "7D", "", ""},
		{"custom without bounds", "custom", "", ""},
		{"custom without to", "custom", "2026-08-01T00:00:00Z", ""},
		{"custom without from", "custom", "", "2026-08-01T00:00:00Z"},
		{"custom with unparseable from", "custom", "2026-08-01", "2026-08-02T00:00:00Z"},
		{"custom with unparseable to", "custom", "2026-08-01T00:00:00Z", "tomorrow"},
		{"custom reversed", "custom", "2026-08-02T00:00:00Z", "2026-08-01T00:00:00Z"},
		{"custom empty window", "custom", "2026-08-02T00:00:00Z", "2026-08-02T00:00:00Z"},
		{"custom wider than a year", "custom", "2020-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRange(tc.label, tc.from, tc.to, now)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			var apiErr *httpx.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected an *httpx.APIError, got %T", err)
			}
			if apiErr.Status != http.StatusBadRequest {
				t.Errorf("status: got %d want %d", apiErr.Status, http.StatusBadRequest)
			}
		})
	}
}

func TestParseRangeGranularityBoundaries(t *testing.T) {
	tests := []struct {
		name string
		span time.Duration
		want string
	}{
		{"one hour", time.Hour, GranularityHour},
		{"exactly 24 hours", 24 * time.Hour, GranularityHour},
		{"just over 24 hours", 24*time.Hour + time.Second, GranularityDay},
		{"exactly 30 days", 30 * 24 * time.Hour, GranularityDay},
		{"just over 30 days", 30*24*time.Hour + time.Second, GranularityWeek},
		{"90 days", 90 * 24 * time.Hour, GranularityWeek},
		{"one year", 365 * 24 * time.Hour, GranularityWeek},
		{"five years", 5 * 365 * 24 * time.Hour, GranularityMonth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := granularityFor(tc.span); got != tc.want {
				t.Errorf("granularityFor(%s) = %q, want %q", tc.span, got, tc.want)
			}
		})
	}
}

func TestRangeDaysTruncatesToUTCMidnight(t *testing.T) {
	rng := Range{
		From: time.Date(2026, 8, 15, 23, 59, 59, 0, time.UTC),
		To:   time.Date(2026, 8, 18, 0, 0, 1, 0, time.UTC),
	}
	from, to := rng.Days()

	if want := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC); !from.Equal(want) {
		t.Errorf("from: got %s want %s", from, want)
	}
	// The upper bound is inclusive in TopDimensionValues, so the final partial
	// day must still be counted.
	if want := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC); !to.Equal(want) {
		t.Errorf("to: got %s want %s", to, want)
	}
}

func TestShortURL(t *testing.T) {
	if got := shortURL("go.example.com", "launch"); got != "https://go.example.com/launch" {
		t.Errorf("got %q", got)
	}
}

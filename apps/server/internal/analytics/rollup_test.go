package analytics

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAggregateCollapsesABatch(t *testing.T) {
	workspace := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	linkA := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	linkB := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	hourOne := time.Date(2026, 8, 15, 10, 5, 0, 0, time.UTC)
	hourTwo := time.Date(2026, 8, 15, 11, 59, 59, 0, time.UTC)
	nextDay := time.Date(2026, 8, 16, 0, 0, 1, 0, time.UTC)

	chromeWindows := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	events := []enriched{
		enrich(ClickEvent{LinkID: linkA, WorkspaceID: workspace, ClickedAt: hourOne, UserAgent: chromeWindows, Referrer: "https://news.example.com/a?x=1", UTMSource: "newsletter"}),
		enrich(ClickEvent{LinkID: linkA, WorkspaceID: workspace, ClickedAt: hourOne.Add(20 * time.Minute), UserAgent: chromeWindows, Referrer: "https://news.example.com/b"}),
		enrich(ClickEvent{LinkID: linkA, WorkspaceID: workspace, ClickedAt: hourTwo, UserAgent: chromeWindows}),
		enrich(ClickEvent{LinkID: linkB, WorkspaceID: workspace, ClickedAt: hourOne, UserAgent: chromeWindows}),
		enrich(ClickEvent{LinkID: linkA, WorkspaceID: workspace, ClickedAt: nextDay, UserAgent: chromeWindows}),
	}

	got := aggregate(events)

	t.Run("hourly buckets", func(t *testing.T) {
		want := map[hourlyKey]int64{
			{workspace, linkA, time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)}: 2,
			{workspace, linkA, time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)}: 1,
			{workspace, linkA, time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)}:  1,
			{workspace, linkB, time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)}: 1,
		}
		if len(got.Hourly) != len(want) {
			t.Fatalf("got %d hourly buckets, want %d: %+v", len(got.Hourly), len(want), got.Hourly)
		}
		for _, bucket := range got.Hourly {
			key := hourlyKey{bucket.WorkspaceID, bucket.LinkID, bucket.Bucket}
			clicks, ok := want[key]
			if !ok {
				t.Errorf("unexpected bucket %+v", bucket)
				continue
			}
			if bucket.Clicks != clicks {
				t.Errorf("bucket %+v: got %d clicks want %d", key, bucket.Clicks, clicks)
			}
		}
	})

	t.Run("link totals", func(t *testing.T) {
		want := map[uuid.UUID]int64{linkA: 4, linkB: 1}
		if len(got.Links) != len(want) {
			t.Fatalf("got %d link counters, want %d", len(got.Links), len(want))
		}
		for _, link := range got.Links {
			if link.Clicks != want[link.LinkID] {
				t.Errorf("link %s: got %d clicks want %d", link.LinkID, link.Clicks, want[link.LinkID])
			}
		}
	})

	t.Run("dimensions", func(t *testing.T) {
		day15 := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
		day16 := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
		want := map[dimensionKey]int64{
			{workspace, day15, DimensionDevice, "desktop"}:            4,
			{workspace, day15, DimensionOS, "Windows"}:                4,
			{workspace, day15, DimensionBrowser, "Chrome"}:            4,
			{workspace, day15, DimensionReferrer, "news.example.com"}: 2,
			{workspace, day15, DimensionUTMSource, "newsletter"}:      1,
			{workspace, day16, DimensionDevice, "desktop"}:            1,
			{workspace, day16, DimensionOS, "Windows"}:                1,
			{workspace, day16, DimensionBrowser, "Chrome"}:            1,
		}
		if len(got.Dimensions) != len(want) {
			t.Fatalf("got %d dimension rows, want %d: %+v", len(got.Dimensions), len(want), got.Dimensions)
		}
		for _, row := range got.Dimensions {
			key := dimensionKey{row.WorkspaceID, row.Day, row.Dimension, row.Value}
			clicks, ok := want[key]
			if !ok {
				t.Errorf("unexpected dimension row %+v", row)
				continue
			}
			if row.Clicks != clicks {
				t.Errorf("dimension %+v: got %d clicks want %d", key, row.Clicks, clicks)
			}
		}
	})
}

func TestAggregateSkipsEmptyDimensionValues(t *testing.T) {
	ev := enrich(ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC),
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36",
		// no referrer, no UTM parameters, no country
	})

	for _, row := range aggregate([]enriched{ev}).Dimensions {
		if row.Value == "" {
			t.Errorf("dimension %q stored an empty value", row.Dimension)
		}
		switch row.Dimension {
		case DimensionReferrer, DimensionUTMSource, DimensionUTMMedium, DimensionUTMCampaign, DimensionCountry:
			t.Errorf("dimension %q should have been skipped entirely, got %+v", row.Dimension, row)
		}
	}
}

func TestAggregateRollsUpAnonymousVisitors(t *testing.T) {
	workspace := uuid.New()
	day := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	events := []enriched{
		enrich(ClickEvent{LinkID: uuid.New(), WorkspaceID: workspace, ClickedAt: day, IPHash: "visitor-a"}),
		enrich(ClickEvent{LinkID: uuid.New(), WorkspaceID: workspace, ClickedAt: day.Add(time.Hour), IPHash: "visitor-a"}),
		enrich(ClickEvent{LinkID: uuid.New(), WorkspaceID: workspace, ClickedAt: day, IPHash: "visitor-b"}),
	}

	visitors := map[string]int64{}
	for _, row := range aggregate(events).Dimensions {
		if row.Dimension == DimensionVisitor {
			visitors[row.Value] = row.Clicks
		}
	}
	if visitors["visitor-a"] != 2 || visitors["visitor-b"] != 1 || len(visitors) != 2 {
		t.Fatalf("unexpected visitor rollup: %#v", visitors)
	}
}

func TestAggregateIsDeterministicallyOrdered(t *testing.T) {
	// Two transactions upserting the same keys in different orders deadlock,
	// so the order the aggregate produces must not depend on map iteration.
	workspace := uuid.New()
	base := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)

	var events []enriched
	for i := range 12 {
		events = append(events, enrich(ClickEvent{
			LinkID:      uuid.New(),
			WorkspaceID: workspace,
			ClickedAt:   base.Add(time.Duration(i) * time.Hour),
			UTMSource:   string(rune('a' + i)),
		}))
	}

	first := aggregate(events)
	for range 20 {
		next := aggregate(events)
		for i := range first.Hourly {
			if first.Hourly[i] != next.Hourly[i] {
				t.Fatalf("hourly order is unstable at %d: %+v vs %+v", i, first.Hourly[i], next.Hourly[i])
			}
		}
		for i := range first.Dimensions {
			if first.Dimensions[i] != next.Dimensions[i] {
				t.Fatalf("dimension order is unstable at %d: %+v vs %+v", i, first.Dimensions[i], next.Dimensions[i])
			}
		}
		for i := range first.Links {
			if first.Links[i] != next.Links[i] {
				t.Fatalf("link order is unstable at %d: %+v vs %+v", i, first.Links[i], next.Links[i])
			}
		}
	}
}

func TestAggregateClipsOversizedDimensionValues(t *testing.T) {
	ev := enrich(ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Now().UTC(),
		UTMCampaign: strings.Repeat("é", 400),
	})

	for _, row := range aggregate([]enriched{ev}).Dimensions {
		if len([]rune(row.Value)) > maxDimensionValue {
			t.Errorf("dimension %q value is %d runes, column holds %d",
				row.Dimension, len([]rune(row.Value)), maxDimensionValue)
		}
	}
}

func TestAggregateEmptyBatch(t *testing.T) {
	got := aggregate(nil)
	if len(got.Hourly) != 0 || len(got.Dimensions) != 0 || len(got.Links) != 0 {
		t.Errorf("an empty batch should produce no writes, got %+v", got)
	}
}

func TestClip(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short enough", "abc", 8, "abc"},
		{"exactly at the limit", "abcd", 4, "abcd"},
		{"truncated", "abcdef", 4, "abcd"},
		{"multibyte counted as runes", "ééééé", 3, "ééé"},
		{"empty", "", 4, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clip(tc.in, tc.max); got != tc.want {
				t.Errorf("clip(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestNullableMapsEmptyToNil(t *testing.T) {
	if got := nullable("", 10); got != nil {
		t.Errorf("an empty value should become SQL NULL, got %#v", got)
	}
	if got := nullable("id", 2); got != "id" {
		t.Errorf("got %#v want %q", got, "id")
	}
}

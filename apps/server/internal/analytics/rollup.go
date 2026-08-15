package analytics

import (
	"bytes"
	"cmp"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/urlx"
)

// Dimension names stored in click_dimension_daily.dimension (VARCHAR(24)).
// One generic table rather than one column per breakdown, so a new dimension
// is a new string here instead of a migration.
const (
	DimensionReferrer    = "referrer"
	DimensionUTMSource   = "utm_source"
	DimensionUTMMedium   = "utm_medium"
	DimensionUTMCampaign = "utm_campaign"
	DimensionDevice      = "device"
	DimensionOS          = "os"
	DimensionBrowser     = "browser"
	DimensionCountry     = "country"
	// Visitor stores only the keyed, irreversible IP hash. Keeping it in the
	// generic rollup makes distinct visitors queryable without retaining an IP.
	DimensionVisitor = "visitor"
)

// enriched is a stream event plus everything the worker derives from it. The
// derivation happens here rather than on the redirect path (§45: no expensive
// metadata parsing during a redirect).
type enriched struct {
	ClickEvent
	Device       string
	OS           string
	Browser      string
	ReferrerHost string
}

// enrich parses the user agent and reduces the referrer to its host, so a
// query string in someone's Referer header never lands in the database.
func enrich(ev ClickEvent) enriched {
	device, os, browser := ParseUserAgent(ev.UserAgent)
	return enriched{
		ClickEvent:   ev,
		Device:       device,
		OS:           os,
		Browser:      browser,
		ReferrerHost: urlx.ReferrerHost(ev.Referrer),
	}
}

// HourlyBucket is one row for click_hourly.
type HourlyBucket struct {
	WorkspaceID uuid.UUID
	LinkID      uuid.UUID
	Bucket      time.Time
	Clicks      int64
}

// DimensionBucket is one row for click_dimension_daily.
type DimensionBucket struct {
	WorkspaceID uuid.UUID
	Day         time.Time
	Dimension   string
	Value       string
	Clicks      int64
}

// LinkCount is one links.click_count increment.
type LinkCount struct {
	LinkID uuid.UUID
	Clicks int64
}

// Rollup is a batch of events collapsed into the writes it implies. A batch of
// 256 clicks typically becomes a handful of upserts, which is the whole reason
// PostgreSQL keeps up without ClickHouse.
type Rollup struct {
	Hourly     []HourlyBucket
	Dimensions []DimensionBucket
	Links      []LinkCount
}

type hourlyKey struct {
	workspaceID uuid.UUID
	linkID      uuid.UUID
	bucket      time.Time
}

type dimensionKey struct {
	workspaceID uuid.UUID
	day         time.Time
	dimension   string
	value       string
}

// aggregate folds a batch of events into per-key counters in memory.
//
// The result is sorted, and that is load-bearing rather than cosmetic: several
// worker containers can hold overlapping keys at once, and two transactions
// issuing the same upserts in different orders deadlock. A total order on the
// keys makes that impossible.
func aggregate(events []enriched) Rollup {
	hourly := make(map[hourlyKey]int64)
	dimensions := make(map[dimensionKey]int64)
	links := make(map[uuid.UUID]int64)

	for _, ev := range events {
		at := ev.ClickedAt.UTC()

		hourly[hourlyKey{
			workspaceID: ev.WorkspaceID,
			linkID:      ev.LinkID,
			bucket:      at.Truncate(time.Hour),
		}]++

		links[ev.LinkID]++

		day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
		for dimension, value := range map[string]string{
			DimensionReferrer:    ev.ReferrerHost,
			DimensionUTMSource:   ev.UTMSource,
			DimensionUTMMedium:   ev.UTMMedium,
			DimensionUTMCampaign: ev.UTMCampaign,
			DimensionDevice:      ev.Device,
			DimensionOS:          ev.OS,
			DimensionBrowser:     ev.Browser,
			DimensionCountry:     ev.Country,
			DimensionVisitor:     ev.IPHash,
		} {
			// An empty value carries no information and would still cost a row
			// in every breakdown query, so it is never stored.
			if value == "" {
				continue
			}
			dimensions[dimensionKey{
				workspaceID: ev.WorkspaceID,
				day:         day,
				dimension:   dimension,
				value:       clip(value, maxDimensionValue),
			}]++
		}
	}

	out := Rollup{
		Hourly:     make([]HourlyBucket, 0, len(hourly)),
		Dimensions: make([]DimensionBucket, 0, len(dimensions)),
		Links:      make([]LinkCount, 0, len(links)),
	}
	for key, clicks := range hourly {
		out.Hourly = append(out.Hourly, HourlyBucket{
			WorkspaceID: key.workspaceID,
			LinkID:      key.linkID,
			Bucket:      key.bucket,
			Clicks:      clicks,
		})
	}
	for key, clicks := range dimensions {
		out.Dimensions = append(out.Dimensions, DimensionBucket{
			WorkspaceID: key.workspaceID,
			Day:         key.day,
			Dimension:   key.dimension,
			Value:       key.value,
			Clicks:      clicks,
		})
	}
	for linkID, clicks := range links {
		out.Links = append(out.Links, LinkCount{LinkID: linkID, Clicks: clicks})
	}

	slices.SortFunc(out.Hourly, func(a, b HourlyBucket) int {
		return cmp.Or(
			compareUUID(a.LinkID, b.LinkID),
			a.Bucket.Compare(b.Bucket),
		)
	})
	slices.SortFunc(out.Dimensions, func(a, b DimensionBucket) int {
		return cmp.Or(
			compareUUID(a.WorkspaceID, b.WorkspaceID),
			a.Day.Compare(b.Day),
			cmp.Compare(a.Dimension, b.Dimension),
			cmp.Compare(a.Value, b.Value),
		)
	})
	slices.SortFunc(out.Links, func(a, b LinkCount) int {
		return compareUUID(a.LinkID, b.LinkID)
	})

	return out
}

func compareUUID(a, b uuid.UUID) int {
	return bytes.Compare(a[:], b[:])
}

// Package analytics implements the click pipeline: a non-blocking Redis
// Streams producer on the redirect path, a background worker that drains the
// stream into PostgreSQL rollup tables, and the reporting API that reads those
// rollups.
//
// The split exists because the redirect path is latency-critical (§45 of the
// plan): it must never wait on PostgreSQL. The producer hands the event to a
// buffered channel and returns; everything else happens elsewhere.
package analytics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Stream field names. They are deliberately short — every byte is multiplied
// by the stream's max length — and must stay stable, because a running worker
// may still be draining entries written by an older binary. Add new fields,
// never rename or reuse one.
const (
	fieldLinkID      = "l"
	fieldWorkspaceID = "w"
	fieldClickedAt   = "t"
	fieldIPHash      = "i"
	fieldUserAgent   = "ua"
	fieldReferrer    = "r"
	fieldUTMSource   = "us"
	fieldUTMMedium   = "um"
	fieldUTMCampaign = "uc"
	fieldCountry     = "c"
)

// ClickEvent is one redirect, as it travels through the Redis stream. It holds
// only what the redirect handler already has in hand: no parsing, no GeoIP, no
// database lookup happens before it is enqueued.
type ClickEvent struct {
	LinkID      uuid.UUID
	WorkspaceID uuid.UUID
	ClickedAt   time.Time
	IPHash      string
	UserAgent   string
	Referrer    string
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	Country     string // reserved for GeoIP enrichment later; empty for now
}

// Fields renders the event as a Redis stream entry. Empty optional values are
// omitted rather than written as "", which keeps the common entry small.
//
// The timestamp travels as Unix nanoseconds: it is compact, ordering-safe, and
// unambiguous about the zone, unlike a formatted string.
func (e ClickEvent) Fields() map[string]any {
	values := map[string]any{
		fieldLinkID:      e.LinkID.String(),
		fieldWorkspaceID: e.WorkspaceID.String(),
		fieldClickedAt:   strconv.FormatInt(e.ClickedAt.UTC().UnixNano(), 10),
	}
	for key, value := range map[string]string{
		fieldIPHash:      e.IPHash,
		fieldUserAgent:   e.UserAgent,
		fieldReferrer:    e.Referrer,
		fieldUTMSource:   e.UTMSource,
		fieldUTMMedium:   e.UTMMedium,
		fieldUTMCampaign: e.UTMCampaign,
		fieldCountry:     e.Country,
	} {
		if value != "" {
			values[key] = value
		}
	}
	return values
}

// ParseEvent reconstructs an event from a stream entry. An error here means
// the entry is malformed and will never parse, so the caller must treat it as
// a poison message rather than retrying it forever.
func ParseEvent(values map[string]any) (ClickEvent, error) {
	linkID, err := uuid.Parse(entryString(values[fieldLinkID]))
	if err != nil {
		return ClickEvent{}, fmt.Errorf("link id: %w", err)
	}
	workspaceID, err := uuid.Parse(entryString(values[fieldWorkspaceID]))
	if err != nil {
		return ClickEvent{}, fmt.Errorf("workspace id: %w", err)
	}
	nanos, err := strconv.ParseInt(entryString(values[fieldClickedAt]), 10, 64)
	if err != nil {
		return ClickEvent{}, fmt.Errorf("clicked at: %w", err)
	}

	return ClickEvent{
		LinkID:      linkID,
		WorkspaceID: workspaceID,
		ClickedAt:   time.Unix(0, nanos).UTC(),
		IPHash:      entryString(values[fieldIPHash]),
		UserAgent:   entryString(values[fieldUserAgent]),
		Referrer:    entryString(values[fieldReferrer]),
		UTMSource:   entryString(values[fieldUTMSource]),
		UTMMedium:   entryString(values[fieldUTMMedium]),
		UTMCampaign: entryString(values[fieldUTMCampaign]),
		Country:     entryString(values[fieldCountry]),
	}, nil
}

// entryString normalises a stream value. go-redis decodes entries as
// map[string]interface{} holding strings, but RESP3 and future client versions
// may hand back []byte, so both are accepted.
func entryString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}

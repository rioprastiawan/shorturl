package analytics

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClickEventRoundTrip(t *testing.T) {
	linkID := uuid.MustParse("6f1a2b3c-4d5e-4f60-8123-456789abcdef")
	workspaceID := uuid.MustParse("11111111-2222-4333-8444-555555555555")

	tests := []struct {
		name  string
		event ClickEvent
	}{
		{
			name: "every field populated",
			event: ClickEvent{
				LinkID:      linkID,
				WorkspaceID: workspaceID,
				ClickedAt:   time.Date(2026, 8, 15, 13, 45, 12, 123456789, time.UTC),
				IPHash:      "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
				UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0",
				Referrer:    "https://news.example.com/story?utm_source=x#frag",
				UTMSource:   "newsletter",
				UTMMedium:   "email",
				UTMCampaign: "launch-2026",
				Country:     "ID",
			},
		},
		{
			name: "only the required fields",
			event: ClickEvent{
				LinkID:      linkID,
				WorkspaceID: workspaceID,
				ClickedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "unicode survives",
			event: ClickEvent{
				LinkID:      linkID,
				WorkspaceID: workspaceID,
				ClickedAt:   time.Date(2026, 3, 9, 7, 8, 9, 42, time.UTC),
				UTMCampaign: "kampanye-lebaran-🎉",
				UserAgent:   "Mozilla/5.0 (Linux; Android 14; SM-S911B) SamsungBrowser/23.0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEvent(tc.event.Fields())
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			if got != tc.event {
				t.Errorf("round trip changed the event:\n got %+v\nwant %+v", got, tc.event)
			}
		})
	}
}

func TestClickEventFieldsOmitsEmptyValues(t *testing.T) {
	ev := ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Now().UTC(),
		UTMSource:   "twitter",
	}

	fields := ev.Fields()
	if _, ok := fields[fieldReferrer]; ok {
		t.Error("an empty referrer should not be written to the stream")
	}
	if _, ok := fields[fieldUTMSource]; !ok {
		t.Error("a populated utm_source should be written to the stream")
	}
	if len(fields) != 4 {
		t.Errorf("expected 4 fields (link, workspace, time, utm_source), got %d: %v", len(fields), fields)
	}
}

func TestClickEventNormalisesToUTC(t *testing.T) {
	zone := time.FixedZone("WIB", 7*60*60)
	ev := ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Date(2026, 8, 15, 20, 30, 0, 0, zone),
	}

	got, err := ParseEvent(ev.Fields())
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if !got.ClickedAt.Equal(ev.ClickedAt) {
		t.Errorf("instant changed: got %s want %s", got.ClickedAt, ev.ClickedAt)
	}
	if got.ClickedAt.Location() != time.UTC {
		t.Errorf("expected UTC, got %s", got.ClickedAt.Location())
	}
}

func TestParseEventRejectsMalformedEntries(t *testing.T) {
	valid := ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Now().UTC(),
	}.Fields()

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing link id", func(m map[string]any) { delete(m, fieldLinkID) }},
		{"link id is not a uuid", func(m map[string]any) { m[fieldLinkID] = "not-a-uuid" }},
		{"missing workspace id", func(m map[string]any) { delete(m, fieldWorkspaceID) }},
		{"timestamp is not a number", func(m map[string]any) { m[fieldClickedAt] = "yesterday" }},
		{"missing timestamp", func(m map[string]any) { delete(m, fieldClickedAt) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := make(map[string]any, len(valid))
			for k, v := range valid {
				entry[k] = v
			}
			tc.mutate(entry)

			if _, err := ParseEvent(entry); err == nil {
				t.Error("expected an error so the entry is treated as poison, got nil")
			}
		})
	}
}

func TestParseEventAcceptsByteValues(t *testing.T) {
	ev := ClickEvent{
		LinkID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ClickedAt:   time.Now().UTC().Truncate(time.Nanosecond),
		UTMSource:   "email",
	}

	entry := make(map[string]any)
	for k, v := range ev.Fields() {
		entry[k] = []byte(v.(string))
	}

	got, err := ParseEvent(entry)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if got.UTMSource != "email" {
		t.Errorf("utm_source: got %q want %q", got.UTMSource, "email")
	}
}

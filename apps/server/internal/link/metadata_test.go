package link

import (
	"encoding/json"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractPageMetadata(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<!doctype html><html><head>
<title>Fallback title</title>
<meta property="og:title" content="Launch Day">
<meta property="og:description" content="  A useful   preview. ">
<meta property="og:image" content="/images/card.jpg">
<meta property="og:site_name" content="Example Inc">
<link rel="icon" href="/assets/icon.png">
<link rel="canonical" href="/launch">
</head></html>`))
	if err != nil {
		t.Fatal(err)
	}
	base := mustURL(t, "https://example.com/articles/source")
	got := extractPageMetadata(doc, base)

	if got.Title != "Launch Day" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Description != "A useful preview." {
		t.Errorf("description = %q", got.Description)
	}
	if got.ImageURL != "https://example.com/images/card.jpg" {
		t.Errorf("image = %q", got.ImageURL)
	}
	if got.FaviconURL != "https://example.com/assets/icon.png" {
		t.Errorf("favicon = %q", got.FaviconURL)
	}
	if got.CanonicalURL != "https://example.com/launch" {
		t.Errorf("canonical = %q", got.CanonicalURL)
	}
}

func TestPublicIPGuard(t *testing.T) {
	tests := map[string]bool{
		"8.8.8.8":              true,
		"2606:4700:4700::1111": true,
		"127.0.0.1":            false,
		"10.0.0.1":             false,
		"169.254.169.254":      false,
		"100.64.0.1":           false,
		"192.0.2.1":            false,
		"::1":                  false,
		"fc00::1":              false,
		"2001:db8::1":          false,
	}
	for raw, want := range tests {
		if got := isPublicIP(netip.MustParseAddr(raw)); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestMergePreviewMetadataPreservesCallerFields(t *testing.T) {
	raw, err := mergePreviewMetadata(json.RawMessage(`{"source":"crm"}`), PageMetadata{Title: "Example"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["source"] != "crm" || got["preview"] == nil {
		t.Fatalf("merged = %#v", got)
	}
	if err := validateMetadataObject(json.RawMessage(`[]`)); err == nil {
		t.Fatal("array metadata should be rejected")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

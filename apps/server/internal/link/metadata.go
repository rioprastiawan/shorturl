package link

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	metadataTimeout   = 6 * time.Second
	maxHTMLBytes      = 1 << 20 // 1 MiB is ample for a document head.
	maxMetadataField  = 2048
	metadataUserAgent = "ShortURL-Metadata/1.0"
)

// PageMetadata is safe, display-oriented metadata extracted from a destination
// page. It is persisted under metadata.preview so the dashboard never has to
// revisit the third-party page merely to render a hover card.
type PageMetadata struct {
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty"`
	ImageURL     string    `json:"image_url,omitempty"`
	FaviconURL   string    `json:"favicon_url,omitempty"`
	SiteName     string    `json:"site_name,omitempty"`
	CanonicalURL string    `json:"canonical_url,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// FetchPageMetadata fetches one public HTML page with strict SSRF boundaries.
func FetchPageMetadata(ctx context.Context, rawURL string) (PageMetadata, error) {
	target, err := url.Parse(rawURL)
	if err != nil || target.Hostname() == "" || !slices.Contains([]string{"http", "https"}, target.Scheme) {
		return PageMetadata{}, errors.New("destination must be a public HTTP or HTTPS URL")
	}

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isPublicIP(ip) {
					return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				}
			}
			return nil, fmt.Errorf("destination host %q does not resolve to a public address", host)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   metadataTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many redirects")
			}
			if !slices.Contains([]string{"http", "https"}, req.URL.Scheme) {
				return errors.New("redirected to an unsupported URL scheme")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return PageMetadata{}, err
	}
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("fetch destination metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PageMetadata{}, fmt.Errorf("destination returned HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return PageMetadata{}, errors.New("destination is not an HTML page")
	}

	limited := io.LimitReader(resp.Body, maxHTMLBytes+1)
	doc, err := html.Parse(limited)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("parse destination HTML: %w", err)
	}
	meta := extractPageMetadata(doc, resp.Request.URL)
	meta.FetchedAt = time.Now().UTC()
	return meta, nil
}

func isPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, reserved := range []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("2001:db8::/32"),
	} {
		if reserved.Contains(ip) {
			return false
		}
	}
	return true
}

func extractPageMetadata(doc *html.Node, base *url.URL) PageMetadata {
	var meta PageMetadata
	var documentTitle, iconHref, canonicalHref string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if documentTitle == "" && n.FirstChild != nil {
					documentTitle = n.FirstChild.Data
				}
			case "meta":
				attrs := attributes(n)
				key := strings.ToLower(firstNonEmpty(attrs["property"], attrs["name"]))
				value := attrs["content"]
				switch key {
				case "og:title", "twitter:title":
					if meta.Title == "" {
						meta.Title = value
					}
				case "og:description", "twitter:description", "description":
					if meta.Description == "" {
						meta.Description = value
					}
				case "og:image", "twitter:image", "twitter:image:src":
					if meta.ImageURL == "" {
						meta.ImageURL = resolveWebURL(base, value)
					}
				case "og:site_name":
					meta.SiteName = value
				case "og:url":
					if meta.CanonicalURL == "" {
						meta.CanonicalURL = resolveWebURL(base, value)
					}
				}
			case "link":
				attrs := attributes(n)
				rel := strings.ToLower(attrs["rel"])
				if iconHref == "" && strings.Contains(rel, "icon") {
					iconHref = attrs["href"]
				}
				if canonicalHref == "" && strings.Contains(rel, "canonical") {
					canonicalHref = attrs["href"]
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	if meta.Title == "" {
		meta.Title = documentTitle
	}
	if meta.CanonicalURL == "" {
		meta.CanonicalURL = resolveWebURL(base, canonicalHref)
	}
	meta.FaviconURL = resolveWebURL(base, iconHref)
	if meta.FaviconURL == "" {
		root := *base
		root.Path, root.RawQuery, root.Fragment = "/favicon.ico", "", ""
		meta.FaviconURL = root.String()
	}
	meta.Title = cleanMetadataText(meta.Title, 255)
	meta.Description = cleanMetadataText(meta.Description, 500)
	meta.SiteName = cleanMetadataText(meta.SiteName, 120)
	return meta
}

func attributes(n *html.Node) map[string]string {
	out := make(map[string]string, len(n.Attr))
	for _, attr := range n.Attr {
		out[strings.ToLower(attr.Key)] = strings.TrimSpace(attr.Val)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cleanMetadataText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
}

func resolveWebURL(base *url.URL, raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(parsed)
	if !slices.Contains([]string{"http", "https"}, resolved.Scheme) {
		return ""
	}
	if len(resolved.String()) > maxMetadataField {
		return ""
	}
	return resolved.String()
}

func mergePreviewMetadata(raw json.RawMessage, preview PageMetadata) (json.RawMessage, error) {
	object := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, errors.New("metadata must be a valid JSON object")
		}
	}
	object["preview"] = preview
	merged, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	if len(merged) > MaxMetadataBytes {
		return nil, fmt.Errorf("metadata with preview exceeds %d bytes", MaxMetadataBytes)
	}
	return merged, nil
}

func previewFromMetadata(raw json.RawMessage) (PageMetadata, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return PageMetadata{}, false
	}
	var envelope struct {
		Preview PageMetadata `json:"preview"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Preview.FetchedAt.IsZero() {
		return PageMetadata{}, false
	}
	return envelope.Preview, true
}

func validateMetadataObject(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return errors.New("must be a valid JSON object")
	}
	return nil
}

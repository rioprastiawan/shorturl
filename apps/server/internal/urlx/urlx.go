// Package urlx validates destination URLs and normalises hostnames.
package urlx

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// MaxURLLength bounds a destination. Browsers and proxies start truncating
// well before this, and it stops a caller from using the shortener as storage.
const MaxURLLength = 2048

// privateRanges are refused when the server resolves a URL itself. They are
// the classic SSRF targets: loopback, RFC1918, link-local (including the cloud
// metadata endpoint at 169.254.169.254), and their IPv6 equivalents.
var privateRanges = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

// ValidateDestination checks a URL a user or integration wants to redirect to.
//
// It deliberately does NOT block private addresses. A self-hosted shortener's
// main job at this company is sharing internal document links, so
// http://10.0.0.5/report is a legitimate destination. The browser performs the
// request, not the server, so this is not an SSRF vector. The SSRF guard below
// applies only where the *server* would fetch a URL.
func ValidateDestination(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return "", fmt.Errorf("must not be empty")
	case len(raw) > MaxURLLength:
		return "", fmt.Errorf("must be at most %d characters", MaxURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("must be a valid URL")
	}

	// Only http and https. Rejecting javascript:, data:, file:, and friends is
	// the important part: a redirect to javascript: is a stored XSS primitive.
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	case "":
		return "", fmt.Errorf("must include a scheme, for example https://")
	default:
		return "", fmt.Errorf("must use the http or https scheme")
	}

	if u.Host == "" {
		return "", fmt.Errorf("must include a host")
	}

	return u.String(), nil
}

// IsPrivateAddress reports whether a resolved IP is in a private or
// link-local range. Use this before any request the server itself makes.
func IsPrivateAddress(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true // unparseable: refuse rather than guess
	}
	addr = addr.Unmap()
	for _, prefix := range privateRanges {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// NormalizeHostname lowercases a hostname and strips any port, so that
// "GO.Example.com:443" and "go.example.com" resolve to the same domain row.
func NormalizeHostname(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	// A trailing dot is a valid fully-qualified name but never stored.
	return strings.TrimSuffix(host, ".")
}

// ValidateHostname checks a custom domain a user is adding.
func ValidateHostname(host string) (string, error) {
	normalized := NormalizeHostname(host)
	switch {
	case normalized == "":
		return "", fmt.Errorf("must not be empty")
	case len(normalized) > 253:
		return "", fmt.Errorf("must be at most 253 characters")
	case !strings.Contains(normalized, "."):
		return "", fmt.Errorf("must be a fully-qualified domain name, for example go.example.com")
	case net.ParseIP(normalized) != nil:
		return "", fmt.Errorf("must be a domain name, not an IP address")
	}

	for _, label := range strings.Split(normalized, ".") {
		if label == "" {
			return "", fmt.Errorf("must not contain an empty label")
		}
		if len(label) > 63 {
			return "", fmt.Errorf("each label must be at most 63 characters")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("labels must not start or end with a hyphen")
		}
		for _, r := range label {
			isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			if !isAllowed {
				return "", fmt.Errorf("may only contain letters, digits, hyphens, and dots")
			}
		}
	}

	return normalized, nil
}

// ReferrerHost extracts the host from a Referer header for analytics, dropping
// the path so that query strings and document paths are never stored.
func ReferrerHost(referrer string) string {
	if referrer == "" {
		return ""
	}
	u, err := url.Parse(referrer)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

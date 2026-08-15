package urlx

import (
	"net"
	"strings"
	"testing"
)

func TestValidateDestinationAcceptsHTTPAndHTTPS(t *testing.T) {
	valid := []string{
		"https://example.com",
		"http://example.com/path?query=1#frag",
		"https://internal.company.com/documents/very-long?token=abc",
		"http://10.0.0.5/report.pdf", // internal targets are legitimate here
		"https://xn--80ak6aa92e.com",
	}
	for _, raw := range valid {
		if _, err := ValidateDestination(raw); err != nil {
			t.Errorf("ValidateDestination(%q) = %v, want nil", raw, err)
		}
	}
}

func TestValidateDestinationRejectsDangerousSchemes(t *testing.T) {
	// A redirect to javascript: is a stored XSS primitive; the others are
	// either useless or a local-file disclosure vector.
	dangerous := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"file:///etc/passwd",
		"ftp://example.com/f",
		"vbscript:msgbox(1)",
	}
	for _, raw := range dangerous {
		t.Run(raw, func(t *testing.T) {
			if _, err := ValidateDestination(raw); err == nil {
				t.Errorf("ValidateDestination(%q) = nil, want rejection", raw)
			}
		})
	}
}

func TestValidateDestinationRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"whitespace":  "   ",
		"no scheme":   "example.com",
		"no host":     "https://",
		"over length": "https://example.com/" + strings.Repeat("a", MaxURLLength),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateDestination(raw); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestNormalizeHostname(t *testing.T) {
	cases := map[string]string{
		"GO.Example.COM":     "go.example.com",
		"go.example.com:443": "go.example.com",
		"go.example.com.":    "go.example.com",
		"  go.example.com  ": "go.example.com",
		"":                   "",
	}
	for in, want := range cases {
		if got := NormalizeHostname(in); got != want {
			t.Errorf("NormalizeHostname(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateHostname(t *testing.T) {
	valid := []string{"go.example.com", "link.example.org", "short.company.id", "a.b.c.d.example.com"}
	for _, h := range valid {
		if _, err := ValidateHostname(h); err != nil {
			t.Errorf("ValidateHostname(%q) = %v, want nil", h, err)
		}
	}

	invalid := map[string]string{
		"empty":           "",
		"no dot":          "localhost",
		"ipv4":            "192.168.1.1",
		"ipv6":            "::1",
		"empty label":     "go..example.com",
		"leading hyphen":  "-go.example.com",
		"trailing hyphen": "go-.example.com",
		"underscore":      "go_link.example.com",
		"space":           "go example.com",
		"label too long":  strings.Repeat("a", 64) + ".example.com",
		"too long":        strings.Repeat("a.", 130) + "com",
	}
	for name, h := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateHostname(h); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestValidateHostnameNormalizes(t *testing.T) {
	got, err := ValidateHostname("GO.Example.COM:8443")
	if err != nil {
		t.Fatalf("ValidateHostname() error = %v", err)
	}
	if got != "go.example.com" {
		t.Errorf("got %q, want the lowercased hostname with the port stripped", got)
	}
}

func TestIsPrivateAddress(t *testing.T) {
	private := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", // the cloud metadata endpoint
		"::1", "fc00::1", "fe80::1",
	}
	for _, raw := range private {
		if !IsPrivateAddress(net.ParseIP(raw)) {
			t.Errorf("IsPrivateAddress(%s) = false, want true", raw)
		}
	}

	public := []string{"8.8.8.8", "203.0.113.5", "2001:4860:4860::8888"}
	for _, raw := range public {
		if IsPrivateAddress(net.ParseIP(raw)) {
			t.Errorf("IsPrivateAddress(%s) = true, want false", raw)
		}
	}

	if !IsPrivateAddress(nil) {
		t.Error("IsPrivateAddress(nil) = false; an unparseable address must be refused, not allowed")
	}
}

func TestReferrerHost(t *testing.T) {
	cases := map[string]string{
		"https://news.example.com/article/123?utm=x": "news.example.com",
		"http://Example.COM/":                        "example.com",
		"https://example.com:8443/x":                 "example.com",
		"":                                           "",
		"not a url":                                  "",
		"/relative/path":                             "",
	}
	for in, want := range cases {
		if got := ReferrerHost(in); got != want {
			t.Errorf("ReferrerHost(%q) = %q, want %q", in, got, want)
		}
	}
}

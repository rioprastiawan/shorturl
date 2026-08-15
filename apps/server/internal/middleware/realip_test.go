package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRealIP(t *testing.T) {
	// Matches the production default: Traefik reaches the server over a
	// private Docker bridge network.
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("127.0.0.0/8"),
	}

	tests := map[string]struct {
		remoteAddr string
		forwarded  string
		realIP     string
		want       string
		why        string
	}{
		"direct connection, no headers": {
			remoteAddr: "203.0.113.5:1234",
			want:       "203.0.113.5",
			why:        "the socket address is all there is",
		},
		"untrusted peer spoofing XFF": {
			remoteAddr: "203.0.113.5:1234",
			forwarded:  "1.2.3.4",
			want:       "203.0.113.5",
			why:        "a direct client must not be able to choose its own rate-limit key",
		},
		"trusted proxy forwarding a client": {
			remoteAddr: "10.0.0.2:1234",
			forwarded:  "203.0.113.5",
			want:       "203.0.113.5",
			why:        "the header is believable only from our own proxy",
		},
		"chain of trusted proxies": {
			remoteAddr: "10.0.0.2:1234",
			forwarded:  "203.0.113.5, 10.0.0.9, 10.0.0.3",
			want:       "203.0.113.5",
			why:        "walk right to left, skipping our own hops",
		},
		"client spoofs an extra hop through our proxy": {
			remoteAddr: "10.0.0.2:1234",
			forwarded:  "1.2.3.4, 203.0.113.5",
			want:       "203.0.113.5",
			why:        "only the rightmost untrusted entry was actually observed by us",
		},
		"trusted proxy with X-Real-IP": {
			remoteAddr: "10.0.0.2:1234",
			realIP:     "203.0.113.9",
			want:       "203.0.113.9",
			why:        "fall back to X-Real-IP when XFF is absent",
		},
		"untrusted peer with X-Real-IP": {
			remoteAddr: "203.0.113.5:1234",
			realIP:     "1.2.3.4",
			want:       "203.0.113.5",
			why:        "X-Real-IP is no more trustworthy than XFF",
		},
		"all hops trusted": {
			remoteAddr: "10.0.0.2:1234",
			forwarded:  "10.0.0.7, 10.0.0.9",
			want:       "10.0.0.2",
			why:        "the request originated inside the trusted network",
		},
		"garbage in the header": {
			remoteAddr: "10.0.0.2:1234",
			forwarded:  "not-an-ip, 203.0.113.5",
			want:       "203.0.113.5",
			why:        "unparseable entries are skipped, not treated as the client",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			handler := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIP(r)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			if got != tc.want {
				t.Errorf("ClientIP() = %q, want %q\n  %s", got, tc.want, tc.why)
			}
		})
	}
}

func TestClientIPWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	if got := ClientIP(req); got != "203.0.113.5" {
		t.Errorf("ClientIP() = %q, want the peer address as a fallback", got)
	}
}

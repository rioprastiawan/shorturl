package domain

import (
	"context"
	"errors"
	"net"
	"time"
)

// lookupTimeout bounds every DNS query. A user is waiting on the verify
// request, and an unreachable authoritative server would otherwise hold the
// connection open for the whole handler timeout.
const lookupTimeout = 5 * time.Second

// publicDNSServers are queried directly instead of the host's configured
// resolver. A user clicks "Verify" right after publishing a DNS record and
// expects the current answer, but the system resolver (systemd-resolved,
// dnsmasq, a corporate stub, etc.) may hold a stale or negative answer past
// its TTL. Cloudflare is tried first; Google is the fallback if it can't be
// reached.
var publicDNSServers = []string{"1.1.1.1:53", "8.8.8.8:53"}

// Resolver is the slice of DNS the verifier needs. It exists so verification
// can be tested without touching the network.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// netResolver is the production Resolver, backed by direct queries to
// publicDNSServers rather than the system resolver.
type netResolver struct {
	r *net.Resolver
}

// NewResolver returns a Resolver that queries publicDNSServers directly. It
// forces Go's pure-Go DNS client (PreferGo) and ignores the address Go would
// otherwise dial, which is what makes bypassing the OS resolver's cache
// actually take effect — without PreferGo, some platforms resolve through
// cgo/getaddrinfo instead, and Dial is never called at all.
func NewResolver() Resolver {
	dialer := &net.Dialer{Timeout: lookupTimeout}
	return netResolver{r: &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			for _, server := range publicDNSServers {
				conn, err := dialer.DialContext(ctx, network, server)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}}
}

func (n netResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return n.r.LookupTXT(ctx, name)
}

func (n netResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return n.r.LookupHost(ctx, host)
}

func (n netResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return n.r.LookupCNAME(ctx, host)
}

// dnsReason condenses a lookup failure into something safe and useful to show
// a user. The full error names the resolver address, which is infrastructure
// detail they cannot act on.
func dnsReason(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		switch {
		case dnsErr.IsNotFound:
			return "the name does not exist"
		case dnsErr.IsTimeout:
			return "the lookup timed out"
		case dnsErr.Err != "":
			return dnsErr.Err
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "the lookup timed out"
	}
	return "the lookup failed"
}

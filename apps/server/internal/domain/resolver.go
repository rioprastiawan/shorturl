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

// Resolver is the slice of DNS the verifier needs. It exists so verification
// can be tested without touching the network.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// netResolver is the production Resolver, backed by the system resolver.
type netResolver struct {
	r *net.Resolver
}

// NewResolver returns a Resolver that queries the system's configured DNS
// servers.
func NewResolver() Resolver { return netResolver{r: net.DefaultResolver} }

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

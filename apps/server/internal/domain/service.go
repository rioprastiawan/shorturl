// Package domain manages custom hostnames: adding them, proving the user
// controls them, pointing Traefik at them, and retiring them.
//
// Verification deliberately asks two independent questions, because either one
// alone is unsafe. A TXT record proves the user controls the *name*, but a name
// whose A record points somewhere else would produce a domain we happily serve
// certificates for and never receive traffic on. Address records prove traffic
// *arrives here*, but anyone can point a hostname at a public IP, so on their
// own they would let a stranger attach their domain to this installation. Both
// must hold before a domain goes active.
package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/security"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/urlx"
)

// Domain lifecycle states, mirroring the domains_status_valid constraint.
const (
	StatusPending = "pending"
	StatusActive  = "active"
	StatusFailed  = "failed"
)

// Certificate states. Traefik issues asynchronously, so a freshly verified
// domain reports pending until the ACME exchange completes.
const (
	SSLPending = "pending"
	SSLActive  = "active"
)

// MethodTXT is the only verification method the MVP offers. The column allows
// dns_cname and dns_a as well, for a later self-service choice.
const MethodTXT = "dns_txt"

const (
	// verificationLabel is prefixed to the hostname to form the TXT record name.
	verificationLabel = "_shorturl-verification"
	// tokenPrefix namespaces the TXT value so an unrelated record on the same
	// name is never mistaken for ours.
	tokenPrefix = "shorturl-verification="
)

// Store is the subset of store.Querier this package uses. Naming it here keeps
// the dependency visible and lets the verification tests run without a
// database. *store.Queries satisfies it.
type Store interface {
	ClearDefaultDomain(ctx context.Context, workspaceID uuid.UUID) error
	CountDomains(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	CountLinksForDomain(ctx context.Context, domainID uuid.UUID) (int64, error)
	CreateDomain(ctx context.Context, arg store.CreateDomainParams) (store.Domain, error)
	DeleteDomain(ctx context.Context, arg store.DeleteDomainParams) (int64, error)
	GetDomainInWorkspace(ctx context.Context, arg store.GetDomainInWorkspaceParams) (store.Domain, error)
	ListActiveDomains(ctx context.Context) ([]store.Domain, error)
	ListDomainHostnamesForWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]string, error)
	ListDomainsPage(ctx context.Context, arg store.ListDomainsPageParams) ([]store.Domain, error)
	ListLinksForDomain(ctx context.Context, domainID uuid.UUID) ([]store.ListLinksForDomainRow, error)
	SetDefaultDomain(ctx context.Context, id uuid.UUID) (store.Domain, error)
	UpdateDomainRootRedirect(ctx context.Context, arg store.UpdateDomainRootRedirectParams) (store.Domain, error)
	UpdateDomainVerification(ctx context.Context, arg store.UpdateDomainVerificationParams) (store.Domain, error)
}

// TxBeginner starts a transaction. *pgxpool.Pool satisfies it; SetDefault needs
// one because clearing the old default and setting the new one must not leave
// a workspace with zero or two defaults.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service holds the domain business rules.
type Service struct {
	q        Store
	db       TxBeginner
	cfg      config.Config
	resolver Resolver
}

// NewService builds the service. db may be nil only in contexts that never call
// SetDefault.
func NewService(q Store, db TxBeginner, cfg config.Config) *Service {
	return &Service{q: q, db: db, cfg: cfg, resolver: NewResolver()}
}

// appDomain is the hostname this installation answers on, normalised the same
// way stored hostnames are so comparisons are meaningful.
func (s *Service) appDomain() string { return urlx.NormalizeHostname(s.cfg.AppDomain) }

// Add registers a hostname for a workspace in the pending state.
func (s *Service) Add(ctx context.Context, workspaceID uuid.UUID, hostname string) (store.Domain, error) {
	normalized, err := urlx.ValidateHostname(hostname)
	if err != nil {
		return store.Domain{}, httpx.Invalid(map[string][]string{"hostname": {err.Error()}})
	}

	// The application's own hostname already has a router and a certificate,
	// and handing it to a workspace would let that workspace shadow the
	// dashboard and API with short links.
	if normalized == s.appDomain() {
		return store.Domain{}, httpx.Conflictf("reserved_hostname",
			"%s is the address of this ShortURL installation and cannot be added as a custom domain", normalized)
	}

	token, err := security.NewToken(16)
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}

	// The first hostname a workspace adds becomes its default, so link creation
	// has something to fall back on without an extra step in the UI.
	existing, err := s.q.ListDomainHostnamesForWorkspace(ctx, workspaceID)
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}

	d, err := s.q.CreateDomain(ctx, store.CreateDomainParams{
		WorkspaceID:        workspaceID,
		Hostname:           normalized,
		VerificationToken:  token,
		VerificationMethod: MethodTXT,
		IsDefault:          len(existing) == 0,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Deliberately vague: saying *which* workspace holds the hostname
			// would turn this endpoint into a tenant-enumeration oracle.
			return store.Domain{}, httpx.Conflictf("hostname_taken", "That hostname is already connected to a workspace")
		}
		return store.Domain{}, httpx.Internal(err)
	}
	return d, nil
}

// List returns one page of domains in the workspace, default first.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]store.Domain, int64, error) {
	total, err := s.q.CountDomains(ctx, workspaceID)
	if err != nil {
		return nil, 0, httpx.Internal(err)
	}
	domains, err := s.q.ListDomainsPage(ctx, store.ListDomainsPageParams{
		WorkspaceID: workspaceID,
		PageLimit:   int32(limit),
		PageOffset:  int32(offset),
	})
	if err != nil {
		return nil, 0, httpx.Internal(err)
	}
	return domains, total, nil
}

// Get returns one domain, scoped to the workspace so an ID from another tenant
// is indistinguishable from one that does not exist.
func (s *Service) Get(ctx context.Context, workspaceID, domainID uuid.UUID) (store.Domain, error) {
	d, err := s.q.GetDomainInWorkspace(ctx, store.GetDomainInWorkspaceParams{
		ID:          domainID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Domain{}, httpx.ErrNotFound
		}
		return store.Domain{}, httpx.Internal(err)
	}
	return d, nil
}

// UpdateRootRedirect configures where the bare hostname points. A nil URL
// restores the installation dashboard fallback.
func (s *Service) UpdateRootRedirect(ctx context.Context, workspaceID, domainID uuid.UUID, raw *string) (store.Domain, error) {
	d, err := s.Get(ctx, workspaceID, domainID)
	if err != nil {
		return store.Domain{}, err
	}

	var destination *string
	if raw != nil && strings.TrimSpace(*raw) != "" {
		normalized, validationErr := urlx.ValidateDestination(*raw)
		if validationErr != nil {
			return store.Domain{}, httpx.Invalid(map[string][]string{"root_redirect_url": {validationErr.Error()}})
		}
		parsed, _ := url.Parse(normalized)
		if urlx.NormalizeHostname(parsed.Host) == d.Hostname && strings.Trim(parsed.Path, "/") == "" {
			return store.Domain{}, httpx.Invalid(map[string][]string{"root_redirect_url": {"must not point back to this domain root"}})
		}
		destination = &normalized
	}

	updated, err := s.q.UpdateDomainRootRedirect(ctx, store.UpdateDomainRootRedirectParams{
		ID:              domainID,
		WorkspaceID:     workspaceID,
		RootRedirectUrl: destination,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Domain{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}
	return updated, nil
}

// Verify runs both DNS checks and records the outcome. It never returns an
// error for a failed check: a failure is a valid state of the resource, and the
// dashboard renders verification_error to tell the user what to fix.
func (s *Service) Verify(ctx context.Context, workspaceID, domainID uuid.UUID) (store.Domain, error) {
	d, err := s.Get(ctx, workspaceID, domainID)
	if err != nil {
		return store.Domain{}, err
	}

	result := s.check(ctx, d)

	params := store.UpdateDomainVerificationParams{
		ID:         d.ID,
		Status:     StatusFailed,
		SslStatus:  d.SslStatus,
		VerifiedAt: d.VerifiedAt,
	}
	if result.passed() {
		now := time.Now().UTC()
		params.Status = StatusActive
		// Traefik requests the certificate the moment the router appears, which
		// happens below and takes seconds to minutes. The dashboard shows SSL
		// state separately from DNS state for exactly this window (plan §39).
		params.SslStatus = SSLPending
		params.VerifiedAt = &now
		params.VerificationError = nil
	} else {
		message := result.message
		params.VerificationError = &message
	}

	updated, err := s.q.UpdateDomainVerification(ctx, params)
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}

	if updated.Status == StatusActive {
		s.syncTraefikBestEffort(ctx, "verify", updated.Hostname)
	}
	return updated, nil
}

// SetDefault makes a domain the workspace's default for new links.
func (s *Service) SetDefault(ctx context.Context, workspaceID, domainID uuid.UUID) (store.Domain, error) {
	d, err := s.Get(ctx, workspaceID, domainID)
	if err != nil {
		return store.Domain{}, err
	}
	if d.Status != StatusActive {
		return store.Domain{}, httpx.Conflictf("domain_not_active",
			"Verify %s before making it the default domain", d.Hostname)
	}
	if d.IsDefault {
		return d, nil
	}
	if s.db == nil {
		return store.Domain{}, httpx.Internal(errors.New("domain: service was built without a transaction pool"))
	}

	// A partial unique index allows one default per workspace, so the clear and
	// the set have to land in the same transaction or the update conflicts.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := store.New(tx)
	if err := qtx.ClearDefaultDomain(ctx, workspaceID); err != nil {
		return store.Domain{}, httpx.Internal(err)
	}
	updated, err := qtx.SetDefaultDomain(ctx, d.ID)
	if err != nil {
		return store.Domain{}, httpx.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Domain{}, httpx.Internal(err)
	}
	return updated, nil
}

// Delete removes a domain that carries no links.
func (s *Service) Delete(ctx context.Context, workspaceID, domainID uuid.UUID) error {
	d, err := s.Get(ctx, workspaceID, domainID)
	if err != nil {
		return err
	}

	// The foreign key cascades, so without this check deleting a domain would
	// silently destroy every short link published on it.
	linkCount, err := s.q.CountLinksForDomain(ctx, d.ID)
	if err != nil {
		return httpx.Internal(err)
	}
	if linkCount > 0 {
		return httpx.Conflictf("domain_has_links",
			"%s still has %d links. Delete or move them before removing the domain.",
			d.Hostname, linkCount)
	}

	rows, err := s.q.DeleteDomain(ctx, store.DeleteDomainParams{ID: d.ID, WorkspaceID: workspaceID})
	if err != nil {
		return httpx.Internal(err)
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}

	s.syncTraefikBestEffort(ctx, "delete", d.Hostname)
	return nil
}

// Instructions describes the records the user must create. It performs no I/O,
// so the dashboard can render it alongside any domain it already has.
func (s *Service) Instructions(d store.Domain) DNSInstructions {
	app := s.appDomain()
	return DNSInstructions{
		Verification: DNSRecord{
			Type:  "TXT",
			Name:  verificationLabel + "." + d.Hostname,
			Value: tokenPrefix + d.VerificationToken,
		},
		Routing: []DNSRecord{
			{Type: "CNAME", Name: d.Hostname, Value: app},
			// Offered as an alternative for providers that cannot host a CNAME
			// at the record the user wants. The address is not resolved here
			// because this function stays pure; the operator reads it off their
			// own server, and the UI presents this as guidance.
			{Type: "A", Name: d.Hostname, Value: "the IPv4 address of " + app},
		},
	}
}

// verification is the outcome of the two checks.
type verification struct {
	ownership bool
	routing   bool
	message   string
}

func (v verification) passed() bool { return v.ownership && v.routing }

// check runs both DNS questions and composes one message covering everything
// that is wrong, so a user fixing their DNS sees the whole picture in one pass.
func (s *Service) check(ctx context.Context, d store.Domain) verification {
	app := s.appDomain()

	var v verification
	var problems []string

	ownershipMessage := ""
	v.ownership, ownershipMessage = s.checkOwnership(ctx, d)
	if !v.ownership {
		problems = append(problems, ownershipMessage)
	}

	routingMessage := ""
	v.routing, routingMessage = s.checkRouting(ctx, d.Hostname, app)
	if !v.routing {
		problems = append(problems, routingMessage)
	}

	v.message = strings.Join(problems, " ")
	return v
}

// checkOwnership proves the user controls the name by finding our token in a
// TXT record only they could have published.
func (s *Service) checkOwnership(ctx context.Context, d store.Domain) (bool, string) {
	name := verificationLabel + "." + d.Hostname
	expected := tokenPrefix + d.VerificationToken

	records, err := s.resolver.LookupTXT(ctx, name)
	if err != nil {
		return false, fmt.Sprintf(
			"No TXT record could be read at %s (%s). Add a TXT record named %s with the value %q.",
			name, dnsReason(err), name, expected)
	}

	found := make([]string, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == expected {
			return true, ""
		}
		if record != "" {
			found = append(found, record)
		}
	}

	if len(found) == 0 {
		return false, fmt.Sprintf(
			"No TXT record found at %s. Add a TXT record named %s with the value %q.",
			name, name, expected)
	}
	return false, fmt.Sprintf(
		"The TXT record at %s does not match. Expected %q, found %s.",
		name, expected, quoteAll(found))
}

// checkRouting proves traffic for the hostname actually reaches this
// installation, by matching it against however the application's own domain
// resolves.
func (s *Service) checkRouting(ctx context.Context, hostname, app string) (bool, string) {
	// CNAME first: it is the record we recommend, and it keeps working when the
	// server's address changes.
	cname, cnameErr := s.resolver.LookupCNAME(ctx, hostname)
	canonical := urlx.NormalizeHostname(cname)
	if cnameErr == nil && canonical != "" && canonical == app {
		return true, ""
	}

	hostAddrs, hostErr := s.resolver.LookupHost(ctx, hostname)
	appAddrs, appErr := s.resolver.LookupHost(ctx, app)

	if hostErr == nil && appErr == nil {
		appSet := addrSet(appAddrs)
		for _, addr := range hostAddrs {
			if _, ok := appSet[normalizeAddr(addr)]; ok {
				return true, ""
			}
		}
	}

	expected := fmt.Sprintf("a CNAME record pointing to %s", app)
	if appErr == nil && len(appAddrs) > 0 {
		expected += fmt.Sprintf(", or an A/AAAA record pointing to %s", strings.Join(appAddrs, " or "))
	}

	return false, fmt.Sprintf(
		"%s does not point to this ShortURL installation yet. Expected %s. Found %s. DNS changes can take up to an hour to propagate.",
		hostname, expected, describeRouting(hostname, canonical, cnameErr, hostAddrs, hostErr))
}

// describeRouting states what the hostname resolves to today, in the same terms
// the expectation is phrased in.
func describeRouting(hostname, canonical string, cnameErr error, addrs []string, hostErr error) string {
	var parts []string
	if cnameErr == nil && canonical != "" && canonical != urlx.NormalizeHostname(hostname) {
		parts = append(parts, "a CNAME to "+canonical)
	}
	switch {
	case hostErr != nil:
		parts = append(parts, "no address records ("+dnsReason(hostErr)+")")
	case len(addrs) > 0:
		parts = append(parts, "the address "+strings.Join(addrs, " and "))
	default:
		parts = append(parts, "no address records")
	}
	return strings.Join(parts, " resolving to ")
}

// addrSet indexes addresses by their canonical text form so an IPv6 address
// written two ways still compares equal.
func addrSet(addrs []string) map[string]struct{} {
	set := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		set[normalizeAddr(addr)] = struct{}{}
	}
	return set
}

func normalizeAddr(addr string) string {
	parsed, err := netip.ParseAddr(strings.TrimSpace(addr))
	if err != nil {
		return strings.TrimSpace(addr)
	}
	return parsed.Unmap().String()
}

func quoteAll(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(quoted, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// syncTraefikBestEffort reconciles the router files without letting a
// filesystem problem fail the user's request. The domain row is the source of
// truth; the next sync, including the one at startup, repairs the directory.
func (s *Service) syncTraefikBestEffort(ctx context.Context, operation, hostname string) {
	if err := s.SyncTraefik(ctx); err != nil {
		slog.ErrorContext(ctx, "traefik dynamic config sync failed",
			slog.String("operation", operation),
			slog.String("hostname", hostname),
			slog.String("error", err.Error()),
		)
	}
}

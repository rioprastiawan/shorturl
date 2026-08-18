package domain

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

const (
	testAppDomain = "short.example.com"
	testHostname  = "go.example.com"
	testToken     = "tok3n-abc"
)

// fakeResolver answers from fixed maps and returns an NXDOMAIN-shaped error for
// anything it was not given, the way the real resolver does.
type fakeResolver struct {
	txt   map[string][]string
	hosts map[string][]string
	cname map[string]string
}

func notFound(name string) error {
	return &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (f fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	records, ok := f.txt[name]
	if !ok {
		return nil, notFound(name)
	}
	return records, nil
}

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	addrs, ok := f.hosts[host]
	if !ok {
		return nil, notFound(host)
	}
	return addrs, nil
}

func (f fakeResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if target, ok := f.cname[host]; ok {
		return target + ".", nil
	}
	// net.Resolver returns the host itself when there is no CNAME chain.
	if _, ok := f.hosts[host]; ok {
		return host + ".", nil
	}
	return "", notFound(host)
}

// fakeStore implements Store in memory. Only the methods the tests exercise do
// anything useful.
type fakeStore struct {
	domains    map[uuid.UUID]store.Domain
	hostnames  []string
	links      []store.ListLinksForDomainRow
	createErr  error
	lastCreate store.CreateDomainParams
}

func newFakeStore() *fakeStore {
	return &fakeStore{domains: map[uuid.UUID]store.Domain{}}
}

func (f *fakeStore) ClearDefaultDomain(context.Context, uuid.UUID) error { return nil }

func (f *fakeStore) CountDomains(_ context.Context, workspaceID uuid.UUID) (int64, error) {
	var total int64
	for _, d := range f.domains {
		if d.WorkspaceID == workspaceID {
			total++
		}
	}
	return total, nil
}

func (f *fakeStore) CreateDomain(_ context.Context, arg store.CreateDomainParams) (store.Domain, error) {
	f.lastCreate = arg
	if f.createErr != nil {
		return store.Domain{}, f.createErr
	}
	d := store.Domain{
		ID:                 uuid.New(),
		WorkspaceID:        arg.WorkspaceID,
		Hostname:           arg.Hostname,
		Status:             StatusPending,
		VerificationToken:  arg.VerificationToken,
		VerificationMethod: arg.VerificationMethod,
		SslStatus:          SSLPending,
		IsDefault:          arg.IsDefault,
	}
	f.domains[d.ID] = d
	f.hostnames = append(f.hostnames, d.Hostname)
	return d, nil
}

func (f *fakeStore) DeleteDomain(_ context.Context, arg store.DeleteDomainParams) (int64, error) {
	if _, ok := f.domains[arg.ID]; !ok {
		return 0, nil
	}
	delete(f.domains, arg.ID)
	return 1, nil
}

func (f *fakeStore) GetDomainInWorkspace(_ context.Context, arg store.GetDomainInWorkspaceParams) (store.Domain, error) {
	d, ok := f.domains[arg.ID]
	if !ok || d.WorkspaceID != arg.WorkspaceID {
		return store.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

func (f *fakeStore) ListActiveDomains(context.Context) ([]store.Domain, error) {
	var out []store.Domain
	for _, d := range f.domains {
		if d.Status == StatusActive {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeStore) ListDomainHostnamesForWorkspace(context.Context, uuid.UUID) ([]string, error) {
	return f.hostnames, nil
}

func (f *fakeStore) ListDomainsPage(_ context.Context, arg store.ListDomainsPageParams) ([]store.Domain, error) {
	var out []store.Domain
	for _, d := range f.domains {
		if d.WorkspaceID == arg.WorkspaceID {
			out = append(out, d)
		}
	}
	start := min(int(arg.PageOffset), len(out))
	end := min(start+int(arg.PageLimit), len(out))
	return out[start:end], nil
}

func (f *fakeStore) ListLinksForDomain(context.Context, uuid.UUID) ([]store.ListLinksForDomainRow, error) {
	return f.links, nil
}

func (f *fakeStore) CountLinksForDomain(context.Context, uuid.UUID) (int64, error) {
	return int64(len(f.links)), nil
}

func (f *fakeStore) SetDefaultDomain(_ context.Context, id uuid.UUID) (store.Domain, error) {
	d := f.domains[id]
	d.IsDefault = true
	f.domains[id] = d
	return d, nil
}

func (f *fakeStore) UpdateDomainRootRedirect(_ context.Context, arg store.UpdateDomainRootRedirectParams) (store.Domain, error) {
	d, ok := f.domains[arg.ID]
	if !ok || d.WorkspaceID != arg.WorkspaceID {
		return store.Domain{}, pgx.ErrNoRows
	}
	d.RootRedirectUrl = arg.RootRedirectUrl
	f.domains[arg.ID] = d
	return d, nil
}

func (f *fakeStore) UpdateDomainVerification(_ context.Context, arg store.UpdateDomainVerificationParams) (store.Domain, error) {
	d, ok := f.domains[arg.ID]
	if !ok {
		return store.Domain{}, pgx.ErrNoRows
	}
	d.Status = arg.Status
	d.SslStatus = arg.SslStatus
	d.VerificationError = arg.VerificationError
	d.VerifiedAt = arg.VerifiedAt
	f.domains[d.ID] = d
	return d, nil
}

// newTestService wires a service with in-memory dependencies and no Traefik
// directory, so SyncTraefik is a no-op.
func newTestService(t *testing.T, res Resolver) (*Service, *fakeStore) {
	t.Helper()
	st := newFakeStore()
	svc := NewService(st, nil, config.Config{
		AppDomain:           testAppDomain,
		TraefikCertResolver: "letsencrypt",
	})
	svc.resolver = res
	return svc, st
}

// seedDomain inserts a pending domain owned by workspaceID.
func seedDomain(st *fakeStore, workspaceID uuid.UUID) store.Domain {
	d := store.Domain{
		ID:                 uuid.New(),
		WorkspaceID:        workspaceID,
		Hostname:           testHostname,
		Status:             StatusPending,
		VerificationToken:  testToken,
		VerificationMethod: MethodTXT,
		SslStatus:          SSLPending,
	}
	st.domains[d.ID] = d
	return d
}

func validTXT() map[string][]string {
	return map[string][]string{
		verificationLabel + "." + testHostname: {tokenPrefix + testToken},
	}
}

func TestVerify(t *testing.T) {
	tests := []struct {
		name          string
		resolver      fakeResolver
		wantStatus    string
		wantSSL       string
		wantVerified  bool
		wantContains  []string
		wantNoMessage bool
	}{
		{
			name: "ownership and routing both pass by shared address",
			resolver: fakeResolver{
				txt: validTXT(),
				hosts: map[string][]string{
					testHostname:  {"203.0.113.10"},
					testAppDomain: {"203.0.113.10"},
				},
			},
			wantStatus:    StatusActive,
			wantSSL:       SSLPending,
			wantVerified:  true,
			wantNoMessage: true,
		},
		{
			name: "ownership and routing both pass by CNAME",
			resolver: fakeResolver{
				txt:   validTXT(),
				cname: map[string]string{testHostname: testAppDomain},
				hosts: map[string][]string{testAppDomain: {"203.0.113.10"}},
			},
			wantStatus:    StatusActive,
			wantSSL:       SSLPending,
			wantVerified:  true,
			wantNoMessage: true,
		},
		{
			name: "CNAME chain ending at the app domain still passes",
			resolver: fakeResolver{
				txt: validTXT(),
				// The resolver flattens the chain, so an intermediate alias is
				// reported as the final canonical name.
				cname: map[string]string{testHostname: testAppDomain},
				hosts: map[string][]string{
					testHostname:  {"198.51.100.7"},
					testAppDomain: {"203.0.113.10"},
				},
			},
			wantStatus:   StatusActive,
			wantVerified: true,
		},
		{
			name: "ownership passes but DNS points elsewhere",
			resolver: fakeResolver{
				txt: validTXT(),
				hosts: map[string][]string{
					testHostname:  {"198.51.100.7"},
					testAppDomain: {"203.0.113.10"},
				},
			},
			wantStatus: StatusFailed,
			wantSSL:    SSLPending,
			wantContains: []string{
				testHostname,
				"does not point to this ShortURL installation",
				testAppDomain,
				"203.0.113.10",
				"198.51.100.7",
			},
		},
		{
			name: "hostname does not resolve at all",
			resolver: fakeResolver{
				txt:   validTXT(),
				hosts: map[string][]string{testAppDomain: {"203.0.113.10"}},
			},
			wantStatus: StatusFailed,
			wantContains: []string{
				"does not point to this ShortURL installation",
				"no address records",
			},
		},
		{
			name: "ownership fails when the TXT record is missing",
			resolver: fakeResolver{
				hosts: map[string][]string{
					testHostname:  {"203.0.113.10"},
					testAppDomain: {"203.0.113.10"},
				},
			},
			wantStatus: StatusFailed,
			wantContains: []string{
				verificationLabel + "." + testHostname,
				tokenPrefix + testToken,
			},
		},
		{
			name: "ownership fails when the TXT record holds a stale token",
			resolver: fakeResolver{
				txt: map[string][]string{
					verificationLabel + "." + testHostname: {tokenPrefix + "an-old-token"},
				},
				hosts: map[string][]string{
					testHostname:  {"203.0.113.10"},
					testAppDomain: {"203.0.113.10"},
				},
			},
			wantStatus: StatusFailed,
			wantContains: []string{
				"does not match",
				tokenPrefix + testToken,
				"an-old-token",
			},
		},
		{
			name:       "both checks fail and both are reported",
			resolver:   fakeResolver{hosts: map[string][]string{testAppDomain: {"203.0.113.10"}}},
			wantStatus: StatusFailed,
			wantContains: []string{
				"No TXT record could be read",
				"does not point to this ShortURL installation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, st := newTestService(t, tt.resolver)
			workspaceID := uuid.New()
			d := seedDomain(st, workspaceID)

			got, err := svc.Verify(context.Background(), workspaceID, d.ID)
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}

			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if tt.wantSSL != "" && got.SslStatus != tt.wantSSL {
				t.Errorf("ssl_status = %q, want %q", got.SslStatus, tt.wantSSL)
			}
			if tt.wantVerified && got.VerifiedAt == nil {
				t.Error("verified_at is nil, want a timestamp")
			}
			if !tt.wantVerified && got.VerifiedAt != nil {
				t.Errorf("verified_at = %v, want nil", got.VerifiedAt)
			}

			message := ""
			if got.VerificationError != nil {
				message = *got.VerificationError
			}
			if tt.wantStatus == StatusActive && got.VerificationError != nil {
				t.Errorf("verification_error = %q, want nil after success", message)
			}
			if tt.wantStatus == StatusFailed && message == "" {
				t.Fatal("verification_error is empty, want an explanation")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(message, want) {
					t.Errorf("verification_error = %q, want it to contain %q", message, want)
				}
			}
		})
	}
}

func TestVerifyUnknownDomainIsNotFound(t *testing.T) {
	svc, _ := newTestService(t, fakeResolver{})

	_, err := svc.Verify(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("Verify() error = %v, want ErrNotFound", err)
	}
}

func TestVerifyForeignWorkspaceIsNotFound(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	d := seedDomain(st, uuid.New())

	if _, err := svc.Verify(context.Background(), uuid.New(), d.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("Verify() error = %v, want ErrNotFound", err)
	}
}

func TestAdd(t *testing.T) {
	t.Run("first domain becomes the workspace default", func(t *testing.T) {
		svc, _ := newTestService(t, fakeResolver{})
		workspaceID := uuid.New()

		first, err := svc.Add(context.Background(), workspaceID, "GO.Example.com:443")
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if first.Hostname != testHostname {
			t.Errorf("hostname = %q, want %q", first.Hostname, testHostname)
		}
		if !first.IsDefault {
			t.Error("first domain is not default, want default")
		}
		if first.VerificationToken == "" {
			t.Error("verification_token is empty")
		}
		if first.VerificationMethod != MethodTXT {
			t.Errorf("verification_method = %q, want %q", first.VerificationMethod, MethodTXT)
		}

		second, err := svc.Add(context.Background(), workspaceID, "link.example.org")
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if second.IsDefault {
			t.Error("second domain is default, want not default")
		}
		if second.VerificationToken == first.VerificationToken {
			t.Error("both domains share a verification token, want distinct tokens")
		}
	})

	t.Run("the application domain is reserved", func(t *testing.T) {
		svc, _ := newTestService(t, fakeResolver{})

		_, err := svc.Add(context.Background(), uuid.New(), "SHORT.example.com")
		assertAPIError(t, err, 409, "reserved_hostname")
	})

	t.Run("a hostname owned elsewhere conflicts without naming the owner", func(t *testing.T) {
		svc, st := newTestService(t, fakeResolver{})
		st.createErr = &pgconn.PgError{Code: "23505", ConstraintName: "domains_hostname_unique"}

		_, err := svc.Add(context.Background(), uuid.New(), testHostname)
		assertAPIError(t, err, 409, "hostname_taken")

		var apiErr *httpx.APIError
		if errors.As(err, &apiErr) && strings.Contains(apiErr.Message, testHostname) {
			t.Errorf("message %q names the hostname, which leaks tenant information", apiErr.Message)
		}
	})

	t.Run("an unexpected database error stays internal", func(t *testing.T) {
		svc, st := newTestService(t, fakeResolver{})
		st.createErr = errors.New("connection reset")

		_, err := svc.Add(context.Background(), uuid.New(), testHostname)
		assertAPIError(t, err, 500, "internal_error")
	})
}

func TestAddHostnameValidation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // normalised hostname, empty means the input must be rejected
	}{
		{name: "plain hostname", input: "go.example.com", want: "go.example.com"},
		{name: "uppercase is normalised", input: "GO.EXAMPLE.COM", want: "go.example.com"},
		{name: "surrounding space is trimmed", input: "  go.example.com  ", want: "go.example.com"},
		{name: "port is stripped", input: "go.example.com:8443", want: "go.example.com"},
		{name: "trailing dot is dropped", input: "go.example.com.", want: "go.example.com"},
		{name: "hyphens inside labels are allowed", input: "go-links.example.com", want: "go-links.example.com"},
		{name: "deep subdomain", input: "a.b.c.example.com", want: "a.b.c.example.com"},
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "no dot", input: "localhost", want: ""},
		{name: "ipv4 address", input: "203.0.113.10", want: ""},
		{name: "ipv6 address", input: "2001:db8::1", want: ""},
		{name: "empty label", input: "go..example.com", want: ""},
		{name: "leading hyphen", input: "-go.example.com", want: ""},
		{name: "trailing hyphen", input: "go-.example.com", want: ""},
		{name: "underscore", input: "go_links.example.com", want: ""},
		{name: "scheme included", input: "https://go.example.com", want: ""},
		{name: "path included", input: "go.example.com/promo", want: ""},
		{name: "label over 63 characters", input: strings.Repeat("a", 64) + ".example.com", want: ""},
		{name: "over 253 characters", input: strings.Repeat("a.", 130) + "example.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t, fakeResolver{})

			d, err := svc.Add(context.Background(), uuid.New(), tt.input)
			if tt.want == "" {
				assertAPIError(t, err, 422, "validation_error")
				return
			}
			if err != nil {
				t.Fatalf("Add(%q) error = %v", tt.input, err)
			}
			if d.Hostname != tt.want {
				t.Errorf("hostname = %q, want %q", d.Hostname, tt.want)
			}
		})
	}
}

func TestDeleteRefusesADomainWithLinks(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	workspaceID := uuid.New()
	d := seedDomain(st, workspaceID)
	st.links = []store.ListLinksForDomainRow{{Slug: "promo", Hostname: testHostname}}

	err := svc.Delete(context.Background(), workspaceID, d.ID)
	assertAPIError(t, err, 409, "domain_has_links")

	if _, still := st.domains[d.ID]; !still {
		t.Error("domain was deleted despite carrying links")
	}
}

func TestDeleteRemovesADomainWithoutLinks(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	workspaceID := uuid.New()
	d := seedDomain(st, workspaceID)

	if err := svc.Delete(context.Background(), workspaceID, d.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, still := st.domains[d.ID]; still {
		t.Error("domain still present after Delete")
	}
}

func TestSetDefaultRequiresAnActiveDomain(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	workspaceID := uuid.New()
	d := seedDomain(st, workspaceID)

	_, err := svc.SetDefault(context.Background(), workspaceID, d.ID)
	assertAPIError(t, err, 409, "domain_not_active")
}

func TestInstructions(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	d := seedDomain(st, uuid.New())

	got := svc.Instructions(d)

	if got.Verification.Type != "TXT" {
		t.Errorf("verification type = %q, want TXT", got.Verification.Type)
	}
	if want := "_shorturl-verification." + testHostname; got.Verification.Name != want {
		t.Errorf("verification name = %q, want %q", got.Verification.Name, want)
	}
	if want := "shorturl-verification=" + testToken; got.Verification.Value != want {
		t.Errorf("verification value = %q, want %q", got.Verification.Value, want)
	}

	if len(got.Routing) != 2 {
		t.Fatalf("routing records = %d, want 2 so the UI can offer a choice", len(got.Routing))
	}
	cname := got.Routing[0]
	if cname.Type != "CNAME" || cname.Name != testHostname || cname.Value != testAppDomain {
		t.Errorf("cname record = %+v, want CNAME %s -> %s", cname, testHostname, testAppDomain)
	}
	if got.Routing[1].Type != "A" {
		t.Errorf("second routing record type = %q, want A", got.Routing[1].Type)
	}
	if !strings.Contains(got.Routing[1].Value, testAppDomain) {
		t.Errorf("A record note = %q, want it to reference %s", got.Routing[1].Value, testAppDomain)
	}
}

func assertAPIError(t *testing.T, err error, status int, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error with code %q, got nil", code)
	}
	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not an *httpx.APIError", err)
	}
	if apiErr.Status != status || apiErr.Code != code {
		t.Fatalf("error = %d/%s, want %d/%s", apiErr.Status, apiErr.Code, status, code)
	}
}

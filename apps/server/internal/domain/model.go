package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// DNSRecord is one record the operator has to create at their DNS provider.
type DNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DNSInstructions is everything the dashboard needs to render the "Expected
// DNS" panel described in §39 of the plan: one ownership record, plus the
// routing records the user can choose between.
type DNSInstructions struct {
	Verification DNSRecord   `json:"verification"`
	Routing      []DNSRecord `json:"routing"`
}

// addRequest is the body of POST /domains.
type addRequest struct {
	Hostname string `json:"hostname"`
}

type updateRootRedirectRequest struct {
	RootRedirectURL *string `json:"root_redirect_url"`
}

// domainResponse is the wire shape of a domain.
//
// DNSInstructions is omitted from list responses because it carries the
// verification token, which only the person configuring DNS for one specific
// domain needs to see.
type domainResponse struct {
	ID                 uuid.UUID        `json:"id"`
	Hostname           string           `json:"hostname"`
	Status             string           `json:"status"`
	SSLStatus          string           `json:"ssl_status"`
	IsDefault          bool             `json:"is_default"`
	RootRedirectURL    *string          `json:"root_redirect_url"`
	VerificationMethod string           `json:"verification_method"`
	VerificationError  *string          `json:"verification_error"`
	VerifiedAt         *time.Time       `json:"verified_at"`
	CreatedAt          time.Time        `json:"created_at"`
	DNSInstructions    *DNSInstructions `json:"dns_instructions,omitempty"`
}

func newDomainResponse(d store.Domain) domainResponse {
	return domainResponse{
		ID:                 d.ID,
		Hostname:           d.Hostname,
		Status:             d.Status,
		SSLStatus:          d.SslStatus,
		IsDefault:          d.IsDefault,
		RootRedirectURL:    d.RootRedirectUrl,
		VerificationMethod: d.VerificationMethod,
		VerificationError:  d.VerificationError,
		VerifiedAt:         d.VerifiedAt,
		CreatedAt:          d.CreatedAt,
	}
}

// withInstructions attaches the DNS setup instructions to a single-domain
// response.
func (r domainResponse) withInstructions(in DNSInstructions) domainResponse {
	r.DNSInstructions = &in
	return r
}

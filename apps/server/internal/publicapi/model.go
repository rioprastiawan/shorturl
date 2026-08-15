package publicapi

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/httpx"
	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
)

// CreatedVia marks links that arrived through the machine-to-machine API, so
// the dashboard can show where a link came from.
const CreatedVia = "api"

// createRequest is the body of POST /links (plan §14.1, §14.2).
type createRequest struct {
	DestinationURL    string          `json:"destination_url"`
	Domain            *string         `json:"domain"`
	DomainID          *uuid.UUID      `json:"domain_id"`
	Slug              *string         `json:"slug"`
	Title             *string         `json:"title"`
	RedirectType      *int16          `json:"redirect_type"`
	Password          *string         `json:"password"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	MaxClicks         *int64          `json:"max_clicks"`
	ExternalReference *string         `json:"external_reference"`
	Metadata          json.RawMessage `json:"metadata"`
}

// toInput binds the request to the workspace the API key belongs to. The
// workspace is never read from the body — an integration cannot address
// another tenant by guessing an ID.
func (r createRequest) toInput(workspaceID uuid.UUID) link.CreateInput {
	return link.CreateInput{
		WorkspaceID:       workspaceID,
		DomainID:          r.DomainID,
		Hostname:          r.Domain,
		DestinationURL:    r.DestinationURL,
		Slug:              r.Slug,
		Title:             r.Title,
		RedirectType:      r.RedirectType,
		Password:          r.Password,
		ExpiresAt:         r.ExpiresAt,
		MaxClicks:         r.MaxClicks,
		ExternalReference: r.ExternalReference,
		Metadata:          r.Metadata,
		CreatedVia:        CreatedVia,
	}
}

// updateRequest is the body of PATCH /links/{linkId} (plan §14.6).
//
// Nullable fields arrive as *json.RawMessage so an absent key ("leave it
// alone") is distinguishable from an explicit null ("clear it"). A plain
// pointer collapses those two into one, and an integration could then never
// remove an expiry it had previously set.
type updateRequest struct {
	DestinationURL *string          `json:"destination_url"`
	Status         *string          `json:"status"`
	RedirectType   *int16           `json:"redirect_type"`
	Slug           *string          `json:"slug"`
	Domain         *string          `json:"domain"`
	DomainID       *uuid.UUID       `json:"domain_id"`
	Title          *json.RawMessage `json:"title"`
	ExpiresAt      *json.RawMessage `json:"expires_at"`
	MaxClicks      *json.RawMessage `json:"max_clicks"`
	Password       *json.RawMessage `json:"password"`
	Metadata       *json.RawMessage `json:"metadata"`
}

func (r updateRequest) toInput() (link.UpdateInput, error) {
	in := link.UpdateInput{
		DestinationURL: r.DestinationURL,
		Status:         r.Status,
		RedirectType:   r.RedirectType,
		Slug:           r.Slug,
		Hostname:       r.Domain,
		DomainID:       r.DomainID,
	}

	if r.Title != nil {
		in.SetTitle = true
		if !isJSONNull(*r.Title) {
			var v string
			if err := json.Unmarshal(*r.Title, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"title": {"must be a string or null"}})
			}
			in.Title = &v
		}
	}
	if r.ExpiresAt != nil {
		in.SetExpiresAt = true
		if !isJSONNull(*r.ExpiresAt) {
			var v time.Time
			if err := json.Unmarshal(*r.ExpiresAt, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"expires_at": {"must be an RFC3339 timestamp or null"}})
			}
			in.ExpiresAt = &v
		}
	}
	if r.MaxClicks != nil {
		in.SetMaxClicks = true
		if !isJSONNull(*r.MaxClicks) {
			var v int64
			if err := json.Unmarshal(*r.MaxClicks, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"max_clicks": {"must be a number or null"}})
			}
			in.MaxClicks = &v
		}
	}
	if r.Password != nil {
		in.SetPassword = true
		if !isJSONNull(*r.Password) {
			var v string
			if err := json.Unmarshal(*r.Password, &v); err != nil {
				return in, httpx.Invalid(map[string][]string{"password": {"must be a string or null"}})
			}
			in.Password = &v
		}
	}
	if r.Metadata != nil {
		in.SetMetadata = true
		if !isJSONNull(*r.Metadata) {
			in.Metadata = *r.Metadata
		}
	}

	return in, nil
}

func isJSONNull(raw json.RawMessage) bool { return string(raw) == "null" }

// Package link owns short-link creation, updates, and the cache entries the
// redirect engine reads.
package link

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// View is a link joined with its hostname, which is what every caller
// actually wants: a link is meaningless without the domain it lives on.
type View struct {
	Link           store.Link
	Hostname       string
	CreatedByName  *string
	CreatedByEmail *string
}

// ShortURL is the value integrations care about most.
func (v View) ShortURL() string {
	return "https://" + v.Hostname + "/" + v.Link.Slug
}

// DTO is the JSON representation returned by both the dashboard and the
// public API, so an integration and the UI never see different shapes.
type DTO struct {
	ID                uuid.UUID       `json:"id"`
	ShortURL          string          `json:"short_url"`
	Slug              string          `json:"slug"`
	Domain            string          `json:"domain"`
	DomainID          uuid.UUID       `json:"domain_id"`
	DestinationURL    string          `json:"destination_url"`
	Title             *string         `json:"title"`
	Status            string          `json:"status"`
	RedirectType      int16           `json:"redirect_type"`
	HasPassword       bool            `json:"has_password"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	MaxClicks         *int64          `json:"max_clicks"`
	ClickCount        int64           `json:"click_count"`
	ExternalReference *string         `json:"external_reference"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedVia        string          `json:"created_via"`
	CreatedByName     *string         `json:"created_by_name"`
	CreatedByEmail    *string         `json:"created_by_email"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// ToDTO renders a view for the API.
func ToDTO(v View) DTO {
	d := DTO{
		ID:                v.Link.ID,
		ShortURL:          v.ShortURL(),
		Slug:              v.Link.Slug,
		Domain:            v.Hostname,
		DomainID:          v.Link.DomainID,
		DestinationURL:    v.Link.DestinationUrl,
		Title:             v.Link.Title,
		Status:            v.Link.Status,
		RedirectType:      v.Link.RedirectType,
		HasPassword:       v.Link.PasswordHash != nil && *v.Link.PasswordHash != "",
		ExpiresAt:         v.Link.ExpiresAt,
		MaxClicks:         v.Link.MaxClicks,
		ClickCount:        v.Link.ClickCount,
		ExternalReference: v.Link.ExternalReference,
		CreatedVia:        v.Link.CreatedVia,
		CreatedByName:     v.CreatedByName,
		CreatedByEmail:    v.CreatedByEmail,
		CreatedAt:         v.Link.CreatedAt,
		UpdatedAt:         v.Link.UpdatedAt,
	}
	if len(v.Link.Metadata) > 0 {
		d.Metadata = json.RawMessage(v.Link.Metadata)
	}
	return d
}

// CreateInput describes a new link. Pointer fields are optional; nil means
// "not supplied", which is different from "set to empty".
type CreateInput struct {
	WorkspaceID uuid.UUID

	// Exactly one of DomainID or Hostname may be set. Both nil selects the
	// workspace default domain, which is what the public API relies on when a
	// caller sends only destination_url.
	DomainID *uuid.UUID
	Hostname *string

	DestinationURL    string
	Slug              *string
	Title             *string
	RedirectType      *int16
	Password          *string
	ExpiresAt         *time.Time
	MaxClicks         *int64
	ExternalReference *string
	Metadata          json.RawMessage

	CreatedBy  *uuid.UUID
	CreatedVia string
}

// UpdateInput describes a partial update. A nil pointer leaves the field
// alone; the Set* flags allow explicitly clearing a nullable column, which a
// nil pointer alone cannot express.
type UpdateInput struct {
	DestinationURL *string
	Status         *string
	RedirectType   *int16
	Slug           *string
	DomainID       *uuid.UUID
	Hostname       *string

	Title        *string
	SetTitle     bool
	ExpiresAt    *time.Time
	SetExpiresAt bool
	MaxClicks    *int64
	SetMaxClicks bool
	Password     *string
	SetPassword  bool
	Metadata     json.RawMessage
	SetMetadata  bool
}

// ListInput drives the filtered, cursor-paginated list query.
type ListInput struct {
	WorkspaceID       uuid.UUID
	Search            string
	Tag               string
	Status            string
	DomainID          *uuid.UUID
	ExternalReference string
	CreatedAfter      *time.Time
	CreatedBefore     *time.Time
	Limit             int
	Cursor            string
}

// MaxBulkLinks bounds a single bulk-create request. It's small enough that a
// batch finishes well inside the route's timeout even on a slow database.
const MaxBulkLinks = 1000

// CreatedViaImport marks links created through the CSV bulk importer, so
// Create skips the live metadata fetch it does for interactive dashboard
// submissions (plan §14.2's "dashboard" fallback isn't useful for
// historical data, and would be the dominant per-row cost otherwise).
const CreatedViaImport = "import"

// BulkResult reports per-row outcomes for a bulk-create request: unlike a
// single create, one bad row must not fail the rest of the batch.
type BulkResult struct {
	Created []DTO         `json:"created"`
	Failed  []BulkFailure `json:"failed"`
}

// BulkFailure ties a failure back to its position in the request payload so
// the caller can map it back to, e.g., a CSV line number.
type BulkFailure struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AuditEntry struct {
	ID          uuid.UUID       `json:"id"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    *uuid.UUID      `json:"entity_id"`
	EntityLabel *string         `json:"entity_label"`
	ActorName   *string         `json:"actor_name"`
	ActorEmail  *string         `json:"actor_email"`
	Details     json.RawMessage `json:"details"`
	CreatedAt   time.Time       `json:"created_at"`
}

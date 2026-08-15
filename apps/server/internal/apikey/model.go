package apikey

import (
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

// keyShownOnce is what the dashboard must put in front of the user the moment
// a key is created. There is no second chance: only the digest is stored.
const keyShownOnce = "Copy this key now. It is stored only as a hash and will never be shown again."

// DTO is the safe representation of a key. The hash is deliberately absent —
// it is not secret in the way the key is, but it has no reason to leave the
// database either.
type DTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ToDTO renders a stored key for the dashboard.
func ToDTO(k store.ApiKey) DTO {
	scopes := k.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return DTO{
		ID:         k.ID,
		Name:       k.Name,
		KeyPrefix:  k.KeyPrefix,
		Scopes:     scopes,
		LastUsedAt: k.LastUsedAt,
		ExpiresAt:  k.ExpiresAt,
		RevokedAt:  k.RevokedAt,
		CreatedAt:  k.CreatedAt,
	}
}

// createdDTO carries the one-time plaintext alongside the stored fields.
type createdDTO struct {
	DTO
	Key     string `json:"key"`
	Warning string `json:"warning"`
}

func newCreatedDTO(k store.ApiKey, plaintext string) createdDTO {
	return createdDTO{DTO: ToDTO(k), Key: plaintext, Warning: keyShownOnce}
}

type createRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`

	// Test issues a shr_test_ key instead of shr_live_. Nothing behaves
	// differently server-side; the prefix exists so an integration can tell
	// which environment a call came from in its own logs.
	Test bool `json:"test"`
}

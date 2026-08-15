package auth

import (
	"time"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

// Name length bounds. The upper bound matches users.name VARCHAR(120), so an
// over-long name is rejected with a field error instead of a database error.
const (
	NameMinLength = 2
	NameMaxLength = 120
)

// UserDTO is the only shape a user is serialised in.
//
// It is a separate struct rather than store.User with json tags because
// store.User carries PasswordHash: copying the four public fields across means
// a hash can never leak, not even if the generated model gains a field.
type UserDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserDTO projects a stored user onto the public shape.
func NewUserDTO(u *store.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		IsAdmin:   u.IsAdmin,
		Language:  u.Language,
		Timezone:  u.Timezone,
		CreatedAt: u.CreatedAt,
	}
}

// ValidateAccount checks the account fields shared by registration and the
// first-run setup wizard, returning the trimmed name and lowercased email so
// callers store the normalised values.
func ValidateAccount(v *validate.Errors, name, email, password string) (string, string) {
	trimmedName := v.Required("name", name)
	if trimmedName != "" {
		v.Length("name", trimmedName, NameMinLength, NameMaxLength)
	}
	normalisedEmail := v.Email("email", email)
	v.Password("password", password)
	return trimmedName, normalisedEmail
}

type registerRequest struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	InvitationToken string `json:"invitation_token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type twoFactorSetupRequest struct {
	Password string `json:"password"`
}
type twoFactorCodeRequest struct {
	Code string `json:"code"`
}
type twoFactorDisableRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}
type twoFactorStatusDTO struct {
	Enabled bool `json:"enabled"`
}
type twoFactorSetupDTO struct {
	Secret        string   `json:"secret"`
	URI           string   `json:"uri"`
	RecoveryCodes []string `json:"recovery_codes"`
}

type updateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type updatePreferencesRequest struct {
	Language string `json:"language"`
	Timezone string `json:"timezone"`
}

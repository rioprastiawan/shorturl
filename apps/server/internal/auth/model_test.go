package auth

import (
	"strings"
	"testing"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/validate"
)

func TestValidateAccount(t *testing.T) {
	const goodPassword = "correct-horse-battery"

	tests := []struct {
		name       string
		inName     string
		inEmail    string
		inPassword string
		wantName   string
		wantEmail  string
		wantFields []string
	}{
		{
			name: "valid", inName: "Ada Lovelace", inEmail: "ada@example.com", inPassword: goodPassword,
			wantName: "Ada Lovelace", wantEmail: "ada@example.com",
		},
		{
			name: "trims and lowercases", inName: "  Ada  ", inEmail: "  Ada@Example.COM ", inPassword: goodPassword,
			wantName: "Ada", wantEmail: "ada@example.com",
		},
		{
			name: "missing name", inName: "", inEmail: "ada@example.com", inPassword: goodPassword,
			wantEmail: "ada@example.com", wantFields: []string{"name"},
		},
		{
			name: "whitespace-only name", inName: "   ", inEmail: "ada@example.com", inPassword: goodPassword,
			wantEmail: "ada@example.com", wantFields: []string{"name"},
		},
		{
			name: "name too short", inName: "A", inEmail: "ada@example.com", inPassword: goodPassword,
			wantName: "A", wantEmail: "ada@example.com", wantFields: []string{"name"},
		},
		{
			name: "name too long", inName: strings.Repeat("a", NameMaxLength+1), inEmail: "ada@example.com", inPassword: goodPassword,
			wantName: strings.Repeat("a", NameMaxLength+1), wantEmail: "ada@example.com", wantFields: []string{"name"},
		},
		{
			name: "malformed email", inName: "Ada", inEmail: "not-an-email", inPassword: goodPassword,
			wantName: "Ada", wantEmail: "not-an-email", wantFields: []string{"email"},
		},
		{
			name: "missing email", inName: "Ada", inEmail: "", inPassword: goodPassword,
			wantName: "Ada", wantFields: []string{"email"},
		},
		{
			name: "password too short", inName: "Ada", inEmail: "ada@example.com", inPassword: "short",
			wantName: "Ada", wantEmail: "ada@example.com", wantFields: []string{"password"},
		},
		{
			name: "everything wrong", inName: "", inEmail: "nope", inPassword: "",
			wantEmail: "nope", wantFields: []string{"name", "email", "password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := validate.New()
			gotName, gotEmail := ValidateAccount(v, tt.inName, tt.inEmail, tt.inPassword)

			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
			if gotEmail != tt.wantEmail {
				t.Errorf("email = %q, want %q", gotEmail, tt.wantEmail)
			}

			fields := v.Fields()
			if len(fields) != len(tt.wantFields) {
				t.Fatalf("fields = %v, want errors on exactly %v", fields, tt.wantFields)
			}
			for _, field := range tt.wantFields {
				if len(fields[field]) == 0 {
					t.Errorf("fields = %v, want an error on %q", fields, field)
				}
			}
		})
	}
}

// TestUserDTOOmitsPasswordHash is a regression guard: the DTO exists so a hash
// can never reach a response body.
func TestUserDTOOmitsPasswordHash(t *testing.T) {
	user := store.User{
		Name:         "Ada",
		Email:        "ada@example.com",
		PasswordHash: "$argon2id$super-secret",
	}

	dto := NewUserDTO(&user)
	if dto.Name != user.Name || dto.Email != user.Email {
		t.Fatalf("NewUserDTO dropped public fields: %+v", dto)
	}

	encoded := mustJSON(t, dto)
	if strings.Contains(encoded, "argon2id") || strings.Contains(encoded, "password") {
		t.Errorf("UserDTO JSON = %s, want no password material", encoded)
	}
}

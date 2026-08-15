package security

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q, want an argon2id PHC string", hash)
	}
	if strings.Contains(hash, password) {
		t.Fatal("hash contains the plaintext password")
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !ok {
		t.Error("VerifyPassword() = false for the correct password")
	}

	ok, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if ok {
		t.Error("VerifyPassword() = true for an incorrect password")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, err := HashPassword("same input")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same input")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical, so the salt is not random")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for name, hash := range map[string]string{
		"empty":          "",
		"plain text":     "hunter2",
		"wrong algo":     "$bcrypt$v=19$m=65536,t=2,p=4$c2FsdA$aGFzaA",
		"missing fields": "$argon2id$v=19$m=65536",
		"bad base64":     "$argon2id$v=19$m=65536,t=2,p=4$!!!$!!!",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyPassword("anything", hash); err == nil {
				t.Fatal("want an error for a malformed hash, got nil")
			}
		})
	}
}

func TestNewTokenIsRandomAndURLSafe(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		token, err := NewToken(32)
		if err != nil {
			t.Fatalf("NewToken() error = %v", err)
		}
		if seen[token] {
			t.Fatal("NewToken() returned a duplicate within 100 calls")
		}
		seen[token] = true

		if strings.ContainsAny(token, "+/=") {
			t.Errorf("token %q contains characters that need URL escaping", token)
		}
	}
}

func TestHashTokenIsStable(t *testing.T) {
	a := HashToken("shr_live_abc123")
	b := HashToken("shr_live_abc123")
	if a != b {
		t.Error("HashToken() is not deterministic")
	}
	if a == HashToken("shr_live_abc124") {
		t.Error("HashToken() collided on different inputs")
	}
	if len(a) != 64 {
		t.Errorf("HashToken() length = %d, want 64 hex characters", len(a))
	}
}

func TestHashIP(t *testing.T) {
	secret := []byte("test-secret")

	t.Run("stable per address", func(t *testing.T) {
		if HashIP(secret, "203.0.113.5") != HashIP(secret, "203.0.113.5") {
			t.Error("the same address hashed to different values")
		}
	})

	t.Run("port is ignored", func(t *testing.T) {
		if HashIP(secret, "203.0.113.5") != HashIP(secret, "203.0.113.5:44321") {
			t.Error("the port changed the hash; it must be stripped first")
		}
	})

	t.Run("different addresses differ", func(t *testing.T) {
		if HashIP(secret, "203.0.113.5") == HashIP(secret, "203.0.113.6") {
			t.Error("distinct addresses collided")
		}
	})

	t.Run("secret changes the output", func(t *testing.T) {
		if HashIP(secret, "203.0.113.5") == HashIP([]byte("other"), "203.0.113.5") {
			t.Error("the secret did not affect the hash, so it is not keyed")
		}
	})

	t.Run("unparseable input yields empty", func(t *testing.T) {
		for _, bad := range []string{"", "not-an-ip", "999.999.999.999"} {
			if got := HashIP(secret, bad); got != "" {
				t.Errorf("HashIP(%q) = %q, want empty", bad, got)
			}
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		if HashIP(secret, "2001:db8::1") == "" {
			t.Error("an IPv6 address was rejected")
		}
	})
}

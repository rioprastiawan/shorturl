package security

import (
	"testing"
	"time"
)

func TestValidateTOTPRFC6238Vector(t *testing.T) {
	// RFC 6238 SHA-1 seed "12345678901234567890" at Unix time 59,
	// truncated to the six digits used by authenticator apps.
	if !ValidateTOTP("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "287082", time.Unix(59, 0)) {
		t.Fatal("known TOTP vector was rejected")
	}
	if ValidateTOTP("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "000000", time.Unix(59, 0)) {
		t.Fatal("incorrect TOTP was accepted")
	}
}

func TestSecretEncryptionRoundTrip(t *testing.T) {
	key := []byte("test session secret")
	ciphertext, err := EncryptSecret(key, "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptSecret(key, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("got %q", plain)
	}
}

func TestRecoveryCodesAreHashed(t *testing.T) {
	code, err := RecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	if code == "" || HashRecoveryCode(code) == code {
		t.Fatal("recovery code was not generated and hashed")
	}
	if HashRecoveryCode(code) != HashRecoveryCode(code) {
		t.Fatal("hash is not stable")
	}
}

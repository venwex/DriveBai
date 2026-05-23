package auth_test

import (
	"strings"
	"testing"

	"github.com/drivebai/backend/internal/auth"
)

// TestHashPassword_ProducesValidBcryptHash verifies the hash looks like bcrypt output.
func TestHashPassword_ProducesValidBcryptHash(t *testing.T) {
	hash, err := auth.HashPassword("s3cureP@ss!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("expected bcrypt hash prefix, got: %q", hash[:min(10, len(hash))])
	}
}

// TestHashPassword_DifferentSalts ensures two hashes of the same password differ.
func TestHashPassword_DifferentSalts(t *testing.T) {
	h1, _ := auth.HashPassword("same-password")
	h2, _ := auth.HashPassword("same-password")
	if h1 == h2 {
		t.Error("expected different hashes due to random salt, got identical hashes")
	}
}

// TestCheckPassword_Correct verifies that the correct plaintext matches its hash.
func TestCheckPassword_Correct(t *testing.T) {
	password := "MyP@ssw0rd"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !auth.CheckPassword(password, hash) {
		t.Error("CheckPassword: expected true for correct password, got false")
	}
}

// TestCheckPassword_Wrong verifies that an incorrect password does not match.
func TestCheckPassword_Wrong(t *testing.T) {
	hash, _ := auth.HashPassword("correct-horse-battery-staple")
	if auth.CheckPassword("wrong-password", hash) {
		t.Error("CheckPassword: expected false for wrong password, got true")
	}
}

// TestCheckPassword_EmptyPassword verifies that an empty password is rejected.
func TestCheckPassword_EmptyPassword(t *testing.T) {
	hash, _ := auth.HashPassword("some-real-password")
	if auth.CheckPassword("", hash) {
		t.Error("CheckPassword: expected false for empty password, got true")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

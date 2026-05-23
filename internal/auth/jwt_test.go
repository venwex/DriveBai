package auth_test

import (
	"testing"
	"time"

	"github.com/drivebai/backend/internal/auth"
	"github.com/drivebai/backend/internal/models"
	"github.com/google/uuid"
)

func newTestJWT() *auth.JWTService {
	return auth.NewJWTService("test-secret-32-chars-long-enough!", 15*time.Minute, 30*24*time.Hour)
}

func testUser(role models.Role) *models.User {
	return &models.User{
		ID:    uuid.New(),
		Email: "test@example.com",
		Role:  role,
	}
}

// TestAccessToken_RoundTrip verifies that a generated token decodes to the same claims.
func TestAccessToken_RoundTrip(t *testing.T) {
	svc := newTestJWT()
	user := testUser(models.RoleDriver)

	tokenStr, expiresAt, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}
	if expiresAt.IsZero() {
		t.Fatal("expected non-zero expiry")
	}

	claims, err := svc.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("user_id mismatch: got %v want %v", claims.UserID, user.ID)
	}
	if claims.Email != user.Email {
		t.Errorf("email mismatch: got %q want %q", claims.Email, user.Email)
	}
	if claims.Role != user.Role {
		t.Errorf("role mismatch: got %q want %q", claims.Role, user.Role)
	}
}

// TestAccessToken_Roles verifies that role is preserved for every role value.
func TestAccessToken_Roles(t *testing.T) {
	svc := newTestJWT()

	roles := []models.Role{models.RoleDriver, models.RoleCarOwner, models.RoleAdmin}
	for _, role := range roles {
		user := testUser(role)
		token, _, err := svc.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("role %s: GenerateAccessToken: %v", role, err)
		}
		claims, err := svc.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("role %s: ValidateAccessToken: %v", role, err)
		}
		if claims.Role != role {
			t.Errorf("role %s: expected %q got %q", role, role, claims.Role)
		}
	}
}

// TestAccessToken_Expired verifies that a token with a past TTL returns ErrTokenExpired.
func TestAccessToken_Expired(t *testing.T) {
	// Negative TTL → token expired before it is even validated.
	svc := auth.NewJWTService("test-secret-32-chars-long-enough!", -1*time.Second, 30*24*time.Hour)
	user := testUser(models.RoleDriver)

	token, _, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = svc.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if err != models.ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

// TestAccessToken_WrongSecret verifies that a token signed with a different key is rejected.
func TestAccessToken_WrongSecret(t *testing.T) {
	svcA := auth.NewJWTService("secret-A-32-chars-long-enough!!!", 15*time.Minute, 30*24*time.Hour)
	svcB := auth.NewJWTService("secret-B-32-chars-long-enough!!!", 15*time.Minute, 30*24*time.Hour)

	user := testUser(models.RoleDriver)
	token, _, err := svcA.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = svcB.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for wrong-secret token, got nil")
	}
	if err != models.ErrTokenInvalid {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

// TestAccessToken_Tampered verifies that modifying the token string invalidates it.
func TestAccessToken_Tampered(t *testing.T) {
	svc := newTestJWT()
	user := testUser(models.RoleAdmin)

	token, _, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	// Flip the last character to simulate tampering.
	tampered := token[:len(token)-1] + "X"
	_, err = svc.ValidateAccessToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

// TestRefreshToken_HashIsUnique verifies that two refresh tokens hash to different values.
func TestRefreshToken_HashIsUnique(t *testing.T) {
	svc := newTestJWT()

	raw1, hash1, _, _ := svc.GenerateRefreshToken()
	raw2, hash2, _, _ := svc.GenerateRefreshToken()

	if raw1 == raw2 {
		t.Error("two refresh tokens should not be equal")
	}
	if hash1 == hash2 {
		t.Error("two refresh token hashes should not be equal")
	}
	if svc.HashRefreshToken(raw1) != hash1 {
		t.Error("HashRefreshToken should reproduce the stored hash")
	}
}

// TestRegistrationToken_RoundTrip verifies OTP-based registration token flow.
func TestRegistrationToken_RoundTrip(t *testing.T) {
	svc := newTestJWT()
	email := "register@example.com"

	token, err := svc.GenerateRegistrationToken(email)
	if err != nil {
		t.Fatalf("GenerateRegistrationToken: %v", err)
	}

	got, err := svc.ValidateRegistrationToken(token)
	if err != nil {
		t.Fatalf("ValidateRegistrationToken: %v", err)
	}
	if got != email {
		t.Errorf("email mismatch: got %q want %q", got, email)
	}
}

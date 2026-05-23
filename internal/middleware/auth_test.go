package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/drivebai/backend/internal/auth"
	"github.com/drivebai/backend/internal/httputil"
	"github.com/drivebai/backend/internal/middleware"
	"github.com/drivebai/backend/internal/models"
	"github.com/google/uuid"
)

func newJWT() *auth.JWTService {
	return auth.NewJWTService("test-secret-32-chars-long-enough!", 15*time.Minute, 30*24*time.Hour)
}

func validToken(svc *auth.JWTService, role models.Role) string {
	user := &models.User{ID: uuid.New(), Email: "u@example.com", Role: role}
	tok, _, _ := svc.GenerateAccessToken(user)
	return tok
}

// sentinelHandler is a terminal handler that records whether it was reached.
type sentinelHandler struct{ called bool }

func (h *sentinelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

func errorCode(body []byte) string {
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.Error.Code
}

// --- AuthMiddleware tests ---

func TestAuthMiddleware_NoHeader(t *testing.T) {
	svc := newJWT()
	sentinel := &sentinelHandler{}
	handler := middleware.AuthMiddleware(svc)(sentinel)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if sentinel.called {
		t.Error("next handler must not be called when auth header is missing")
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	svc := newJWT()
	handler := middleware.AuthMiddleware(svc)(&sentinelHandler{})

	for _, hdr := range []string{"Token abc", "Bearer", "basic abc"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", hdr)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("header %q: expected 401, got %d", hdr, rr.Code)
		}
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	svc := newJWT()
	handler := middleware.AuthMiddleware(svc)(&sentinelHandler{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	// Service with -1s TTL → token expires immediately.
	expiredSvc := auth.NewJWTService("test-secret-32-chars-long-enough!", -1*time.Second, 30*24*time.Hour)
	svc := newJWT() // validation service

	user := &models.User{ID: uuid.New(), Email: "u@example.com", Role: models.RoleDriver}
	token, _, _ := expiredSvc.GenerateAccessToken(user)

	handler := middleware.AuthMiddleware(svc)(&sentinelHandler{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	code := errorCode(rr.Body.Bytes())
	if code != models.ErrCodeTokenExpired {
		t.Errorf("expected error code %q, got %q", models.ErrCodeTokenExpired, code)
	}
}

func TestAuthMiddleware_ValidToken_PopulatesContext(t *testing.T) {
	svc := newJWT()
	userID := uuid.New()

	user := &models.User{ID: userID, Email: "driver@example.com", Role: models.RoleDriver}
	token, _, _ := svc.GenerateAccessToken(user)

	var capturedID uuid.UUID
	var capturedRole models.Role

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID, _ = httputil.GetUserID(r.Context())
		capturedRole, _ = httputil.GetRole(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.AuthMiddleware(svc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedID != userID {
		t.Errorf("user_id mismatch: got %v want %v", capturedID, userID)
	}
	if capturedRole != models.RoleDriver {
		t.Errorf("role mismatch: got %q want %q", capturedRole, models.RoleDriver)
	}
}

// --- RequireRole tests ---

func TestRequireRole_CorrectRole_Passes(t *testing.T) {
	sentinel := &sentinelHandler{}
	handler := middleware.RequireRole(models.RoleAdmin)(sentinel)

	ctx := context.WithValue(context.Background(), httputil.RoleKey, models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !sentinel.called {
		t.Error("next handler should be called when role matches")
	}
}

func TestRequireRole_WrongRole_Returns403(t *testing.T) {
	handler := middleware.RequireRole(models.RoleAdmin)(&sentinelHandler{})

	ctx := context.WithValue(context.Background(), httputil.RoleKey, models.RoleDriver)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireRole_NoRoleInContext_Returns401(t *testing.T) {
	handler := middleware.RequireRole(models.RoleAdmin)(&sentinelHandler{})

	req := httptest.NewRequest(http.MethodGet, "/", nil) // no role in context
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	handler := middleware.RequireRole(models.RoleAdmin, models.RoleCarOwner)(&sentinelHandler{})

	for _, role := range []models.Role{models.RoleAdmin, models.RoleCarOwner} {
		ctx := context.WithValue(context.Background(), httputil.RoleKey, role)
		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("role %q: expected 200, got %d", role, rr.Code)
		}
	}
}

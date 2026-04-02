package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/drivebai/backend/internal/models"
	"github.com/google/uuid"
)

// context keys
type contextKey string

const (
	ctxUserID contextKey = "user_id"
	ctxEmail  contextKey = "email"
	ctxRole   contextKey = "role"
)

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxUserID).(uuid.UUID)
	return id, ok
}

func GetRole(ctx context.Context) (models.Role, bool) {
	r, ok := ctx.Value(ctxRole).(models.Role)
	return r, ok
}

func SetAuthContext(ctx context.Context, userID uuid.UUID, email string, role models.Role) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxEmail, email)
	ctx = context.WithValue(ctx, ctxRole, role)
	return ctx
}

// json helpers
type errorResponse struct {
	Error *models.APIError `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, apiErr *models.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: apiErr})
}

func DecodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

package httputil

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/drivebai/backend/internal/models"
	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error *models.APIError `json:"error"`
}

type SuccessResponse struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteSuccess(w http.ResponseWriter, status int, message string, data interface{}) {
	response := SuccessResponse{
		Message: message,
		Data:    data,
	}
	WriteJSON(w, status, response)
}

func WriteError(w http.ResponseWriter, status int, apiErr *models.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: apiErr})
}

type ContextKey string

const (
	UserIDKey ContextKey = "user_id"
	EmailKey  ContextKey = "email"
	RoleKey   ContextKey = "role"
)

// Context helpers
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

func GetEmail(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailKey).(string)
	return email, ok
}

func GetRole(ctx context.Context) (models.Role, bool) {
	role, ok := ctx.Value(RoleKey).(models.Role)
	return role, ok
}

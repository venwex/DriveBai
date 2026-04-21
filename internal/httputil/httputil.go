package httputil

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

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

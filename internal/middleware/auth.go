package middleware

import (
	"net/http"
	"strings"

	"github.com/drivebai/backend/internal/auth"
	"github.com/drivebai/backend/internal/handlers"
	"github.com/drivebai/backend/internal/models"
)

func AuthMiddleware(jwtSvc *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				handlers.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				handlers.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
				return
			}

			claims, err := jwtSvc.ValidateAccessToken(parts[1])
			if err != nil {
				if apiErr := models.GetAPIError(err); apiErr != nil {
					handlers.WriteError(w, http.StatusUnauthorized, apiErr)
				} else {
					handlers.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
				}
				return
			}

			ctx := handlers.SetAuthContext(r.Context(), claims.UserID, claims.Email, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

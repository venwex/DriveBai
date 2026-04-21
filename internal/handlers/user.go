package handlers

import (
	"log/slog"
	"net/http"

	"github.com/drivebai/backend/internal/httputil"
	"github.com/drivebai/backend/internal/models"
	"github.com/drivebai/backend/internal/repository"
)

type UserHandler struct {
	userRepo  *repository.UserRepository
	docRepo   *repository.DocumentRepository
	logger    *slog.Logger
	uploadDir string
}

func NewUserHandler(userRepo *repository.UserRepository, docRepo *repository.DocumentRepository, slog *slog.Logger, uploadDir string) *UserHandler {
	return &UserHandler{
		userRepo:  userRepo,
		docRepo:   docRepo,
		logger:    slog,
		uploadDir: uploadDir,
	}
}

func (h *UserHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		if apiErr := models.GetAPIError(err); apiErr != nil {
			WriteError(w, http.StatusNotFound, apiErr)
		} else {
			h.logger.Error("failed to get user", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		}
		return
	}

	profile := UserProfile{
		ID:               user.ID,
		Email:            user.Email,
		Role:             user.Role,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Phone:            user.Phone,
		IsEmailVerified:  user.IsEmailVerified,
		OnboardingStatus: user.OnboardingStatus,
		ProfilePhotoURL:  user.ProfilePhotoURL,
	}

	WriteJSON(w, http.StatusOK, profile)
}

type UpdateProfileRequest struct {
	Role      *models.Role `json:"role,omitempty"`
	FirstName *string      `json:"first_name,omitempty"`
	LastName  *string      `json:"last_name,omitempty"`
	Phone     *string      `json:"phone,omitempty"`
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.GetUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewAPIError("INVALID_REQUEST", "Invalid request body"))
		return
	}

	// Get current user
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		if apiErr := models.GetAPIError(err); apiErr != nil {
			WriteError(w, http.StatusNotFound, apiErr)
		} else {
			h.logger.Error("failed to get user", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		}
		return
	}

	// Update role if provided
	if req.Role != nil {
		// Validate role - only driver and car_owner allowed via API
		if *req.Role != models.RoleDriver && *req.Role != models.RoleCarOwner {
			WriteError(w, http.StatusBadRequest, models.NewAPIError("INVALID_ROLE", "Role must be 'driver' or 'car_owner'"))
			return
		}
		if err := h.userRepo.UpdateRole(r.Context(), userID, *req.Role); err != nil {
			h.logger.Error("failed to update role", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
			return
		}
		user.Role = *req.Role
	}

	// Update other fields if provided
	updated := false
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
		updated = true
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
		updated = true
	}
	if req.Phone != nil {
		user.Phone = req.Phone
		updated = true
	}

	if updated {
		if err := h.userRepo.Update(r.Context(), user); err != nil {
			h.logger.Error("failed to update user", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
			return
		}
	}

	// Reload user to get updated data
	user, _ = h.userRepo.GetByID(r.Context(), userID)

	profile := UserProfile{
		ID:               user.ID,
		Email:            user.Email,
		Role:             user.Role,
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Phone:            user.Phone,
		IsEmailVerified:  user.IsEmailVerified,
		OnboardingStatus: user.OnboardingStatus,
		ProfilePhotoURL:  user.ProfilePhotoURL,
	}

	WriteSuccess(w, http.StatusOK, "Profile updated successfully", profile)
}

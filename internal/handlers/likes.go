package handlers

import (
	"net/http"
	"strings"

	"github.com/drivebai/backend/internal/models"
	"github.com/drivebai/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LikesHandler struct {
	likesRepo *repository.LikesRepository
	carRepo   *repository.CarRepository
}

func NewLikesHandler(likesRepo *repository.LikesRepository, carRepo *repository.CarRepository) *LikesHandler {
	return &LikesHandler{likesRepo: likesRepo, carRepo: carRepo}
}

func (h *LikesHandler) GetLikedListings(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	ids, err := h.likesRepo.GetLikedListingIDs(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"liked_listing_ids": ids,
	})
}

func (h *LikesHandler) LikeListing(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	listingID, err := uuid.Parse(strings.ToLower(chi.URLParam(r, "listingId")))
	if err != nil {
		WriteError(w, http.StatusBadRequest, models.NewAPIError("INVALID_LISTING_ID", "Invalid listing ID format"))
		return
	}

	car, err := h.carRepo.GetByID(r.Context(), listingID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if car == nil {
		WriteError(w, http.StatusNotFound, models.NewAPIError("LISTING_NOT_FOUND", "Listing not found"))
		return
	}

	if err := h.likesRepo.AddLike(r.Context(), userID, listingID); err != nil {
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Listing liked successfully"})
}

func (h *LikesHandler) UnlikeListing(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	listingID, err := uuid.Parse(strings.ToLower(chi.URLParam(r, "listingId")))
	if err != nil {
		WriteError(w, http.StatusBadRequest, models.NewAPIError("INVALID_LISTING_ID", "Invalid listing ID format"))
		return
	}

	if err := h.likesRepo.RemoveLike(r.Context(), userID, listingID); err != nil {
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Listing unliked successfully"})
}

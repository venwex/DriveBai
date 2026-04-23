package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/drivebai/backend/internal/httputil"
	"github.com/drivebai/backend/internal/models"
	"github.com/drivebai/backend/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CarHandler struct {
	carRepo   *repository.CarRepository
	photoRepo *repository.CarPhotoRepository
	userRepo  *repository.UserRepository
	docRepo   *repository.CarDocumentRepository
	uploadDir string
}

func NewCarHandler(
	carRepo *repository.CarRepository,
	photoRepo *repository.CarPhotoRepository,
	docRepo *repository.CarDocumentRepository,
	userRepo *repository.UserRepository,
	uploadDir string,
) *CarHandler {
	return &CarHandler{
		carRepo:   carRepo,
		photoRepo: photoRepo,
		docRepo:   docRepo,
		userRepo:  userRepo,
		uploadDir: uploadDir,
	}
}

// ListCars returns all cars for the authenticated owner
func (h *CarHandler) ListCars(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := httputil.GetUserID(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	cars, err := h.carRepo.GetByOwnerID(ctx, userID)
	if err != nil {
		slog.Error("failed to get cars", "error", err, "error_type", fmt.Sprintf("%T", err), "user_id", userID)
		httputil.WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	// Get owner info once
	owner, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		slog.Error("failed to get owner", "error", err, "user_id", userID)
	}

	// Build response with photos and documents for each car
	var responses []*models.CarResponse
	for _, car := range cars {
		photos, _ := h.photoRepo.GetByCarID(ctx, car.ID)
		documents, _ := h.docRepo.GetByCarID(ctx, car.ID)
		responses = append(responses, car.ToResponse(photos, documents, owner))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"cars": responses,
	})
}

// GetCar returns a specific car by ID
func (h *CarHandler) GetCar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := httputil.GetUserID(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	carIDStr := chi.URLParam(r, "carId")
	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid car ID"))
		return
	}

	car, err := h.carRepo.GetByID(ctx, carID)
	if err != nil {
		slog.Error("failed to get car", "error", err, "car_id", carID)
		httputil.WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	if car == nil {
		httputil.WriteError(w, http.StatusNotFound, models.NewAPIError("NOT_FOUND", "Car not found"))
		return
	}

	// Verify ownership
	if car.OwnerID != userID {
		httputil.WriteError(w, http.StatusForbidden, models.NewAPIError("FORBIDDEN", "You do not own this car"))
		return
	}

	// Get photos, documents, and owner info
	photos, _ := h.photoRepo.GetByCarID(ctx, car.ID)
	documents, _ := h.docRepo.GetByCarID(ctx, car.ID)
	owner, _ := h.userRepo.GetByID(ctx, userID)

	httputil.WriteJSON(w, http.StatusOK, car.ToResponse(photos, documents, owner))
}

func (h *CarHandler) CreateCar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	var req models.CreateCarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}
	if req.Make == "" || req.Model == "" || req.Year == 0 {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Make, model, and year are required"))
		return
	}

	title := req.Title
	if title == "" {
		title = fmt.Sprintf("%d %s %s", req.Year, req.Make, req.Model)
	}

	now := time.Now()
	car := &models.Car{
		ID: uuid.New(), OwnerID: userID, Title: title,
		Make: req.Make, Model: req.Model, Year: req.Year,
		BodyType: req.BodyType, FuelType: req.FuelType, Mileage: req.Mileage,
		IsForRent: req.IsForRent, IsForSale: req.IsForSale,
		Currency: "USD", Status: models.CarStatusPending,
		CreatedAt: now, UpdatedAt: now,
	}

	if car.BodyType == "" {
		car.BodyType = models.BodyTypeSedan
	}
	if car.FuelType == "" {
		car.FuelType = models.FuelTypeGas
	}

	if req.Description != nil {
		car.Description = sql.NullString{String: *req.Description, Valid: true}
	}
	if req.Address != nil {
		car.Address = sql.NullString{String: *req.Address, Valid: true}
	}
	if req.Neighborhood != nil {
		car.Neighborhood = sql.NullString{String: *req.Neighborhood, Valid: true}
	}
	if req.Latitude != nil {
		car.Latitude = sql.NullFloat64{Float64: *req.Latitude, Valid: true}
	}
	if req.Longitude != nil {
		car.Longitude = sql.NullFloat64{Float64: *req.Longitude, Valid: true}
	}
	if req.Area != nil {
		car.Area = sql.NullString{String: *req.Area, Valid: true}
	}
	if req.Street != nil {
		car.Street = sql.NullString{String: *req.Street, Valid: true}
	}
	if req.Block != nil {
		car.Block = sql.NullString{String: *req.Block, Valid: true}
	}
	if req.Zip != nil {
		car.Zip = sql.NullString{String: *req.Zip, Valid: true}
	}
	if req.WeeklyRentPrice != nil {
		car.WeeklyRentPrice = sql.NullFloat64{Float64: *req.WeeklyRentPrice, Valid: true}
	}
	if req.SalePrice != nil {
		car.SalePrice = sql.NullFloat64{Float64: *req.SalePrice, Valid: true}
	}
	if req.MinYearsLicensed != nil {
		car.MinYearsLicensed = *req.MinYearsLicensed
	} else {
		car.MinYearsLicensed = 2
	}
	if req.DepositAmount != nil {
		car.DepositAmount = *req.DepositAmount
	} else {
		car.DepositAmount = 500
	}
	if req.InsuranceCoverage != nil {
		car.InsuranceCoverage = *req.InsuranceCoverage
	} else {
		car.InsuranceCoverage = models.InsuranceFullCoverage
	}

	if err := h.carRepo.Create(ctx, car); err != nil {
		slog.Error("failed to create car", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	// Get owner info for response
	owner, _ := h.userRepo.GetByID(ctx, userID)

	slog.Info("car created", "car_id", car.ID, "user_id", userID)
	httputil.WriteJSON(w, http.StatusCreated, car.ToResponse(nil, nil, owner))
}

func (h *CarHandler) UpdateCar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	carID, err := uuid.Parse(chi.URLParam(r, "carId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid car ID"))
		return
	}

	car, err := h.carRepo.GetByID(ctx, carID)
	if err != nil || car == nil {
		WriteError(w, http.StatusNotFound, models.NewAPIError("NOT_FOUND", "Car not found"))
		return
	}
	if car.OwnerID != userID {
		WriteError(w, http.StatusForbidden, models.NewAPIError("FORBIDDEN", "You do not own this car"))
		return
	}

	var req models.UpdateCarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}

	if req.Title != nil {
		car.Title = *req.Title
	}
	if req.Description != nil {
		car.Description = sql.NullString{String: *req.Description, Valid: true}
	}
	if req.Make != nil {
		car.Make = *req.Make
	}
	if req.Model != nil {
		car.Model = *req.Model
	}
	if req.Year != nil {
		car.Year = *req.Year
	}
	if req.BodyType != nil {
		car.BodyType = *req.BodyType
	}
	if req.FuelType != nil {
		car.FuelType = *req.FuelType
	}
	if req.Mileage != nil {
		car.Mileage = *req.Mileage
	}
	if req.Address != nil {
		car.Address = sql.NullString{String: *req.Address, Valid: true}
	}
	if req.Neighborhood != nil {
		car.Neighborhood = sql.NullString{String: *req.Neighborhood, Valid: true}
	}
	if req.Latitude != nil {
		car.Latitude = sql.NullFloat64{Float64: *req.Latitude, Valid: true}
	}
	if req.Longitude != nil {
		car.Longitude = sql.NullFloat64{Float64: *req.Longitude, Valid: true}
	}
	if req.Area != nil {
		car.Area = sql.NullString{String: *req.Area, Valid: true}
	}
	if req.Street != nil {
		car.Street = sql.NullString{String: *req.Street, Valid: true}
	}
	if req.Block != nil {
		car.Block = sql.NullString{String: *req.Block, Valid: true}
	}
	if req.Zip != nil {
		car.Zip = sql.NullString{String: *req.Zip, Valid: true}
	}
	if req.IsForRent != nil {
		car.IsForRent = *req.IsForRent
	}
	if req.WeeklyRentPrice != nil {
		car.WeeklyRentPrice = sql.NullFloat64{Float64: *req.WeeklyRentPrice, Valid: true}
	}
	if req.IsForSale != nil {
		car.IsForSale = *req.IsForSale
	}
	if req.SalePrice != nil {
		car.SalePrice = sql.NullFloat64{Float64: *req.SalePrice, Valid: true}
	}
	if req.MinYearsLicensed != nil {
		car.MinYearsLicensed = *req.MinYearsLicensed
	}
	if req.DepositAmount != nil {
		car.DepositAmount = *req.DepositAmount
	}
	if req.InsuranceCoverage != nil {
		car.InsuranceCoverage = *req.InsuranceCoverage
	}
	if req.Status != nil {
		car.Status = *req.Status
	}
	if req.IsPaused != nil {
		car.IsPaused = *req.IsPaused
	}

	if err := h.carRepo.Update(ctx, car); err != nil {
		slog.Error("failed to update car", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	// Get photos, documents, and owner info for response
	photos, _ := h.photoRepo.GetByCarID(ctx, car.ID)
	documents, _ := h.docRepo.GetByCarID(ctx, car.ID)
	owner, _ := h.userRepo.GetByID(ctx, userID)

	slog.Info("car updated", "car_id", car.ID, "user_id", userID)
	httputil.WriteJSON(w, http.StatusOK, car.ToResponse(photos, documents, owner))
}

// DeleteCar deletes a car listing
func (h *CarHandler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := httputil.GetUserID(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	carIDStr := chi.URLParam(r, "carId")
	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid car ID"))
		return
	}

	// Get existing car
	car, err := h.carRepo.GetByID(ctx, carID)
	if err != nil || car == nil {
		httputil.WriteError(w, http.StatusNotFound, models.NewAPIError("NOT_FOUND", "Car not found"))
		return
	}

	// Verify ownership
	if car.OwnerID != userID {
		httputil.WriteError(w, http.StatusForbidden, models.NewAPIError("FORBIDDEN", "You do not own this car"))
		return
	}

	// Delete photos from disk
	photos, _ := h.photoRepo.GetByCarID(ctx, carID)
	for _, photo := range photos {
		os.Remove(photo.FilePath)
	}

	// Delete documents from disk
	documents, _ := h.docRepo.GetByCarID(ctx, carID)
	for _, doc := range documents {
		os.Remove(doc.FilePath)
	}

	// Delete car (cascades to photos and documents)
	if err := h.carRepo.Delete(ctx, carID); err != nil {
		slog.Error("failed to delete car", "error", err, "car_id", carID)
		httputil.WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	slog.Info("car deleted", "car_id", carID, "user_id", userID)
	httputil.WriteSuccess(w, http.StatusOK, "Car deleted successfully", nil)
}

// PauseCar toggles the paused state of a car
func (h *CarHandler) PauseCar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := httputil.GetUserID(ctx)
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, models.ErrUnauthorized)
		return
	}

	carIDStr := chi.URLParam(r, "carId")
	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid car ID"))
		return
	}

	// Get existing car
	car, err := h.carRepo.GetByID(ctx, carID)
	if err != nil || car == nil {
		httputil.WriteError(w, http.StatusNotFound, models.NewAPIError("NOT_FOUND", "Car not found"))
		return
	}

	// Verify ownership
	if car.OwnerID != userID {
		httputil.WriteError(w, http.StatusForbidden, models.NewAPIError("FORBIDDEN", "You do not own this car"))
		return
	}

	// Toggle pause state
	newIsPaused := !car.IsPaused
	newStatus := models.CarStatusAvailable
	if newIsPaused {
		newStatus = models.CarStatusPaused
	}

	if err := h.carRepo.UpdateStatus(ctx, carID, newStatus, newIsPaused); err != nil {
		slog.Error("failed to pause car", "error", err, "car_id", carID)
		httputil.WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	// Get updated car
	car, _ = h.carRepo.GetByID(ctx, carID)
	photos, _ := h.photoRepo.GetByCarID(ctx, car.ID)
	documents, _ := h.docRepo.GetByCarID(ctx, car.ID)
	owner, _ := h.userRepo.GetByID(ctx, userID)

	slog.Info("car paused toggled", "car_id", carID, "is_paused", newIsPaused)
	httputil.WriteJSON(w, http.StatusOK, car.ToResponse(photos, documents, owner))
}

func (h *CarHandler) ListAvailableListings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters for filtering
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "available"
	}

	search := r.URL.Query().Get("search")

	cars, err := h.carRepo.GetAvailableListings(r.Context(), status, search)
	if err != nil {
		slog.Error("failed to list cars", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	// Build response with photos and owner info for each car
	var responses []*models.CarResponse
	for _, car := range cars {
		photos, _ := h.photoRepo.GetByCarID(ctx, car.ID)
		owner, _ := h.userRepo.GetByID(r.Context(), car.OwnerID)
		responses = append(responses, car.ToResponse(photos, nil, owner))
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"listings": responses,
		"count":    len(responses),
	})
}

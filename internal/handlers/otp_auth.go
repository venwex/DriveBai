package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"drivebai/internal/auth"
	"drivebai/internal/models"
	"drivebai/internal/repository"
)

type OTPAuthHandler struct {
	userRepo  *repository.UserRepository
	tokenRepo *repository.TokenRepository
	otpRepo   *repository.LoginOTPRepository
	jwtSvc    *auth.JWTService
	logger    *slog.Logger
}

func NewOTPAuthHandler(
	userRepo *repository.UserRepository,
	tokenRepo *repository.TokenRepository,
	otpRepo *repository.LoginOTPRepository,
	jwtSvc *auth.JWTService,
	logger *slog.Logger,
) *OTPAuthHandler {
	return &OTPAuthHandler{userRepo: userRepo, tokenRepo: tokenRepo, otpRepo: otpRepo, jwtSvc: jwtSvc, logger: logger}
}

const (
	maxOTPsPerEmail    = 5
	maxIPOTPs          = 10
	otpRateLimitWindow = 15 * time.Minute
	otpExpiry          = 10 * time.Minute
)

// ─── Request / Response types ────────────────────────────────────────────────

type OTPRequestReq struct {
	Email string `json:"email"`
}

type OTPVerifyReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type OTPVerifyLoginResp struct {
	Kind         string             `json:"kind"`
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresAt    models.RFC3339Time `json:"expires_at"`
	User         UserProfile        `json:"user"`
}

type OTPVerifyRegisterResp struct {
	Kind              string `json:"kind"`
	RegistrationToken string `json:"registration_token"`
	Email             string `json:"email"`
}

type CompleteRegReq struct {
	RegistrationToken string      `json:"registration_token"`
	FirstName         string      `json:"first_name"`
	LastName          string      `json:"last_name"`
	Password          string      `json:"password"`
	Phone             string      `json:"phone,omitempty"`
	Role              models.Role `json:"role"`
}

type RegisterResponse struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresAt    models.RFC3339Time `json:"expires_at"`
	User         UserProfile        `json:"user"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func (h *OTPAuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPRequestReq
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("A valid email address is required"))
		return
	}

	ctx := r.Context()
	since := time.Now().Add(-otpRateLimitWindow)

	emailCount, err := h.userRepo.GetOTPSendCount(ctx, req.Email, since)
	if err != nil {
		h.logger.Error("otp rate-limit check failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if emailCount >= maxOTPsPerEmail {
		WriteError(w, http.StatusTooManyRequests, models.ErrRateLimited)
		return
	}

	ip := realIP(r)
	ipCount, err := h.userRepo.GetOTPSendCount(ctx, ip, since)
	if err != nil {
		h.logger.Error("otp IP rate-limit check failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if ipCount >= maxIPOTPs {
		WriteError(w, http.StatusTooManyRequests, models.ErrRateLimited)
		return
	}

	rawCode, codeHash, err := auth.GenerateOTP()
	if err != nil {
		h.logger.Error("failed to generate OTP", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	expiresAt := time.Now().Add(otpExpiry)
	if _, err := h.otpRepo.Create(ctx, req.Email, codeHash, expiresAt, ip, r.UserAgent()); err != nil {
		h.logger.Error("failed to store OTP", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	_ = h.userRepo.RecordOTPSend(ctx, req.Email, ip)
	_ = h.userRepo.RecordOTPSend(ctx, ip, ip)

	// Dev mode: print OTP to console
	fmt.Printf("\n"+
		"╔══════════════════════════════════════════════════════════╗\n"+
		"║  LOGIN OTP (dev mode)                                    ║\n"+
		"╠══════════════════════════════════════════════════════════╣\n"+
		"║  To:   %-50s ║\n"+
		"║  Code: %-50s ║\n"+
		"╚══════════════════════════════════════════════════════════╝\n\n",
		req.Email, rawCode)

	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "If this email is valid, a login code has been sent.",
	})
}

func (h *OTPAuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPVerifyReq
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Code = strings.TrimSpace(req.Code)

	if req.Email == "" || req.Code == "" {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Email and code are required"))
		return
	}
	if !auth.ValidateOTPFormat(req.Code) {
		WriteError(w, http.StatusBadRequest, models.ErrOTPInvalid)
		return
	}

	ctx := r.Context()

	otp, err := h.otpRepo.GetLatestUnconsumed(ctx, req.Email)
	if err != nil {
		h.logger.Error("failed to fetch OTP", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if otp == nil || otp.IsExpired() || otp.IsConsumed() {
		WriteError(w, http.StatusUnauthorized, models.ErrOTPExpired)
		return
	}
	if otp.IsLocked() {
		WriteError(w, http.StatusUnauthorized, models.ErrOTPAttemptsExceeded)
		return
	}

	if auth.HashOTP(req.Code) != otp.CodeHash {
		newAttempts, _ := h.otpRepo.IncrementAttempts(ctx, otp.ID)
		if newAttempts >= models.LoginOTPMaxAttempts {
			WriteError(w, http.StatusUnauthorized, models.ErrOTPAttemptsExceeded)
			return
		}
		WriteError(w, http.StatusUnauthorized, models.ErrOTPInvalid)
		return
	}

	if err := h.otpRepo.MarkConsumed(ctx, otp.ID); err != nil {
		h.logger.Error("failed to consume OTP", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	user, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !models.IsAPIError(err) {
		h.logger.Error("user lookup failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	if user != nil {
		tokens, err := h.generateTokens(ctx, user)
		if err != nil {
			h.logger.Error("token generation failed", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
			return
		}
		WriteJSON(w, http.StatusOK, OTPVerifyLoginResp{
			Kind: "login", AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
			ExpiresAt: models.NewRFC3339Time(tokens.ExpiresAt), User: toUserProfile(user),
		})
		return
	}

	regToken, err := h.jwtSvc.GenerateRegistrationToken(req.Email)
	if err != nil {
		h.logger.Error("registration token failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	WriteJSON(w, http.StatusOK, OTPVerifyRegisterResp{Kind: "register", RegistrationToken: regToken, Email: req.Email})
}

func (h *OTPAuthHandler) CompleteRegistration(w http.ResponseWriter, r *http.Request) {
	var req CompleteRegReq
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}
	if req.RegistrationToken == "" {
		WriteError(w, http.StatusBadRequest, models.ErrRegTokenRequired)
		return
	}

	email, err := h.jwtSvc.ValidateRegistrationToken(req.RegistrationToken)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, models.ErrTokenInvalid)
		return
	}

	if req.FirstName == "" || req.LastName == "" {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("First and last name are required"))
		return
	}
	if req.Password == "" || len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Password must be at least 8 characters"))
		return
	}
	if !req.Role.IsValid() || req.Role == models.RoleAdmin {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Role must be 'driver' or 'car_owner'"))
		return
	}

	ctx := r.Context()

	exists, err := h.userRepo.EmailExists(ctx, email)
	if err != nil {
		h.logger.Error("email check failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if exists {
		WriteError(w, http.StatusConflict, models.ErrEmailTaken)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error("password hash failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	var phone *string
	if req.Phone != "" {
		phone = &req.Phone
	}

	user := &models.User{
		Email: email, PasswordHash: &hash, Role: req.Role,
		FirstName: req.FirstName, LastName: req.LastName, Phone: phone,
		IsEmailVerified: true, OnboardingStatus: models.OnboardingRoleSelected,
	}
	if err := h.userRepo.Create(ctx, user); err != nil {
		if apiErr := models.GetAPIError(err); apiErr != nil {
			WriteError(w, http.StatusConflict, apiErr)
		} else {
			h.logger.Error("user creation failed", "error", err)
			WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		}
		return
	}

	tokens, err := h.generateTokens(ctx, user)
	if err != nil {
		h.logger.Error("token generation failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	WriteJSON(w, http.StatusCreated, RegisterResponse{
		AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
		ExpiresAt: models.NewRFC3339Time(tokens.ExpiresAt), User: toUserProfile(user),
	})
}

func (h *OTPAuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenReq
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Invalid request body"))
		return
	}
	if req.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, models.NewValidationError("Refresh token is required"))
		return
	}

	ctx := r.Context()
	hash := h.jwtSvc.HashRefreshToken(req.RefreshToken)

	stored, err := h.tokenRepo.GetByHash(ctx, hash)
	if err != nil {
		h.logger.Error("token lookup failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}
	if stored == nil || stored.IsRevoked() {
		WriteError(w, http.StatusUnauthorized, models.ErrTokenInvalid)
		return
	}
	if stored.IsExpired() {
		WriteError(w, http.StatusUnauthorized, models.ErrTokenExpired)
		return
	}

	_ = h.tokenRepo.RevokeToken(ctx, stored.ID)

	user, err := h.userRepo.GetByID(ctx, stored.UserID)
	if err != nil {
		h.logger.Error("user lookup failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	tokens, err := h.generateTokens(ctx, user)
	if err != nil {
		h.logger.Error("token generation failed", "error", err)
		WriteError(w, http.StatusInternalServerError, models.ErrInternalError)
		return
	}

	WriteJSON(w, http.StatusOK, RegisterResponse{
		AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken,
		ExpiresAt: models.NewRFC3339Time(tokens.ExpiresAt), User: toUserProfile(user),
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (h *OTPAuthHandler) generateTokens(ctx context.Context, user *models.User) (*auth.TokenPair, error) {
	access, exp, err := h.jwtSvc.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refresh, refreshHash, refreshExp, err := h.jwtSvc.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	tok := &models.RefreshToken{UserID: user.ID, TokenHash: refreshHash, ExpiresAt: refreshExp}
	if err := h.tokenRepo.CreateRefreshToken(ctx, tok); err != nil {
		return nil, err
	}
	return &auth.TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: exp}, nil
}

type UserProfile struct {
	ID               interface{}              `json:"id"`
	Email            string                   `json:"email"`
	Role             models.Role              `json:"role"`
	FirstName        string                   `json:"first_name"`
	LastName         string                   `json:"last_name"`
	Phone            *string                  `json:"phone,omitempty"`
	IsEmailVerified  bool                     `json:"is_email_verified"`
	OnboardingStatus models.OnboardingStatus  `json:"onboarding_status"`
	ProfilePhotoURL  *string                  `json:"profile_photo_url,omitempty"`
}

func toUserProfile(u *models.User) UserProfile {
	return UserProfile{
		ID: u.ID, Email: u.Email, Role: u.Role,
		FirstName: u.FirstName, LastName: u.LastName, Phone: u.Phone,
		IsEmailVerified: u.IsEmailVerified, OnboardingStatus: u.OnboardingStatus,
		ProfilePhotoURL: u.ProfilePhotoURL,
	}
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if idx := strings.Index(ip, ","); idx != -1 {
			return strings.TrimSpace(ip[:idx])
		}
		return strings.TrimSpace(ip)
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

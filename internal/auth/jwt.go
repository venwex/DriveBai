package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"drivebai/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	registrationTokenTTL = 15 * time.Minute
	registrationPurpose  = "otp_registration"
	tokenIssuer          = "drivebai"
)

type JWTService struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

type AccessTokenClaims struct {
	UserID uuid.UUID   `json:"user_id"`
	Email  string      `json:"email"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func NewJWTService(secret string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), accessTokenTTL: accessTTL, refreshTokenTTL: refreshTTL}
}

func (svc *JWTService) GenerateAccessToken(user *models.User) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(svc.accessTokenTTL)
	claims := AccessTokenClaims{
		UserID: user.ID, Email: user.Email, Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    tokenIssuer,
			Subject:   user.ID.String(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(svc.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (svc *JWTService) ValidateAccessToken(tokenString string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return svc.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, models.ErrTokenExpired
		}
		return nil, models.ErrTokenInvalid
	}
	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, models.ErrTokenInvalid
	}
	return claims, nil
}

func (svc *JWTService) GenerateRefreshToken() (string, string, time.Time, error) {
	raw := uuid.New().String() + "-" + uuid.New().String()
	return raw, svc.HashRefreshToken(raw), time.Now().Add(svc.refreshTokenTTL), nil
}

func (svc *JWTService) HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (svc *JWTService) GetRefreshTokenTTL() time.Duration {
	return svc.refreshTokenTTL
}

// ─── Registration token (short-lived JWT proving email ownership) ────────────

type RegistrationClaims struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

func (svc *JWTService) GenerateRegistrationToken(email string) (string, error) {
	claims := RegistrationClaims{
		Email: email, Purpose: registrationPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(registrationTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    tokenIssuer,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(svc.secret)
}

func (svc *JWTService) ValidateRegistrationToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RegistrationClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return svc.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", models.ErrTokenExpired
		}
		return "", models.ErrTokenInvalid
	}
	claims, ok := token.Claims.(*RegistrationClaims)
	if !ok || !token.Valid || claims.Purpose != registrationPurpose {
		return "", models.ErrTokenInvalid
	}
	return claims.Email, nil
}

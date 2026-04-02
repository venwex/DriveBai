package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const otpLength = 6

// GenerateOTP creates a 6-digit code and its SHA-256 hash.
func GenerateOTP() (string, string, error) {
	code := ""
	for i := 0; i < otpLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, HashOTP(code), nil
}

func HashOTP(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

func ValidateOTPFormat(code string) bool {
	if len(code) != otpLength {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

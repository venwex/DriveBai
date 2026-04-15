package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const otpLength = 6

// Генерирует 6-значный OTP и возвращает как исходный код так и его хеш
func GenerateOTP() (string, string, error) {
	code := ""
	for i := 0; i < otpLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}

	hash := HashOTP(code)

	return code, hash, nil
}

// Создаем хенкод sha256 для OTP
func HashOTP(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:])
}

// Проверяем имеет ли код OTP правильный формат
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
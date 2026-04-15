package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// Генерирует хеш пароль в виде открытого текста с использованием алгоритма bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Сравниваем пароль в виде открытого текста с хешем
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
package users

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"rastapi/internal/auth"
)

const (
	minPasswordLen = 8
	maxEmailLen    = 254 // ограничение длины адреса по RFC 5321
)

// NormalizeEmail убирает пробелы по краям и приводит к нижнему регистру,
// чтобы "A@B.c" и "a@b.c" считались одним адресом.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateEmail проверяет уже нормализованный email.
func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > maxEmailLen {
		return fmt.Errorf("email must be at most %d characters", maxEmailLen)
	}
	addr, err := mail.ParseAddress(email)
	// ParseAddress принимает и "Имя <a@b.c>" — нам нужен только голый адрес.
	if err != nil || addr.Address != email || addr.Name != "" {
		return errors.New("email is invalid")
	}
	return nil
}

// validatePassword: 8..72 байта (верхний предел — лимит bcrypt).
func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	if len(password) > auth.MaxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", auth.MaxPasswordBytes)
	}
	return nil
}

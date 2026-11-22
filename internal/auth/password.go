// Package auth — пароли (bcrypt), JWT-токены и middleware авторизации.
package auth

import "golang.org/x/crypto/bcrypt"

// MaxPasswordBytes — bcrypt учитывает только первые 72 байта пароля.
const MaxPasswordBytes = 72

// HashPassword возвращает bcrypt-хеш пароля (соль генерируется внутри и хранится в хеше).
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword сообщает, подходит ли пароль к хешу.
// Любая ошибка (неверный пароль, битый хеш) трактуется как «не подходит».
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

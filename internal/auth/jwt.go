package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken — общая ошибка для любого невалидного токена.
// Причину (подпись, срок, формат) наружу не раскрываем.
var ErrInvalidToken = errors.New("invalid or expired token")

// Identity — кто выполняет запрос (достаётся из валидного токена).
type Identity struct {
	UserID int64
	Email  string
}

// claims — содержимое токена: sub=user_id, email, exp, iat.
type claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// Tokens выпускает и проверяет JWT (HS256).
type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // подменяется в тестах
}

// NewTokens создаёт выпускатель токенов с секретом и временем жизни.
func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// Issue выпускает токен для пользователя и возвращает момент его истечения.
func (t *Tokens) Issue(userID int64, email string) (string, time.Time, error) {
	now := t.now()
	// В токене время хранится с точностью до секунды — отдаём клиенту то же значение.
	expiresAt := now.Add(t.ttl).Truncate(jwt.TimePrecision)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	signed, err := tok.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse проверяет подпись, алгоритм и срок действия токена.
// Любая проблема возвращается как ErrInvalidToken.
func (t *Tokens) Parse(token string) (Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // защита от alg=none и подмены алгоритма
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}

	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || id <= 0 {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UserID: id, Email: c.Email}, nil
}

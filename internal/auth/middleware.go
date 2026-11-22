package auth

import (
	"context"
	"net/http"
	"strings"

	"rastapi/internal/httpx"
	"rastapi/internal/logger"
)

type identityKey struct{}

// WithIdentity кладёт пользователя в контекст.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFromContext достаёт пользователя из контекста.
// ok=false, если запрос прошёл без RequireAuth.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

// TokenParser — то, что умеет проверять токен (*Tokens подходит).
type TokenParser interface {
	Parse(token string) (Identity, error)
}

// RequireAuth пропускает запрос дальше только с валидным
// заголовком "Authorization: Bearer <token>", иначе отвечает 401.
func RequireAuth(tokens TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				unauthorized(w, r, "missing bearer token")
				return
			}
			id, err := tokens.Parse(raw)
			if err != nil {
				unauthorized(w, r, err.Error())
				return
			}

			ctx := WithIdentity(r.Context(), id)
			// Все логи внутри хендлера будут содержать user_id.
			ctx = logger.WithContext(ctx, logger.FromContext(ctx).With("user_id", id.UserID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken достаёт токен из заголовка Authorization.
// Схема "Bearer" сравнивается без учёта регистра (RFC 7235).
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(h, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func unauthorized(w http.ResponseWriter, r *http.Request, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer`)
	httpx.Error(w, r, http.StatusUnauthorized, msg)
}

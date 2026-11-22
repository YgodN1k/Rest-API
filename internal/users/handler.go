package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"rastapi/internal/auth"
	"rastapi/internal/httpx"
	"rastapi/internal/logger"
)

// Store — то, что нужно хендлерам от хранилища (*Repo подходит; в тестах — фейк).
type Store interface {
	Create(ctx context.Context, email, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
}

// TokenIssuer — выпуск JWT (*auth.Tokens подходит).
type TokenIssuer interface {
	Issue(userID int64, email string) (string, time.Time, error)
}

// Handler — HTTP-ручки /auth/register и /auth/login.
type Handler struct {
	store  Store
	tokens TokenIssuer
	// dummyHash сравнивается с паролем, когда пользователя нет,
	// чтобы время ответа не выдавало, зарегистрирован ли email.
	dummyHash string
}

// NewHandler создаёт хендлеры авторизации.
func NewHandler(store Store, tokens TokenIssuer) (*Handler, error) {
	dummy, err := auth.HashPassword("dummy-password-for-timing")
	if err != nil {
		return nil, err
	}
	return &Handler{store: store, tokens: tokens, dummyHash: dummy}, nil
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

const invalidCredentials = "invalid email or password"

// Register — POST /auth/register {email, password} → 201 {id, email}.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, err.Error())
		return
	}

	email := NormalizeEmail(req.Email)
	if err := validateEmail(email); err != nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := validatePassword(req.Password); err != nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.internalError(w, r, "hash password", err)
		return
	}

	u, err := h.store.Create(r.Context(), email, hash)
	switch {
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(w, r, http.StatusConflict, ErrEmailTaken.Error())
		return
	case err != nil:
		h.internalError(w, r, "create user", err)
		return
	}

	logger.FromContext(r.Context()).Info("user registered", "user_id", u.ID)
	httpx.JSON(w, r, http.StatusCreated, registerResponse{ID: u.ID, Email: u.Email})
}

// Login — POST /auth/login {email, password} → 200 {token, expires_at}.
// На «нет пользователя» и «неверный пароль» отвечаем одинаково (401),
// чтобы нельзя было выяснить, какие email зарегистрированы.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Password) > auth.MaxPasswordBytes {
		httpx.Error(w, r, http.StatusUnauthorized, invalidCredentials)
		return
	}

	u, err := h.store.GetByEmail(r.Context(), NormalizeEmail(req.Email))
	switch {
	case errors.Is(err, ErrNotFound):
		auth.CheckPassword(h.dummyHash, req.Password) // выравниваем время ответа
		httpx.Error(w, r, http.StatusUnauthorized, invalidCredentials)
		return
	case err != nil:
		h.internalError(w, r, "get user", err)
		return
	}

	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		httpx.Error(w, r, http.StatusUnauthorized, invalidCredentials)
		return
	}

	token, expiresAt, err := h.tokens.Issue(u.ID, u.Email)
	if err != nil {
		h.internalError(w, r, "issue token", err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, loginResponse{Token: token, ExpiresAt: expiresAt.UTC()})
}

// internalError логирует подробности, а клиенту отдаёт безликий 500.
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	logger.FromContext(r.Context()).Error(op, "err", err)
	httpx.Error(w, r, http.StatusInternalServerError, "internal server error")
}

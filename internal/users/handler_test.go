package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rastapi/internal/auth"
)

// fakeStore — хранилище в памяти вместо PostgreSQL.
type fakeStore struct {
	byEmail map[string]User
	nextID  int64
}

func newFakeStore() *fakeStore { return &fakeStore{byEmail: map[string]User{}} }

func (f *fakeStore) Create(_ context.Context, email, hash string) (User, error) {
	if _, ok := f.byEmail[email]; ok {
		return User{}, ErrEmailTaken
	}
	f.nextID++
	u := User{ID: f.nextID, Email: email, PasswordHash: hash, CreatedAt: time.Now()}
	f.byEmail[email] = u
	return u, nil
}

func (f *fakeStore) GetByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestHandler(t *testing.T) (*Handler, *auth.Tokens) {
	t.Helper()
	tokens := auth.NewTokens(testSecret, 15*time.Minute)
	h, err := NewHandler(newFakeStore(), tokens)
	require.NoError(t, err)
	return h, tokens
}

func do(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return rr
}

func TestRegister(t *testing.T) {
	h, _ := newTestHandler(t)

	rr := do(h.Register, `{"email":"  Alice@Example.COM ","password":"secret123"}`)
	require.Equal(t, http.StatusCreated, rr.Code)
	require.JSONEq(t, `{"id":1,"email":"alice@example.com"}`, rr.Body.String())

	// Тот же адрес в другом регистре — уже занят.
	rr = do(h.Register, `{"email":"ALICE@example.com","password":"secret123"}`)
	require.Equal(t, http.StatusConflict, rr.Code)
	require.JSONEq(t, `{"error":"email already registered"}`, rr.Body.String())
}

func TestRegister_Validation(t *testing.T) {
	h, _ := newTestHandler(t)

	cases := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"bad json", `{`, http.StatusBadRequest, "malformed JSON"},
		{"unknown field", `{"email":"a@b.c","password":"secret123","role":"admin"}`, http.StatusBadRequest, "unknown field"},
		{"no email", `{"password":"secret123"}`, http.StatusUnprocessableEntity, "email is required"},
		{"bad email", `{"email":"not-an-email","password":"secret123"}`, http.StatusUnprocessableEntity, "email is invalid"},
		{"email with name", `{"email":"Bob <bob@b.c>","password":"secret123"}`, http.StatusUnprocessableEntity, "email is invalid"},
		{"short password", `{"email":"a@b.c","password":"1234567"}`, http.StatusUnprocessableEntity, "at least 8"},
		{"long password", `{"email":"a@b.c","password":"` + strings.Repeat("p", 73) + `"}`, http.StatusUnprocessableEntity, "at most 72"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(h.Register, tc.body)
			require.Equal(t, tc.wantCode, rr.Code)
			require.Contains(t, rr.Body.String(), tc.wantErr)
		})
	}
}

func TestLogin(t *testing.T) {
	h, tokens := newTestHandler(t)
	require.Equal(t, http.StatusCreated, do(h.Register, `{"email":"bob@example.com","password":"secret123"}`).Code)

	rr := do(h.Login, `{"email":"BOB@example.com","password":"secret123"}`)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.WithinDuration(t, time.Now().Add(15*time.Minute), resp.ExpiresAt, 2*time.Second)

	// Выданный токен валиден и указывает на этого пользователя.
	id, err := tokens.Parse(resp.Token)
	require.NoError(t, err)
	require.Equal(t, auth.Identity{UserID: 1, Email: "bob@example.com"}, id)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	h, _ := newTestHandler(t)
	require.Equal(t, http.StatusCreated, do(h.Register, `{"email":"bob@example.com","password":"secret123"}`).Code)

	// «Неверный пароль» и «нет такого пользователя» неотличимы для клиента.
	wrongPass := do(h.Login, `{"email":"bob@example.com","password":"wrong-pass"}`)
	noUser := do(h.Login, `{"email":"nobody@example.com","password":"secret123"}`)

	for _, rr := range []*httptest.ResponseRecorder{wrongPass, noUser} {
		require.Equal(t, http.StatusUnauthorized, rr.Code)
		require.JSONEq(t, `{"error":"invalid email or password"}`, rr.Body.String())
	}
}

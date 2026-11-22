package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	require.NoError(t, err)
	require.NotEqual(t, "correct horse", hash)

	require.True(t, CheckPassword(hash, "correct horse"))
	require.False(t, CheckPassword(hash, "wrong horse"))
	require.False(t, CheckPassword("not-a-hash", "correct horse"))

	// Соль случайная — хеши одного пароля различаются.
	hash2, err := HashPassword("correct horse")
	require.NoError(t, err)
	require.NotEqual(t, hash, hash2)
}

func TestTokens_RoundTrip(t *testing.T) {
	tokens := NewTokens(testSecret, 15*time.Minute)
	tok, exp, err := tokens.Issue(42, "a@b.c")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(15*time.Minute), exp, 2*time.Second)

	id, err := tokens.Parse(tok)
	require.NoError(t, err)
	require.Equal(t, Identity{UserID: 42, Email: "a@b.c"}, id)
}

func TestTokens_Expired(t *testing.T) {
	tokens := NewTokens(testSecret, 15*time.Minute)
	tok, _, err := tokens.Issue(1, "a@b.c")
	require.NoError(t, err)

	tokens.now = func() time.Time { return time.Now().Add(16 * time.Minute) }
	_, err = tokens.Parse(tok)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokens_WrongSecret(t *testing.T) {
	tok, _, err := NewTokens(testSecret, time.Minute).Issue(1, "a@b.c")
	require.NoError(t, err)

	_, err = NewTokens(strings.Repeat("x", 32), time.Minute).Parse(tok)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokens_RejectsOtherAlgorithms(t *testing.T) {
	tokens := NewTokens(testSecret, time.Minute)
	c := claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}}

	// alg=none — классическая атака «токен без подписи».
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, c).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = tokens.Parse(none)
	require.ErrorIs(t, err, ErrInvalidToken)

	// HS512 тем же секретом — тоже отклоняем, разрешён только HS256.
	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, c).SignedString([]byte(testSecret))
	require.NoError(t, err)
	_, err = tokens.Parse(hs512)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestTokens_Garbage(t *testing.T) {
	tokens := NewTokens(testSecret, time.Minute)
	for _, tok := range []string{"", "abc", "a.b.c"} {
		_, err := tokens.Parse(tok)
		require.ErrorIs(t, err, ErrInvalidToken, tok)
	}
}

func TestRequireAuth(t *testing.T) {
	tokens := NewTokens(testSecret, time.Minute)
	valid, _, err := tokens.Issue(7, "u@x.io")
	require.NoError(t, err)

	var got Identity
	h := RequireAuth(tokens)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := IdentityFromContext(r.Context())
		require.True(t, ok)
		got = id
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + valid, http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
		{"bad token", "Bearer abc.def.ghi", http.StatusUnauthorized},
		{"valid", "Bearer " + valid, http.StatusNoContent},
		{"scheme case-insensitive", "bearer " + valid, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			require.Equal(t, tc.want, rr.Code)
			if tc.want == http.StatusUnauthorized {
				require.Equal(t, "Bearer", rr.Header().Get("WWW-Authenticate"))
				require.Contains(t, rr.Body.String(), `"error"`)
			} else {
				require.Equal(t, Identity{UserID: 7, Email: "u@x.io"}, got)
			}
		})
	}
}

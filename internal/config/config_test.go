package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DB_URL":     "postgres://user:pass@localhost:5432/auction?sslmode=disable",
		"JWT_SECRET": strings.Repeat("s", 32),
		"PORT":       "8080",
	}
}

func TestLoad_OK(t *testing.T) {
	cfg, err := load(envFrom(validEnv()))
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.Port)
	require.Equal(t, ":8080", cfg.Addr())
}

func TestLoad_Errors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(m map[string]string)
		wantErr string
	}{
		{"no db url", func(m map[string]string) { delete(m, "DB_URL") }, "DB_URL is required"},
		{"no secret", func(m map[string]string) { delete(m, "JWT_SECRET") }, "JWT_SECRET is required"},
		{"short secret", func(m map[string]string) { m["JWT_SECRET"] = "short" }, "at least 32"},
		{"no port", func(m map[string]string) { delete(m, "PORT") }, "PORT is required"},
		{"bad port", func(m map[string]string) { m["PORT"] = "abc" }, "PORT must be"},
		{"port out of range", func(m map[string]string) { m["PORT"] = "70000" }, "PORT must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			tc.mutate(env)
			_, err := load(envFrom(env))
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestLoad_AllErrorsAtOnce(t *testing.T) {
	_, err := load(envFrom(map[string]string{}))
	require.Error(t, err)
	require.ErrorContains(t, err, "DB_URL")
	require.ErrorContains(t, err, "JWT_SECRET")
	require.ErrorContains(t, err, "PORT")
}

package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealthz_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	Handler(fakePinger{})(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"status":"ok","db":"up"}`, rr.Body.String())
}

func TestHealthz_DBDown(t *testing.T) {
	rr := httptest.NewRecorder()
	Handler(fakePinger{err: errors.New("conn refused")})(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	require.JSONEq(t, `{"status":"unavailable","db":"down"}`, rr.Body.String())
}

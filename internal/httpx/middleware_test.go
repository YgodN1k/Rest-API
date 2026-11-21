package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"rastapi/internal/logger"
)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestRequestID_GeneratesAndPropagates(t *testing.T) {
	var buf bytes.Buffer
	var seen string
	h := RequestID(newTestLogger(&buf))(Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
		logger.FromContext(r.Context()).Info("inside handler")
		w.WriteHeader(http.StatusTeapot)
	})))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.NotEmpty(t, seen)
	require.Equal(t, seen, rr.Header().Get(RequestIDHeader))
	require.Equal(t, http.StatusTeapot, rr.Code)

	// request_id есть в каждой строке лога (и в хендлере, и в access-логе).
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	for _, line := range lines {
		var rec map[string]any
		require.NoError(t, json.Unmarshal(line, &rec))
		require.Equal(t, seen, rec["request_id"])
	}
}

func TestRequestID_UsesIncomingHeader(t *testing.T) {
	var buf bytes.Buffer
	h := RequestID(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "abc-123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, "abc-123", rr.Header().Get(RequestIDHeader))
}

func TestRecoverer(t *testing.T) {
	var buf bytes.Buffer
	h := RequestID(newTestLogger(&buf))(Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, rr.Body.String())
	require.Contains(t, buf.String(), "panic recovered")
}

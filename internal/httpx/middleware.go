package httpx

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"rastapi/internal/logger"
)

// RequestIDHeader — заголовок, в котором приходит/уходит request_id.
const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

// RequestIDFromContext возвращает request_id текущего запроса.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// RequestID берёт X-Request-ID из запроса (или генерирует uuid),
// кладёт его в context, в заголовок ответа и в логгер запроса.
func RequestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" || len(id) > 128 {
				id = uuid.NewString()
			}
			w.Header().Set(RequestIDHeader, id)

			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			ctx = logger.WithContext(ctx, base.With("request_id", id))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// statusRecorder запоминает статус и размер ответа для логов.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Hijack нужен для WebSocket (gorilla/websocket делает upgrade через Hijacker).
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	if s.status == 0 {
		s.status = http.StatusSwitchingProtocols
	}
	return h.Hijack()
}

// Unwrap позволяет http.ResponseController добраться до исходного writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Logger логирует каждый запрос: метод, путь, статус, длительность, request_id.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		logger.FromContext(r.Context()).Log(r.Context(), level, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

// Recoverer перехватывает панику в хендлере, логирует её и отдаёт 500.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // штатный способ прервать ответ — пробрасываем
			}
			logger.FromContext(r.Context()).Error("panic recovered",
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			Error(w, r, http.StatusInternalServerError, "internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}

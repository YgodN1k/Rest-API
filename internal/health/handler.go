// Package health — ручка GET /healthz.
package health

import (
	"context"
	"net/http"
	"time"

	"rastapi/internal/httpx"
	"rastapi/internal/logger"
)

// Pinger — всё, что умеет проверять доступность (pgxpool.Pool подходит).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler возвращает 200 {"status":"ok"}, если БД отвечает, иначе 503.
func Handler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			logger.FromContext(r.Context()).Warn("healthz: db ping failed", "err", err)
			httpx.JSON(w, r, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"db":     "down",
			})
			return
		}
		httpx.JSON(w, r, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
	}
}

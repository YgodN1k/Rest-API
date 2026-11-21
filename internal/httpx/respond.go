// Package httpx — общие HTTP-хелперы: middleware и JSON-ответы.
package httpx

import (
	"encoding/json"
	"net/http"

	"rastapi/internal/logger"
)

// ErrorResponse — единый формат ошибок API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// JSON пишет ответ с указанным статусом.
func JSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.FromContext(r.Context()).Error("write json response", "err", err)
	}
}

// Error пишет ошибку в едином формате.
func Error(w http.ResponseWriter, r *http.Request, status int, msg string) {
	JSON(w, r, status, ErrorResponse{Error: msg})
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rastapi/internal/config"
	"rastapi/internal/db"
	"rastapi/internal/health"
	"rastapi/internal/httpx"
)

const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("application stopped with error", "err", err)
		os.Exit(1)
	}
	log.Info("application stopped")
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Контекст отменяется по Ctrl+C (SIGINT) или SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info("connected to database")

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           newRouter(log, pool),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Ждём либо сигнала, либо падения сервера.
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	}

	// Повторный Ctrl+C во время shutdown — завершаемся сразу.
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	log.Info("http server stopped")

	// Сюда на следующих этапах добавится остановка воркера и WS-хаба.
	return nil
}

func newRouter(log *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID(log))
	r.Use(httpx.Logger)
	r.Use(httpx.Recoverer)

	r.Get("/healthz", health.Handler(pool))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method not allowed")
	})
	return r
}

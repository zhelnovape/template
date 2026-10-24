package main

import (
	"fmt"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhelnovape/template/api"
	"github.com/zhelnovape/template/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(), 
    	syscall.SIGINT, 
    	syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx); err != nil {
		stop()
		log.Fatalf("%v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	pool, err := initDatabase(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	defer pool.Close()

	log.Println("database connected and pinged successfully")

	if err := runServer(ctx, cfg, pool); err != nil {
		return fmt.Errorf("server run failed: %w", err)
	}

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func readyHandler(pool *pgxpool.Pool, queryTimeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
			return
		}

		writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
    	log.Printf("failed to encode response: %v", err)
	}
}

func initDatabase(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database on startup: %w", err)
	}

	return pool, nil
}

func runServer(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) error {
	r := chi.NewRouter()
	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler(pool, cfg.DB.QueryTimeout))

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	srvErr := make(chan error, 1)

    go func() {
		log.Printf("listening on %s", cfg.HTTP.Addr)
		srvErr <- srv.ListenAndServe()
    }()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received, stopping server...")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Printf("shutdown timeout %s exceeded, forcing exit", cfg.ShutdownTimeout)
			}
			return fmt.Errorf("server forced to shutdown: %w", err)
		}
		
		log.Println("server stopped gracefully")

	case err := <-srvErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("failed to start server: %w", err)
		}
	}

	return nil
}
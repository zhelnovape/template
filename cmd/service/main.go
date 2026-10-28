package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhelnovape/template/internal/config"
	"github.com/zhelnovape/template/internal/container"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	c, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}

	serveErr := serve(ctx, c.Server, c.Logger, cfg.ShutdownTimeout)

	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	c.Close(closeCtx)

	c.Logger.Info("service stopped")
	return serveErr
}

func serve(ctx context.Context, srv *http.Server, logger *slog.Logger, shutdownTimeout time.Duration) error {
	srvErr := make(chan error, 1)

	go func() {
		logger.Info("service started", "addr", srv.Addr)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Error("shutdown timeout exceeded, forcing exit", "timeout", shutdownTimeout)
			}
			return fmt.Errorf("server forced to shutdown: %w", err)
		}

		logger.Info("server stopped gracefully")

	case err := <-srvErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("failed to start server: %w", err)
		}
	}

	return nil
}

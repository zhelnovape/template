package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	defer c.Close()

	if err := serve(ctx, c.Server, cfg.ShutdownTimeout); err != nil {
		return fmt.Errorf("server run failed: %w", err)
	}

	return nil
}

func serve(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	srvErr := make(chan error, 1)

	go func() {
		log.Printf("listening on %s", srv.Addr)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received, stopping server...")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Printf("shutdown timeout %s exceeded, forcing exit", shutdownTimeout)
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

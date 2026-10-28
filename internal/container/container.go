package container

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhelnovape/template/internal/config"
	"github.com/zhelnovape/template/internal/http/health"
	"github.com/zhelnovape/template/internal/http/ready"
	createhttp "github.com/zhelnovape/template/internal/http/trip/create"
	finishhttp "github.com/zhelnovape/template/internal/http/trip/finish"
	gethttp "github.com/zhelnovape/template/internal/http/trip/get"
	"github.com/zhelnovape/template/internal/infra/clock"
	"github.com/zhelnovape/template/internal/infra/postgres"
	"github.com/zhelnovape/template/internal/infra/postgres/transaction"
	triprepo "github.com/zhelnovape/template/internal/repository/trip"
	createuc "github.com/zhelnovape/template/internal/usecase/trip/create"
	finishuc "github.com/zhelnovape/template/internal/usecase/trip/finish"
	getuc "github.com/zhelnovape/template/internal/usecase/trip/get"
)

type Container struct {
	Server *http.Server
	pool   *pgxpool.Pool
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	pool, err := postgres.New(ctx, cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}
	log.Println("database connected and pinged successfully")

	txManager := transaction.New(pool, pgx.ReadCommitted, cfg.DB.QueryTimeout)
	tripRepo := triprepo.New(pool, cfg.DB.QueryTimeout)
	clk := clock.New()

	createTrip := createuc.New(tripRepo, txManager, clk)
	getTrip := getuc.New(tripRepo)
	finishTrip := finishuc.New(tripRepo, txManager, clk)

	r := chi.NewRouter()
	r.Method(http.MethodGet, "/health", health.New())
	r.Method(http.MethodGet, "/ready", ready.New(pool, cfg.DB.QueryTimeout))
	r.Route("/api/v1/trips", func(r chi.Router) {
		r.Method(http.MethodPost, "/", createhttp.New(createTrip))
		r.Method(http.MethodGet, "/{tripId}", gethttp.New(getTrip))
		r.Method(http.MethodPost, "/{tripId}/finish", finishhttp.New(finishTrip))
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return &Container{Server: srv, pool: pool}, nil
}

func (c *Container) Close() {
	c.pool.Close()
}

package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"

	"github.com/zhelnovape/template/internal/config"
	"github.com/zhelnovape/template/internal/http/health"
	"github.com/zhelnovape/template/internal/http/middleware"
	"github.com/zhelnovape/template/internal/http/ready"
	createhttp "github.com/zhelnovape/template/internal/http/trip/create"
	finishhttp "github.com/zhelnovape/template/internal/http/trip/finish"
	gethttp "github.com/zhelnovape/template/internal/http/trip/get"
	"github.com/zhelnovape/template/internal/infra/clock"
	"github.com/zhelnovape/template/internal/infra/postgres"
	"github.com/zhelnovape/template/internal/infra/postgres/transaction"
	"github.com/zhelnovape/template/internal/infra/telemetry"
	triprepo "github.com/zhelnovape/template/internal/repository/trip"
	createuc "github.com/zhelnovape/template/internal/usecase/trip/create"
	finishuc "github.com/zhelnovape/template/internal/usecase/trip/finish"
	getuc "github.com/zhelnovape/template/internal/usecase/trip/get"
)

const (
	serviceName = "trip-service"
	meterName   = "github.com/zhelnovape/template"
)

type Container struct {
	Server    *http.Server
	Logger    *slog.Logger
	pool      *pgxpool.Pool
	telemetry *telemetry.Telemetry
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger, err := telemetry.NewLogger(os.Stdout, cfg.LogLevel, serviceName)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	tel, err := telemetry.New(ctx, cfg.Telemetry, serviceName, logger)
	if err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}
	meter := tel.MeterProvider.Meter(meterName)

	queryTracer, err := postgres.NewQueryTracer(tel.TracerProvider, tel.MeterProvider)
	if err != nil {
		return nil, errors.Join(err, tel.Shutdown(ctx))
	}

	pool, err := postgres.New(ctx, cfg.DB, queryTracer)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("database initialization failed: %w", err), tel.Shutdown(ctx))
	}
	logger.Info("database connected")

	c := &Container{Logger: logger, pool: pool, telemetry: tel}

	handler, err := newHandler(cfg, c, meter)
	if err != nil {
		c.Close(ctx)
		return nil, err
	}

	c.Server = &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return c, nil
}

func newHandler(cfg *config.Config, c *Container, meter metric.Meter) (http.Handler, error) {
	tripsCreated, err := meter.Int64Counter("tripgo.trips.created",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of created trips."),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips created counter: %w", err)
	}

	tripsCompleted, err := meter.Int64Counter("tripgo.trips.completed",
		metric.WithUnit("{trip}"),
		metric.WithDescription("Number of completed trips."),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips completed counter: %w", err)
	}

	txManager := transaction.New(c.pool, cfg.DB.QueryTimeout)
	tripRepo := triprepo.New(c.pool, cfg.DB.QueryTimeout)
	clk := clock.New()

	createTrip := createuc.New(tripRepo, txManager, clk, c.Logger, tripsCreated)
	getTrip := getuc.New(tripRepo)
	finishTrip := finishuc.New(tripRepo, txManager, clk, c.Logger, tripsCompleted)

	router := chi.NewRouter()
	router.Method(http.MethodGet, "/health", health.New())
	router.Method(http.MethodGet, "/ready", ready.New(c.pool, cfg.DB.QueryTimeout))
	router.Route("/api/v1/trips", func(r chi.Router) {
		r.Method(http.MethodPost, "/", createhttp.New(createTrip, c.Logger))
		r.Method(http.MethodGet, "/{tripId}", gethttp.New(getTrip, c.Logger))
		r.Method(http.MethodPost, "/{tripId}/finish", finishhttp.New(finishTrip, c.Logger))
	})

	metrics, err := middleware.Metrics(meter, router)
	if err != nil {
		return nil, err
	}

	return middleware.Chain(router,
		middleware.Recover(c.Logger),
		middleware.Tracing(c.telemetry.TracerProvider, c.telemetry.Propagator, router),
		middleware.Logging(c.Logger, router),
		metrics,
	), nil
}

func (c *Container) Close(ctx context.Context) {
	c.pool.Close()
	if err := c.telemetry.Shutdown(ctx); err != nil {
		c.Logger.Warn("telemetry shutdown failed", "error", err)
	}
}

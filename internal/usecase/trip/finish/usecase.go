package finish

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
	clock     clock
	logger    *slog.Logger
	completed metric.Int64Counter
}

func New(repo tripRepository, txManager txManager, clk clock, logger *slog.Logger, completed metric.Int64Counter) *UseCase {
	return &UseCase{repo: repo, txManager: txManager, clock: clk, logger: logger, completed: completed}
}

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (*trip.Trip, error) {
	var t *trip.Trip

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		t, err = u.repo.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("get trip: %w", err)
		}

		from := t.Status
		if err := t.Finish(u.clock.Now()); err != nil {
			return err
		}

		if err := u.repo.Finish(ctx, t); err != nil {
			return fmt.Errorf("finish trip: %w", err)
		}
		if err := u.repo.SaveStatusChange(ctx, t.ID, &from, t.Status, "trip finished"); err != nil {
			return fmt.Errorf("save status change: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	u.completed.Add(ctx, 1)
	u.logger.InfoContext(ctx, "trip completed", "trip_id", t.ID)

	return t, nil
}

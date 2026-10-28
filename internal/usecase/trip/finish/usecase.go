package finish

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
	clock     clock
}

func New(repo tripRepository, txManager txManager, clk clock) *UseCase {
	return &UseCase{repo: repo, txManager: txManager, clock: clk}
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

	return t, nil
}

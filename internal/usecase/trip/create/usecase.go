package create

import (
	"context"
	"fmt"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
	clock     clock
}

func New(repo tripRepository, txManager txManager, clk clock) *UseCase {
	return &UseCase{repo: repo, txManager: txManager, clock: clk}
}

func (u *UseCase) Execute(ctx context.Context, in model.CreateTrip) (*trip.Trip, error) {
	t, err := trip.New(in.UserID, in.DriverID, in.Start, in.End, in.Price, u.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("new trip: %w", err)
	}

	err = u.txManager.Do(ctx, func(ctx context.Context) error {
		if err := u.repo.Save(ctx, t); err != nil {
			return fmt.Errorf("save trip: %w", err)
		}
		if err := u.repo.SaveStatusChange(ctx, t.ID, nil, t.Status, "trip created"); err != nil {
			return fmt.Errorf("save status change: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return t, nil
}

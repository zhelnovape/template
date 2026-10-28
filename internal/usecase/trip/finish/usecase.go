package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
}

func New(repo tripRepository, txManager txManager) *UseCase {
	return &UseCase{repo: repo, txManager: txManager}
}

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var finishedTrip *model.Trip
	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finishedTrip, err = u.repo.Finish(ctx, id, now())
		if err != nil {
			return err
		}

		fromStatus := trip.StatusActive
		return u.repo.ChangeStatus(ctx, id, &fromStatus, trip.StatusCompleted, "trip finished")
	})
	if err != nil {
		return nil, err
	}
	return finishedTrip, nil
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

package create

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

func (u *UseCase) Execute(ctx context.Context, t *model.Trip) error {
	t.ID = uuid.New()
	t.Status = trip.StatusActive
	t.StartedAt = now()

	return u.txManager.Do(ctx, func(ctx context.Context) error {
		if err := u.repo.Save(ctx, t); err != nil {
			return err
		}

		return u.repo.ChangeStatus(ctx, t.ID, nil, trip.StatusActive, "trip created")
	})
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

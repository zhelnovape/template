package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/model"
)

type (
	tripRepository interface {
		Save(ctx context.Context, trip *model.Trip) error
		ChangeStatus(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus, reason string) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)

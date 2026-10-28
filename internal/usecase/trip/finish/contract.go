package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type (
	tripRepository interface {
		Get(ctx context.Context, id uuid.UUID) (*trip.Trip, error)
		Finish(ctx context.Context, t *trip.Trip) error
		SaveStatusChange(ctx context.Context, tripID uuid.UUID, from *trip.Status, to trip.Status, reason string) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	clock interface {
		Now() time.Time
	}
)

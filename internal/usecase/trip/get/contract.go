package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type tripRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*trip.Trip, error)
}

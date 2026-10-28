package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/model"
)

type tripRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

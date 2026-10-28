package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/model"
)

type tripGetter interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

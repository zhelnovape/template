package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/model"
)

type tripFinisher interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

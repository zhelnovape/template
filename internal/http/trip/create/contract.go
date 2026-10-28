package create

import (
	"context"

	"github.com/zhelnovape/template/internal/model"
)

type tripCreator interface {
	Execute(ctx context.Context, trip *model.Trip) error
}

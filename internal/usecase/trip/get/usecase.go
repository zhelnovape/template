package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/model"
)

type UseCase struct {
	repo tripRepository
}

func New(repo tripRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return u.repo.Get(ctx, id)
}

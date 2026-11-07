//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=${GOFILE} -destination=mocks_test.go -package=${GOPACKAGE}_test
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type tripRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*trip.Trip, error)
}

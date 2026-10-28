package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/model"
)

// Get возвращает поездку по id.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Select(tripColumns...).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select trip query: %w", err)
	}

	trip, err := scanTrip(r.conn(ctx).QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTripNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	return trip, nil
}

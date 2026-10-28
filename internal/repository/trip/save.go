package trip

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/model"
)

// Save добавляет новую поездку.
func (r *Repository) Save(ctx context.Context, trip *model.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartLatitude, trip.StartLongitude,
			trip.EndLatitude, trip.EndLongitude,
			trip.Price, trip.Status, trip.StartedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip query: %w", err)
	}

	if _, err := r.conn(ctx).Exec(ctx, query, args...); err != nil {
		if isActiveDriverViolation(err) {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("execute insert trip: %w", err)
	}
	return nil
}

// Finish переводит активную поездку в статус completed.
func (r *Repository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (*model.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Update("trips").
		Set("status", domain.StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "status": domain.StatusActive}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build finish trip query: %w", err)
	}

	trip, err := scanTrip(r.conn(ctx).QueryRow(ctx, query, args...))
	if err == nil {
		return trip, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("execute finish trip: %w", err)
	}

	if _, err := r.Get(ctx, id); err != nil {
		return nil, err
	}
	return nil, domain.ErrTripCompleted
}

// ChangeStatus записывает смену статуса в историю.
func (r *Repository) ChangeStatus(
	ctx context.Context,
	tripID uuid.UUID,
	fromStatus *string,
	toStatus string,
	reason string,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, fromStatus, toStatus, reason).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}

	if _, err := r.conn(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("execute insert status history: %w", err)
	}
	return nil
}

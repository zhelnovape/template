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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgUniqueViolation = "23505"
	activeDriverIndex = "trips_active_driver_idx"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var tripColumns = []string{
	"id", "user_id", "driver_id",
	"start_latitude", "start_longitude",
	"end_latitude", "end_longitude",
	"price", "status",
	"started_at", "finished_at",
	"created_at", "updated_at",
}

type TripRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *TripRepository) SrartTrip(ctx context.Context, trip *Trip) error {
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

	if _, err := executor(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		if isActiveDriverViolation(err) {
			return ErrDriverBusy
		}
		return fmt.Errorf("execute insert trip: %w", err)
	}
	return nil
}

func (r *TripRepository) SearchTrip(ctx context.Context, id uuid.UUID) (*Trip, error) {
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

	trip, err := scanTrip(executor(ctx, r.pool).QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	return trip, nil
}

func (r *TripRepository) FinishTrip(ctx context.Context, id uuid.UUID, finishedAt time.Time) (*Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Update("trips").
		Set("status", StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "status": StatusActive}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build finish trip query: %w", err)
	}

	trip, err := scanTrip(executor(ctx, r.pool).QueryRow(ctx, query, args...))
	if err == nil {
		return trip, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("execute finish trip: %w", err)
	}

	if _, err := r.SearchTrip(ctx, id); err != nil {
		return nil, err
	}
	return nil, ErrTripCompleted
}

func (r *TripRepository) ChangeStatus(
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

	if _, err := executor(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("execute insert status history: %w", err)
	}
	return nil
}

func scanTrip(row pgx.Row) (*Trip, error) {
	var trip Trip
	err := row.Scan(
		&trip.ID, &trip.UserID, &trip.DriverID,
		&trip.StartLatitude, &trip.StartLongitude,
		&trip.EndLatitude, &trip.EndLongitude,
		&trip.Price, &trip.Status,
		&trip.StartedAt, &trip.FinishedAt,
		&trip.CreatedAt, &trip.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func isActiveDriverViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgUniqueViolation &&
		pgErr.ConstraintName == activeDriverIndex
}

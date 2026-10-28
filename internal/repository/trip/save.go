package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/zhelnovape/template/internal/domain/trip"
)

func (r *Repository) Save(ctx context.Context, t *domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Insert(tripsTable).
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", statusColumn, "started_at",
		).
		Values(
			t.ID, t.UserID, t.DriverID,
			t.Start.Latitude, t.Start.Longitude,
			t.End.Latitude, t.End.Longitude,
			t.Price, string(t.Status), t.StartedAt,
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

func (r *Repository) Finish(ctx context.Context, t *domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Update(tripsTable).
		Set(statusColumn, string(t.Status)).
		Set("finished_at", t.FinishedAt).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": t.ID, statusColumn: string(domain.StatusActive)}).
		Suffix("RETURNING updated_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build finish trip query: %w", err)
	}

	err = r.conn(ctx).QueryRow(ctx, query, args...).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTripCompleted
	}
	if err != nil {
		return fmt.Errorf("execute finish trip: %w", err)
	}
	return nil
}

func (r *Repository) SaveStatusChange(
	ctx context.Context,
	tripID uuid.UUID,
	from *domain.Status,
	to domain.Status,
	reason string,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := psql.
		Insert(historyTable).
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, statusOrNil(from), string(to), reason).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}

	if _, err := r.conn(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("execute insert status history: %w", err)
	}
	return nil
}

package trip

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/zhelnovape/template/internal/infra/postgres/transaction"
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

type Repository struct {
	db           querier
	queryTimeout time.Duration
}

func New(db querier, queryTimeout time.Duration) *Repository {
	return &Repository{db: db, queryTimeout: queryTimeout}
}

func (r *Repository) conn(ctx context.Context) querier {
	if tx, ok := transaction.FromContext(ctx); ok {
		return tx
	}
	return r.db
}

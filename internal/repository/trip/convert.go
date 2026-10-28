package trip

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/zhelnovape/template/internal/model"
)

// scanTrip переводит строку таблицы trips в model.Trip.
func scanTrip(row pgx.Row) (*model.Trip, error) {
	var trip model.Trip
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

// isActiveDriverViolation проверяет, что вставка упала на индексе «одна активная поездка на водителя».
func isActiveDriverViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgUniqueViolation &&
		pgErr.ConstraintName == activeDriverIndex
}

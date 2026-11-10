package trip

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Point struct {
	Latitude  float64
	Longitude float64
}

type Trip struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DriverID uuid.UUID

	Start Point
	End   Point

	Price  int64
	Status Status

	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func New(userID, driverID uuid.UUID, start, end Point, price int64, startedAt time.Time) (*Trip, error) {
	t := &Trip{
		ID:        uuid.New(),
		UserID:    userID,
		DriverID:  driverID,
		Start:     start,
		End:       end,
		Price:     price,
		Status:    StatusActive,
		StartedAt: startedAt,
	}

	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Trip) Validate() error {
	if t.UserID == uuid.Nil {
		return &ValidationError{Field: "user_id", Reason: "must not be empty"}
	}
	if t.DriverID == uuid.Nil {
		return &ValidationError{Field: "driver_id", Reason: "must not be empty"}
	}
	if err := validatePoint("start_point", t.Start); err != nil {
		return err
	}
	if err := validatePoint("end_point", t.End); err != nil {
		return err
	}
	if t.Price < 0 {
		return &ValidationError{Field: "price", Reason: "must not be negative"}
	}

	switch t.Status {
	case StatusActive:
		if t.FinishedAt != nil {
			return &ValidationError{Field: "finished_at", Reason: "must be empty for an active trip"}
		}
	case StatusCompleted:
		if t.FinishedAt == nil {
			return &ValidationError{Field: "finished_at", Reason: "must be set for a completed trip"}
		}
	default:
		return &ValidationError{Field: "status", Reason: "unknown status"}
	}

	return nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at
	return nil
}

func validatePoint(field string, p Point) error {
	if p.Latitude < -90 || p.Latitude > 90 {
		return &ValidationError{Field: field + ".latitude", Reason: "must be between -90 and 90"}
	}
	if p.Longitude < -180 || p.Longitude > 180 {
		return &ValidationError{Field: field + ".longitude", Reason: "must be between -180 and 180"}
	}
	return nil
}

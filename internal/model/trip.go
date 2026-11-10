package model

import (
	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/domain/trip"
)

type CreateTrip struct {
	UserID   uuid.UUID
	DriverID uuid.UUID
	Start    trip.Point
	End      trip.Point
	Price    int64
}

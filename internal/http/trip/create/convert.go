package create

import (
	"github.com/google/uuid"

	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/model"
)

func toModel(req api.TripData) *model.Trip {
	return &model.Trip{
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  req.StartPoint.Latitude,
		StartLongitude: req.StartPoint.Longitude,
		EndLatitude:    req.EndPoint.Latitude,
		EndLongitude:   req.EndPoint.Longitude,
		Price:          req.Price,
	}
}

func validTripData(req api.TripData) bool {
	return req.UserId != uuid.Nil &&
		req.DriverId != uuid.Nil &&
		validCoordinates(req.StartPoint) &&
		validCoordinates(req.EndPoint) &&
		req.Price >= 0
}

func validCoordinates(c api.Coordinates) bool {
	return c.Latitude >= -90 && c.Latitude <= 90 &&
		c.Longitude >= -180 && c.Longitude <= 180
}

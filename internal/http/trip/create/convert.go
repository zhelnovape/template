package create

import (
	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/model"
)

func toModel(req api.TripData) model.CreateTrip {
	return model.CreateTrip{
		UserID:   req.UserId,
		DriverID: req.DriverId,
		Start:    toPoint(req.StartPoint),
		End:      toPoint(req.EndPoint),
		Price:    req.Price,
	}
}

func toPoint(c api.Coordinates) domain.Point {
	return domain.Point{Latitude: c.Latitude, Longitude: c.Longitude}
}

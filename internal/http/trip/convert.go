// Package trip содержит общее для ручек поездки: перевод model.Trip в ответ API.
package trip

import (
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/model"
)

// ToAPI переводит поездку в формат ответа API.
func ToAPI(t *model.Trip) api.Trip {
	return api.Trip{
		Id:         t.ID,
		UserId:     t.UserID,
		DriverId:   t.DriverID,
		StartPoint: api.Coordinates{Latitude: t.StartLatitude, Longitude: t.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: t.EndLatitude, Longitude: t.EndLongitude},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

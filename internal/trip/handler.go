package trip

import (
	"encoding/json"
	"errors"
	"net/http"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zhelnovape/template/api"
)

const problemBaseURL = "https://tripgo.example/problems/"

type TripHandler struct {
	service *TripService
}

func NewTripHandler(service *TripService) *TripHandler {
	return &TripHandler{service: service}
}

func (h *TripHandler) CreateTripHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req api.TripData
 
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || !validTripData(req) {
			writeInvalidRequest(w, r)
			return
		}
 
		trip := &Trip{
			UserID:         req.UserId,
			DriverID:       req.DriverId,
			StartLatitude:  req.StartPoint.Latitude,
			StartLongitude: req.StartPoint.Longitude,
			EndLatitude:    req.EndPoint.Latitude,
			EndLongitude:   req.EndPoint.Longitude,
			Price:          req.Price,
		}
 
		if err := h.service.CreateTrip(r.Context(), trip); err != nil {
			writeError(w, r, err)
			return
		}
 
		w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
		writeJSON(w, http.StatusCreated, toAPITrip(trip))
	}
}

func (h *TripHandler) GetTripHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tripId"))
		if err != nil {
			writeInvalidRequest(w, r)
			return
		}
 
		trip, err := h.service.GetTrip(r.Context(), id)
		if err != nil {
			writeError(w, r, err)
			return
		}
 
		writeJSON(w, http.StatusOK, toAPITrip(trip))
	}
}

func (h *TripHandler) FinishTripHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "tripId"))
		if err != nil {
			writeInvalidRequest(w, r)
			return
		}
 
		trip, err := h.service.FinishTrip(r.Context(), id)
		if err != nil {
			writeError(w, r, err)
			return
		}
 
		writeJSON(w, http.StatusOK, toAPITrip(trip))
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
 
func toAPITrip(t *Trip) api.Trip {
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

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrTripNotFound):
		writeProblem(w, r, http.StatusNotFound, "trip-not-found", "Trip not found", "Trip was not found", "trip_not_found")
	case errors.Is(err, ErrTripCompleted):
		writeProblem(w, r, http.StatusConflict, "trip-completed", "Trip completed", "Operation is not allowed for a completed trip", "trip_completed")
	case errors.Is(err, ErrDriverBusy):
		writeProblem(w, r, http.StatusConflict, "driver-busy", "Driver busy", "Driver already has an active trip", "driver_busy")
	default:
		log.Printf("internal error: %s %s: %v", r.Method, r.URL.Path, err)
		writeProblem(w, r, http.StatusInternalServerError, "internal-error", "Internal Server Error", "Internal server error", "internal_error")
	}
}
 
func writeInvalidRequest(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, http.StatusBadRequest, "invalid-request", "Invalid request", "Request validation failed", "invalid_request")
}
 
func writeProblem(w http.ResponseWriter, r *http.Request, status int, slug, title, detail, code string) {
	instance := r.URL.Path
	writeBody(w, status, "application/problem+json", api.Problem{
		Type:     problemBaseURL + slug,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &instance,
		Code:     code,
	})
}
 
func writeJSON(w http.ResponseWriter, status int, v any) {
	writeBody(w, status, "application/json", v)
}
 
func writeBody(w http.ResponseWriter, status int, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
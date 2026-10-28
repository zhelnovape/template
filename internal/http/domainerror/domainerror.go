package domainerror

import (
	"errors"
	"log"
	"net/http"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/response"
)

const problemBaseURL = "https://tripgo.example/problems/"

func Problem(err error) (int, api.Problem) {
	switch {
	case errors.Is(err, trip.ErrTripNotFound):
		return newProblem(http.StatusNotFound, "trip-not-found", "Trip not found", "Trip was not found", "trip_not_found")
	case errors.Is(err, trip.ErrTripCompleted):
		return newProblem(http.StatusConflict, "trip-completed", "Trip completed", "Operation is not allowed for a completed trip", "trip_completed")
	case errors.Is(err, trip.ErrDriverBusy):
		return newProblem(http.StatusConflict, "driver-busy", "Driver busy", "Driver already has an active trip", "driver_busy")
	default:
		return newProblem(http.StatusInternalServerError, "internal-error", "Internal Server Error", "Internal server error", "internal_error")
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, p := Problem(err)
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %s %s: %v", r.Method, r.URL.Path, err)
	}
	write(w, r, status, p)
}

func WriteInvalidRequest(w http.ResponseWriter, r *http.Request) {
	status, p := newProblem(http.StatusBadRequest, "invalid-request", "Invalid request", "Request validation failed", "invalid_request")
	write(w, r, status, p)
}

func write(w http.ResponseWriter, r *http.Request, status int, p api.Problem) {
	instance := r.URL.Path
	p.Instance = &instance
	response.Problem(w, status, p)
}

func newProblem(status int, slug, title, detail, code string) (int, api.Problem) {
	return status, api.Problem{
		Type:   problemBaseURL + slug,
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}

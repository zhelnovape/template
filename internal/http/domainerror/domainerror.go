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

func Problem(err error) (status int, body api.Problem) {
	var validationErr *trip.ValidationError

	switch {
	case errors.As(err, &validationErr):
		return newProblem(http.StatusBadRequest, "invalid-request", "Invalid request", validationErr.Error(), "invalid_request")
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
	status, body := Problem(err)
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %s %s: %v", r.Method, r.URL.Path, err)
	}

	instance := r.URL.Path
	body.Instance = &instance
	response.Problem(w, status, body)
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

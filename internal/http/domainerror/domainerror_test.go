package domainerror_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "validation error",
			err:        &trip.ValidationError{Field: "price", Reason: "must not be negative"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "invalid price: must not be negative",
		},
		{
			name:       "wrapped validation error",
			err:        fmt.Errorf("new trip: %w", &trip.ValidationError{Field: "user_id", Reason: "must not be empty"}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "invalid user_id: must not be empty",
		},
		{
			name:       "trip not found",
			err:        fmt.Errorf("get trip: %w", trip.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "trip completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name:       "driver busy",
			err:        fmt.Errorf("save trip: %w", trip.ErrDriverBusy),
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name:       "unknown error",
			err:        errors.New(`ERROR: relation "trips" does not exist (SQLSTATE 42P01)`),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, body := domainerror.Problem(tt.err)

			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantStatus, int(body.Status))
			assert.Equal(t, tt.wantCode, body.Code)
			require.NotNil(t, body.Detail)
			assert.Equal(t, tt.wantDetail, *body.Detail)
			assert.NotEmpty(t, body.Type)
			assert.NotEmpty(t, body.Title)
		})
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/trips/42", http.NoBody)

	domainerror.Write(w, r, errors.New("password authentication failed for user tripgo"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	assert.NotContains(t, w.Body.String(), "password")

	var body api.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Code)
	require.NotNil(t, body.Instance)
	assert.Equal(t, "/api/v1/trips/42", *body.Instance)
}

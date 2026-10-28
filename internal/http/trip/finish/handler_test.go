package finish_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/trip/finish"
)

func serve(t *testing.T, usecase *MocktripFinisher, tripID string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	router.Method(http.MethodPost, "/api/v1/trips/{tripId}/finish", finish.New(usecase))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID+"/finish", http.NoBody))
	return w
}

func decodeCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var problem api.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	return problem.Code
}

func TestHandler_OK(t *testing.T) {
	t.Parallel()

	finished, err := domain.New(
		uuid.New(),
		uuid.New(),
		domain.Point{Latitude: 56.8389, Longitude: 60.6057},
		domain.Point{Latitude: 56.7431, Longitude: 60.8027},
		1450,
		time.Now().UTC(),
	)
	require.NoError(t, err)
	require.NoError(t, finished.Finish(time.Now().UTC()))

	usecase := NewMocktripFinisher(gomock.NewController(t))
	usecase.EXPECT().Execute(gomock.Any(), finished.ID).Return(finished, nil)

	w := serve(t, usecase, finished.ID.String())

	assert.Equal(t, http.StatusOK, w.Code)
	var got api.Trip
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, finished.ID, got.Id)
	assert.Equal(t, api.TripStatus("completed"), got.Status)
	assert.NotNil(t, got.FinishedAt)
}

func TestHandler_InvalidID(t *testing.T) {
	t.Parallel()

	usecase := NewMocktripFinisher(gomock.NewController(t))

	w := serve(t, usecase, "42")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "invalid_request", decodeCode(t, w))
}

func TestHandler_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "not found",
			err:        fmt.Errorf("get trip: %w", domain.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "already completed",
			err:        domain.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "repository error",
			err:        errors.New("deadlock detected"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := uuid.New()
			usecase := NewMocktripFinisher(gomock.NewController(t))
			usecase.EXPECT().Execute(gomock.Any(), id).Return(nil, tt.err)

			w := serve(t, usecase, id.String())

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantCode, decodeCode(t, w))
		})
	}
}

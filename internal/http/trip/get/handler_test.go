package get_test

import (
	"encoding/json"
	"errors"
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
	"github.com/zhelnovape/template/internal/http/trip/get"
)

func serve(t *testing.T, usecase *MocktripGetter, tripID string) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	router.Method(http.MethodGet, "/api/v1/trips/{tripId}", get.New(usecase))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID, http.NoBody))
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

	existing, err := domain.New(
		uuid.New(),
		uuid.New(),
		domain.Point{Latitude: 56.8389, Longitude: 60.6057},
		domain.Point{Latitude: 56.7431, Longitude: 60.8027},
		1450,
		time.Now().UTC(),
	)
	require.NoError(t, err)

	usecase := NewMocktripGetter(gomock.NewController(t))
	usecase.EXPECT().Execute(gomock.Any(), existing.ID).Return(existing, nil)

	w := serve(t, usecase, existing.ID.String())

	assert.Equal(t, http.StatusOK, w.Code)
	var got api.Trip
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, existing.ID, got.Id)
	assert.Equal(t, existing.DriverID, got.DriverId)
}

func TestHandler_InvalidID(t *testing.T) {
	t.Parallel()

	usecase := NewMocktripGetter(gomock.NewController(t))

	w := serve(t, usecase, "not-a-uuid")

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
		{name: "not found", err: domain.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: "trip_not_found"},
		{name: "repository error", err: errors.New("timeout"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := uuid.New()
			usecase := NewMocktripGetter(gomock.NewController(t))
			usecase.EXPECT().Execute(gomock.Any(), id).Return(nil, tt.err)

			w := serve(t, usecase, id.String())

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantCode, decodeCode(t, w))
		})
	}
}

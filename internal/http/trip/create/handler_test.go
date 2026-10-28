package create_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/generated/api"
	"github.com/zhelnovape/template/internal/http/trip/create"
	"github.com/zhelnovape/template/internal/model"
)

const tripsURL = "/api/v1/trips"

var (
	userID   = uuid.MustParse("7c9e6679-7425-40de-944b-e07fc1f90ae7")
	driverID = uuid.MustParse("1b4e28ba-2fa1-11d2-883f-0016d3cca427")
)

const validBody = `{
	"user_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
	"driver_id": "1b4e28ba-2fa1-11d2-883f-0016d3cca427",
	"start_point": {"latitude": 56.8389, "longitude": 60.6057},
	"end_point": {"latitude": 56.7431, "longitude": 60.8027},
	"price": 1450
}`

func wantInput() model.CreateTrip {
	return model.CreateTrip{
		UserID:   userID,
		DriverID: driverID,
		Start:    domain.Point{Latitude: 56.8389, Longitude: 60.6057},
		End:      domain.Point{Latitude: 56.7431, Longitude: 60.8027},
		Price:    1450,
	}
}

func serve(t *testing.T, usecase *MocktripCreator, body string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, tripsURL, strings.NewReader(body))
	create.New(usecase, slog.New(slog.DiscardHandler)).ServeHTTP(w, r)
	return w
}

func decodeProblem(t *testing.T, w *httptest.ResponseRecorder) api.Problem {
	t.Helper()

	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var problem api.Problem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &problem))
	return problem
}

func TestHandler_Created(t *testing.T) {
	t.Parallel()

	in := wantInput()
	created, err := domain.New(in.UserID, in.DriverID, in.Start, in.End, in.Price, time.Now().UTC())
	require.NoError(t, err)

	usecase := NewMocktripCreator(gomock.NewController(t))
	usecase.EXPECT().Execute(gomock.Any(), in).Return(created, nil)

	w := serve(t, usecase, validBody)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, tripsURL+"/"+created.ID.String(), w.Header().Get("Location"))

	var got api.Trip
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.Id)
	assert.Equal(t, api.TripStatus("active"), got.Status)
	assert.Equal(t, int64(1450), got.Price)
}

func TestHandler_InvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "broken json", body: `{"user_id":`},
		{name: "unknown field", body: `{"passenger": "Ivan"}`},
		{name: "wrong type", body: `{"price": "free"}`},
		{name: "empty body", body: ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			usecase := NewMocktripCreator(gomock.NewController(t))

			w := serve(t, usecase, tt.body)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, "invalid_request", decodeProblem(t, w).Code)
		})
	}
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
			name:       "validation error",
			err:        fmt.Errorf("new trip: %w", &domain.ValidationError{Field: "price", Reason: "must not be negative"}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "driver busy",
			err:        fmt.Errorf("save trip: %w", domain.ErrDriverBusy),
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "repository error",
			err:        errors.New("dial tcp 10.0.0.5:5432: connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			usecase := NewMocktripCreator(gomock.NewController(t))
			usecase.EXPECT().Execute(gomock.Any(), wantInput()).Return(nil, tt.err)

			w := serve(t, usecase, validBody)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantCode, decodeProblem(t, w).Code)
		})
	}
}

func TestHandler_InternalErrorHidesDetails(t *testing.T) {
	t.Parallel()

	usecase := NewMocktripCreator(gomock.NewController(t))
	usecase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("dial tcp 10.0.0.5:5432: connection refused"))

	w := serve(t, usecase, validBody)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "10.0.0.5")
	assert.NotContains(t, w.Body.String(), "connection refused")
}

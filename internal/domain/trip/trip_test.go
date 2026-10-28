package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zhelnovape/template/internal/domain/trip"
)

var (
	ekaterinburg = trip.Point{Latitude: 56.8389, Longitude: 60.6057}
	airport      = trip.Point{Latitude: 56.7431, Longitude: 60.8027}
	startedAt    = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
)

func TestNew(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	driverID := uuid.New()

	tests := []struct {
		name      string
		userID    uuid.UUID
		driverID  uuid.UUID
		start     trip.Point
		end       trip.Point
		price     int64
		wantField string
	}{
		{
			name:     "valid trip",
			userID:   userID,
			driverID: driverID,
			start:    ekaterinburg,
			end:      airport,
			price:    1450,
		},
		{
			name:     "free trip",
			userID:   userID,
			driverID: driverID,
			start:    ekaterinburg,
			end:      airport,
		},
		{
			name:     "coordinates on the boundary",
			userID:   userID,
			driverID: driverID,
			start:    trip.Point{Latitude: 90, Longitude: 180},
			end:      trip.Point{Latitude: -90, Longitude: -180},
			price:    100,
		},
		{
			name:      "empty user",
			driverID:  driverID,
			start:     ekaterinburg,
			end:       airport,
			wantField: "user_id",
		},
		{
			name:      "empty driver",
			userID:    userID,
			start:     ekaterinburg,
			end:       airport,
			wantField: "driver_id",
		},
		{
			name:      "start latitude above 90",
			userID:    userID,
			driverID:  driverID,
			start:     trip.Point{Latitude: 90.1, Longitude: 0},
			end:       airport,
			wantField: "start_point.latitude",
		},
		{
			name:      "start longitude below -180",
			userID:    userID,
			driverID:  driverID,
			start:     trip.Point{Latitude: 0, Longitude: -180.1},
			end:       airport,
			wantField: "start_point.longitude",
		},
		{
			name:      "end latitude below -90",
			userID:    userID,
			driverID:  driverID,
			start:     ekaterinburg,
			end:       trip.Point{Latitude: -90.1, Longitude: 0},
			wantField: "end_point.latitude",
		},
		{
			name:      "end longitude above 180",
			userID:    userID,
			driverID:  driverID,
			start:     ekaterinburg,
			end:       trip.Point{Latitude: 0, Longitude: 180.1},
			wantField: "end_point.longitude",
		},
		{
			name:      "negative price",
			userID:    userID,
			driverID:  driverID,
			start:     ekaterinburg,
			end:       airport,
			price:     -1,
			wantField: "price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := trip.New(tt.userID, tt.driverID, tt.start, tt.end, tt.price, startedAt)

			if tt.wantField != "" {
				var validationErr *trip.ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Equal(t, tt.wantField, validationErr.Field)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, got.ID)
			assert.Equal(t, tt.userID, got.UserID)
			assert.Equal(t, tt.driverID, got.DriverID)
			assert.Equal(t, tt.start, got.Start)
			assert.Equal(t, tt.end, got.End)
			assert.Equal(t, tt.price, got.Price)
			assert.Equal(t, trip.StatusActive, got.Status)
			assert.Equal(t, startedAt, got.StartedAt)
			assert.Nil(t, got.FinishedAt)
		})
	}
}

func TestNew_GeneratesDifferentIDs(t *testing.T) {
	t.Parallel()

	first, err := trip.New(uuid.New(), uuid.New(), ekaterinburg, airport, 1450, startedAt)
	require.NoError(t, err)
	second, err := trip.New(uuid.New(), uuid.New(), ekaterinburg, airport, 1450, startedAt)
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
}

func TestTrip_Validate_Status(t *testing.T) {
	t.Parallel()

	finishedAt := startedAt.Add(30 * time.Minute)

	tests := []struct {
		name       string
		status     trip.Status
		finishedAt *time.Time
		wantField  string
	}{
		{name: "active without finished_at", status: trip.StatusActive},
		{name: "completed with finished_at", status: trip.StatusCompleted, finishedAt: &finishedAt},
		{name: "active with finished_at", status: trip.StatusActive, finishedAt: &finishedAt, wantField: "finished_at"},
		{name: "completed without finished_at", status: trip.StatusCompleted, wantField: "finished_at"},
		{name: "unknown status", status: trip.Status("cancelled"), wantField: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tr := newActiveTrip(t)
			tr.Status = tt.status
			tr.FinishedAt = tt.finishedAt

			err := tr.Validate()

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var validationErr *trip.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantField, validationErr.Field)
		})
	}
}

func TestTrip_Finish(t *testing.T) {
	t.Parallel()

	t.Run("active trip becomes completed", func(t *testing.T) {
		t.Parallel()

		tr := newActiveTrip(t)
		finishedAt := startedAt.Add(25 * time.Minute)

		err := tr.Finish(finishedAt)

		require.NoError(t, err)
		assert.Equal(t, trip.StatusCompleted, tr.Status)
		require.NotNil(t, tr.FinishedAt)
		assert.Equal(t, finishedAt, *tr.FinishedAt)
		assert.NoError(t, tr.Validate())
	})

	t.Run("completed trip cannot be finished again", func(t *testing.T) {
		t.Parallel()

		tr := newActiveTrip(t)
		firstFinish := startedAt.Add(25 * time.Minute)
		require.NoError(t, tr.Finish(firstFinish))

		err := tr.Finish(firstFinish.Add(time.Hour))

		require.ErrorIs(t, err, trip.ErrTripCompleted)
		assert.Equal(t, trip.StatusCompleted, tr.Status)
		assert.Equal(t, firstFinish, *tr.FinishedAt)
	})
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	err := &trip.ValidationError{Field: "price", Reason: "must not be negative"}

	assert.Equal(t, "invalid price: must not be negative", err.Error())
}

func newActiveTrip(t *testing.T) *trip.Trip {
	t.Helper()

	tr, err := trip.New(uuid.New(), uuid.New(), ekaterinburg, airport, 1450, startedAt)
	require.NoError(t, err)
	return tr
}

package finish_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/usecase/trip/finish"
)

var (
	startedAt  = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	finishedAt = startedAt.Add(25 * time.Minute)
)

type deps struct {
	repo      *MocktripRepository
	txManager *MocktxManager
	clock     *Mockclock
	metrics   *sdkmetric.ManualReader
}

func newUseCase(t *testing.T) (*finish.UseCase, deps) {
	t.Helper()

	ctrl := gomock.NewController(t)
	d := deps{
		repo:      NewMocktripRepository(ctrl),
		txManager: NewMocktxManager(ctrl),
		clock:     NewMockclock(ctrl),
		metrics:   sdkmetric.NewManualReader(),
	}

	completed, err := sdkmetric.NewMeterProvider(sdkmetric.WithReader(d.metrics)).
		Meter("test").
		Int64Counter("tripgo.trips.completed")
	require.NoError(t, err)

	d.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		})

	return finish.New(d.repo, d.txManager, d.clock, slog.New(slog.DiscardHandler), completed), d
}

func (d deps) completedTrips(t *testing.T) int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, d.metrics.Collect(context.Background(), &rm))

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}

func newActiveTrip(t *testing.T) *trip.Trip {
	t.Helper()

	tr, err := trip.New(
		uuid.New(),
		uuid.New(),
		trip.Point{Latitude: 56.8389, Longitude: 60.6057},
		trip.Point{Latitude: 56.7431, Longitude: 60.8027},
		1450,
		startedAt,
	)
	require.NoError(t, err)
	return tr
}

func TestUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	active := newActiveTrip(t)
	fromStatus := trip.StatusActive

	d.repo.EXPECT().Get(gomock.Any(), active.ID).Return(active, nil)
	d.clock.EXPECT().Now().Return(finishedAt)
	d.repo.EXPECT().
		Finish(gomock.Any(), active).
		DoAndReturn(func(_ context.Context, tr *trip.Trip) error {
			assert.Equal(t, trip.StatusCompleted, tr.Status)
			return nil
		})
	d.repo.EXPECT().
		SaveStatusChange(gomock.Any(), active.ID, &fromStatus, trip.StatusCompleted, "trip finished").
		Return(nil)

	got, err := uc.Execute(context.Background(), active.ID)

	require.NoError(t, err)
	assert.Equal(t, active.ID, got.ID)
	assert.Equal(t, trip.StatusCompleted, got.Status)
	require.NotNil(t, got.FinishedAt)
	assert.Equal(t, finishedAt, *got.FinishedAt)
	assert.Equal(t, int64(1), d.completedTrips(t))
}

func TestUseCase_Execute_TripNotFound(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	id := uuid.New()

	d.repo.EXPECT().Get(gomock.Any(), id).Return(nil, trip.ErrTripNotFound)

	got, err := uc.Execute(context.Background(), id)

	require.ErrorIs(t, err, trip.ErrTripNotFound)
	assert.Nil(t, got)
}

func TestUseCase_Execute_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	completed := newActiveTrip(t)
	require.NoError(t, completed.Finish(finishedAt))

	d.repo.EXPECT().Get(gomock.Any(), completed.ID).Return(completed, nil)
	d.clock.EXPECT().Now().Return(finishedAt.Add(time.Hour))

	got, err := uc.Execute(context.Background(), completed.ID)

	require.ErrorIs(t, err, trip.ErrTripCompleted)
	assert.Nil(t, got)
	assert.Equal(t, int64(0), d.completedTrips(t))
}

func TestUseCase_Execute_FinishedByAnotherRequest(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	active := newActiveTrip(t)

	d.repo.EXPECT().Get(gomock.Any(), active.ID).Return(active, nil)
	d.clock.EXPECT().Now().Return(finishedAt)
	d.repo.EXPECT().Finish(gomock.Any(), active).Return(trip.ErrTripCompleted)

	got, err := uc.Execute(context.Background(), active.ID)

	require.ErrorIs(t, err, trip.ErrTripCompleted)
	assert.Nil(t, got)
}

func TestUseCase_Execute_RepositoryErrors(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("connection refused")

	tests := []struct {
		name  string
		setup func(d deps, active *trip.Trip)
	}{
		{
			name: "get fails",
			setup: func(d deps, active *trip.Trip) {
				d.repo.EXPECT().Get(gomock.Any(), active.ID).Return(nil, repoErr)
			},
		},
		{
			name: "finish fails",
			setup: func(d deps, active *trip.Trip) {
				d.repo.EXPECT().Get(gomock.Any(), active.ID).Return(active, nil)
				d.clock.EXPECT().Now().Return(finishedAt)
				d.repo.EXPECT().Finish(gomock.Any(), active).Return(repoErr)
			},
		},
		{
			name: "status history fails",
			setup: func(d deps, active *trip.Trip) {
				d.repo.EXPECT().Get(gomock.Any(), active.ID).Return(active, nil)
				d.clock.EXPECT().Now().Return(finishedAt)
				d.repo.EXPECT().Finish(gomock.Any(), active).Return(nil)
				d.repo.EXPECT().
					SaveStatusChange(gomock.Any(), active.ID, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(repoErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, d := newUseCase(t)
			active := newActiveTrip(t)
			tt.setup(d, active)

			got, err := uc.Execute(context.Background(), active.ID)

			require.ErrorIs(t, err, repoErr)
			assert.Nil(t, got)
		})
	}
}

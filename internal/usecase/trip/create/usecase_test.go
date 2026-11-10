package create_test

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
	"github.com/zhelnovape/template/internal/model"
	"github.com/zhelnovape/template/internal/usecase/trip/create"
)

var now = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

type deps struct {
	repo      *MocktripRepository
	txManager *MocktxManager
	clock     *Mockclock
	metrics   *sdkmetric.ManualReader
}

func newUseCase(t *testing.T) (*create.UseCase, deps) {
	t.Helper()

	ctrl := gomock.NewController(t)
	d := deps{
		repo:      NewMocktripRepository(ctrl),
		txManager: NewMocktxManager(ctrl),
		clock:     NewMockclock(ctrl),
		metrics:   sdkmetric.NewManualReader(),
	}

	created, err := sdkmetric.NewMeterProvider(sdkmetric.WithReader(d.metrics)).
		Meter("test").
		Int64Counter("tripgo.trips.created")
	require.NoError(t, err)

	return create.New(d.repo, d.txManager, d.clock, slog.New(slog.DiscardHandler), created), d
}

func (d deps) createdTrips(t *testing.T) int64 {
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

func (d deps) expectTransaction() {
	d.txManager.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		})
}

func validInput() model.CreateTrip {
	return model.CreateTrip{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Start:    trip.Point{Latitude: 56.8389, Longitude: 60.6057},
		End:      trip.Point{Latitude: 56.7431, Longitude: 60.8027},
		Price:    1450,
	}
}

func TestUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	in := validInput()

	var saved *trip.Trip
	d.clock.EXPECT().Now().Return(now)
	d.expectTransaction()
	d.repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr *trip.Trip) error {
			saved = tr
			return nil
		})
	d.repo.EXPECT().
		SaveStatusChange(gomock.Any(), gomock.Any(), gomock.Nil(), trip.StatusActive, "trip created").
		Return(nil)

	got, err := uc.Execute(context.Background(), in)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Same(t, saved, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, in.UserID, got.UserID)
	assert.Equal(t, in.DriverID, got.DriverID)
	assert.Equal(t, in.Start, got.Start)
	assert.Equal(t, in.End, got.End)
	assert.Equal(t, in.Price, got.Price)
	assert.Equal(t, trip.StatusActive, got.Status)
	assert.Equal(t, now, got.StartedAt)
	assert.Equal(t, int64(1), d.createdTrips(t))
}

func TestUseCase_Execute_SavesHistoryForCreatedTrip(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)

	var savedID uuid.UUID
	d.clock.EXPECT().Now().Return(now)
	d.expectTransaction()
	d.repo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tr *trip.Trip) error {
			savedID = tr.ID
			return nil
		})
	d.repo.EXPECT().
		SaveStatusChange(gomock.Any(), gomock.Any(), gomock.Nil(), trip.StatusActive, gomock.Any()).
		DoAndReturn(func(_ context.Context, tripID uuid.UUID, _ *trip.Status, _ trip.Status, _ string) error {
			assert.Equal(t, savedID, tripID)
			return nil
		})

	_, err := uc.Execute(context.Background(), validInput())

	require.NoError(t, err)
}

func TestUseCase_Execute_DriverBusy(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)

	d.clock.EXPECT().Now().Return(now)
	d.expectTransaction()
	d.repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)

	got, err := uc.Execute(context.Background(), validInput())

	require.ErrorIs(t, err, trip.ErrDriverBusy)
	assert.Nil(t, got)
	assert.Equal(t, int64(0), d.createdTrips(t))
}

func TestUseCase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	in := validInput()
	in.Price = -1

	d.clock.EXPECT().Now().Return(now)

	got, err := uc.Execute(context.Background(), in)

	var validationErr *trip.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "price", validationErr.Field)
	assert.Nil(t, got)
}

func TestUseCase_Execute_RepositoryErrors(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("connection refused")

	tests := []struct {
		name  string
		setup func(d deps)
	}{
		{
			name: "save fails",
			setup: func(d deps) {
				d.repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(repoErr)
			},
		},
		{
			name: "status history fails",
			setup: func(d deps) {
				d.repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
				d.repo.EXPECT().
					SaveStatusChange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(repoErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, d := newUseCase(t)
			d.clock.EXPECT().Now().Return(now)
			d.expectTransaction()
			tt.setup(d)

			got, err := uc.Execute(context.Background(), validInput())

			require.ErrorIs(t, err, repoErr)
			assert.Nil(t, got)
		})
	}
}

func TestUseCase_Execute_TransactionFails(t *testing.T) {
	t.Parallel()

	uc, d := newUseCase(t)
	txErr := errors.New("begin transaction: timeout")

	d.clock.EXPECT().Now().Return(now)
	d.txManager.EXPECT().Do(gomock.Any(), gomock.Any()).Return(txErr)

	got, err := uc.Execute(context.Background(), validInput())

	require.ErrorIs(t, err, txErr)
	assert.Nil(t, got)
}

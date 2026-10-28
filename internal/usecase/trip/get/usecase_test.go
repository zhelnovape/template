package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/usecase/trip/get"
)

func TestUseCase_Execute(t *testing.T) {
	t.Parallel()

	existing, err := trip.New(
		uuid.New(),
		uuid.New(),
		trip.Point{Latitude: 56.8389, Longitude: 60.6057},
		trip.Point{Latitude: 56.7431, Longitude: 60.8027},
		1450,
		time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	repoErr := errors.New("connection refused")

	tests := []struct {
		name     string
		repoTrip *trip.Trip
		repoErr  error
		wantErr  error
	}{
		{name: "trip found", repoTrip: existing},
		{name: "trip not found", repoErr: trip.ErrTripNotFound, wantErr: trip.ErrTripNotFound},
		{name: "repository fails", repoErr: repoErr, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := NewMocktripRepository(gomock.NewController(t))
			repo.EXPECT().Get(gomock.Any(), existing.ID).Return(tt.repoTrip, tt.repoErr)

			got, err := get.New(repo).Execute(context.Background(), existing.ID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Same(t, existing, got)
		})
	}
}

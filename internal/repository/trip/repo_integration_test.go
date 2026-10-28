//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	domain "github.com/zhelnovape/template/internal/domain/trip"
	"github.com/zhelnovape/template/internal/infra/postgres/transaction"
	triprepo "github.com/zhelnovape/template/internal/repository/trip"
)

const (
	queryTimeout  = 3 * time.Second
	createdReason = "trip created"
)

type RepositorySuite struct {
	suite.Suite
	pool      *pgxpool.Pool
	repo      *triprepo.Repository
	txManager *transaction.Manager
}

func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositorySuite))
}

func (s *RepositorySuite) SetupSuite() {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	s.Require().NotEmpty(databaseURL, "set TEST_DATABASE_URL or DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), databaseURL)
	s.Require().NoError(err)
	s.Require().NoError(pool.Ping(context.Background()))

	s.pool = pool
	s.repo = triprepo.New(pool, queryTimeout)
	s.txManager = transaction.New(pool, queryTimeout)
}

func (s *RepositorySuite) TearDownSuite() {
	s.pool.Close()
}

func (s *RepositorySuite) newTrip() *domain.Trip {
	tr, err := domain.New(
		uuid.New(),
		uuid.New(),
		domain.Point{Latitude: 56.8389, Longitude: 60.6057},
		domain.Point{Latitude: 56.7431, Longitude: 60.8027},
		1450,
		time.Now().UTC().Truncate(time.Microsecond),
	)
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		_, err := s.pool.Exec(context.Background(), "DELETE FROM trips WHERE id = $1", tr.ID)
		s.NoError(err)
	})
	return tr
}

func (s *RepositorySuite) countHistory(tripID uuid.UUID) int {
	var count int
	err := s.pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM trip_status_history WHERE trip_id = $1",
		tripID,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *RepositorySuite) TestSaveAndGet() {
	ctx := context.Background()
	tr := s.newTrip()

	s.Require().NoError(s.repo.Save(ctx, tr))

	got, err := s.repo.Get(ctx, tr.ID)
	s.Require().NoError(err)
	s.Equal(tr.ID, got.ID)
	s.Equal(tr.UserID, got.UserID)
	s.Equal(tr.DriverID, got.DriverID)
	s.Equal(tr.Start, got.Start)
	s.Equal(tr.End, got.End)
	s.Equal(tr.Price, got.Price)
	s.Equal(domain.StatusActive, got.Status)
	s.True(tr.StartedAt.Equal(got.StartedAt))
	s.Nil(got.FinishedAt)
	s.False(got.CreatedAt.IsZero())
}

func (s *RepositorySuite) TestGetNotFound() {
	got, err := s.repo.Get(context.Background(), uuid.New())

	s.Require().ErrorIs(err, domain.ErrTripNotFound)
	s.Nil(got)
}

func (s *RepositorySuite) TestSaveDriverBusy() {
	ctx := context.Background()
	first := s.newTrip()
	second := s.newTrip()
	second.DriverID = first.DriverID

	s.Require().NoError(s.repo.Save(ctx, first))
	err := s.repo.Save(ctx, second)

	s.Require().ErrorIs(err, domain.ErrDriverBusy)
}

func (s *RepositorySuite) TestDriverIsFreeAfterFinish() {
	ctx := context.Background()
	first := s.newTrip()
	second := s.newTrip()
	second.DriverID = first.DriverID

	s.Require().NoError(s.repo.Save(ctx, first))
	s.Require().NoError(first.Finish(time.Now().UTC().Truncate(time.Microsecond)))
	s.Require().NoError(s.repo.Finish(ctx, first))

	s.Require().NoError(s.repo.Save(ctx, second))
}

func (s *RepositorySuite) TestFinish() {
	ctx := context.Background()
	tr := s.newTrip()
	s.Require().NoError(s.repo.Save(ctx, tr))

	finishedAt := time.Now().UTC().Truncate(time.Microsecond)
	s.Require().NoError(tr.Finish(finishedAt))
	s.Require().NoError(s.repo.Finish(ctx, tr))

	got, err := s.repo.Get(ctx, tr.ID)
	s.Require().NoError(err)
	s.Equal(domain.StatusCompleted, got.Status)
	s.Require().NotNil(got.FinishedAt)
	s.True(finishedAt.Equal(*got.FinishedAt))
	s.False(tr.UpdatedAt.IsZero())
}

func (s *RepositorySuite) TestFinishAlreadyCompleted() {
	ctx := context.Background()
	tr := s.newTrip()
	s.Require().NoError(s.repo.Save(ctx, tr))
	s.Require().NoError(tr.Finish(time.Now().UTC().Truncate(time.Microsecond)))
	s.Require().NoError(s.repo.Finish(ctx, tr))

	err := s.repo.Finish(ctx, tr)

	s.Require().ErrorIs(err, domain.ErrTripCompleted)
}

func (s *RepositorySuite) TestSaveStatusChange() {
	ctx := context.Background()
	tr := s.newTrip()
	s.Require().NoError(s.repo.Save(ctx, tr))

	err := s.repo.SaveStatusChange(ctx, tr.ID, nil, domain.StatusActive, createdReason)

	s.Require().NoError(err)
	s.Equal(1, s.countHistory(tr.ID))
}

func (s *RepositorySuite) TestTransactionCommit() {
	ctx := context.Background()
	tr := s.newTrip()

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Save(ctx, tr); err != nil {
			return err
		}
		return s.repo.SaveStatusChange(ctx, tr.ID, nil, tr.Status, createdReason)
	})

	s.Require().NoError(err)
	_, err = s.repo.Get(ctx, tr.ID)
	s.Require().NoError(err)
	s.Equal(1, s.countHistory(tr.ID))
}

func (s *RepositorySuite) TestTransactionRollbackOnError() {
	ctx := context.Background()
	tr := s.newTrip()
	errAfterInserts := errors.New("something went wrong after both inserts")

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Save(ctx, tr); err != nil {
			return err
		}
		if err := s.repo.SaveStatusChange(ctx, tr.ID, nil, tr.Status, createdReason); err != nil {
			return err
		}
		return errAfterInserts
	})

	s.Require().ErrorIs(err, errAfterInserts)
	_, err = s.repo.Get(ctx, tr.ID)
	s.Require().ErrorIs(err, domain.ErrTripNotFound)
	s.Equal(0, s.countHistory(tr.ID))
}

func (s *RepositorySuite) TestTransactionRollbackOnDatabaseError() {
	ctx := context.Background()
	tr := s.newTrip()

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Save(ctx, tr); err != nil {
			return err
		}
		return s.repo.SaveStatusChange(ctx, tr.ID, nil, domain.Status("unknown"), createdReason)
	})

	s.Require().Error(err)
	_, err = s.repo.Get(ctx, tr.ID)
	s.Require().ErrorIs(err, domain.ErrTripNotFound)
	s.Equal(0, s.countHistory(tr.ID))
}

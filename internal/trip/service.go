package trip

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type TripService struct {
	repo      *TripRepository
	txManager TxManager
}

func NewTripService(repo *TripRepository, txManager TxManager) *TripService {
	return &TripService{repo: repo, txManager: txManager}
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func (s *TripService) CreateTrip(ctx context.Context, trip *Trip) error {
	trip.ID = uuid.New()
	trip.Status = StatusActive
	trip.StartedAt = now()
 
	return s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.CreateTrip(ctx, trip); err != nil {
			return err
		}
 
		return s.repo.ChangeStatus(ctx, trip.ID, nil, StatusActive, "trip created")
	})
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (*Trip, error) {
	return s.repo.GetTrip(ctx, id)
}

func (s *TripService) FinishTrip(ctx context.Context, id uuid.UUID) (*Trip, error) {
	var finishedTrip *Trip
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finishedTrip, err = s.repo.FinishTrip(ctx, id, now())
		if err != nil {
			return err
		}
		
		fromStatus := StatusActive
		return s.repo.ChangeStatus(ctx, id, &fromStatus, StatusCompleted, "trip finished")
	})
	
	if err != nil {
		return nil, err
	}
	return finishedTrip, nil
}
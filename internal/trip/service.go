package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/EkLeRchEk/template/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo         *Repository
	txManager    *database.TxManager
	queryTimeout time.Duration
}

func NewService(
	repo *Repository,
	txManager *database.TxManager,
	queryTimeout time.Duration,
) *Service {
	return &Service{
		repo:         repo,
		txManager:    txManager,
		queryTimeout: queryTimeout,
	}
}

type CreateTripInput struct {
	UserID   string
	DriverID string

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

func (s *Service) CreateTrip(
	ctx context.Context,
	input CreateTripInput,
) (Trip, error) {
	now := time.Now().UTC()

	trip := Trip{
		ID:             uuid.NewString(),
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         StatusActive,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := s.txManager.WithinTransaction(
		ctx,
		func(ctx context.Context, tx pgx.Tx) error {
			repo := NewRepository(tx, s.queryTimeout)

			if err := repo.Create(ctx, trip); err != nil {
				return fmt.Errorf("create trip: %w", err)
			}

			history := StatusHistory{
				TripID:     trip.ID,
				FromStatus: nil,
				ToStatus:   StatusActive,
				Reason:     nil,
			}

			if err := repo.CreateStatusHistory(ctx, history); err != nil {
				return fmt.Errorf("create initial trip status: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return Trip{}, err
	}

	return trip, nil
}

func (s *Service) GetTrip(
	ctx context.Context,
	id string,
) (Trip, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) FinishTrip(
	ctx context.Context,
	id string,
) (Trip, error) {
	finishedAt := time.Now().UTC()

	err := s.txManager.WithinTransaction(
		ctx,
		func(ctx context.Context, tx pgx.Tx) error {
			repo := NewRepository(tx, s.queryTimeout)

			if err := repo.Finish(ctx, id, finishedAt); err != nil {
				return fmt.Errorf("finish trip: %w", err)
			}

			fromStatus := StatusActive

			history := StatusHistory{
				TripID:     id,
				FromStatus: &fromStatus,
				ToStatus:   StatusCompleted,
			}

			if err := repo.CreateStatusHistory(ctx, history); err != nil {
				return fmt.Errorf("create finish status history: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return Trip{}, err
	}

	return s.repo.GetByID(ctx, id)
}

package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/EkLeRchEk/template/internal/database"
	"github.com/google/uuid"
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

// Создание поездки и первой записи истории в одной транзакции.
func (s *Service) CreateTrip(
	ctx context.Context,
	input CreateTripInput,
) (Trip, error) {
	now := time.Now().UTC()

	newTrip := Trip{
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

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, newTrip); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		history := StatusHistory{
			TripID:   newTrip.ID,
			ToStatus: StatusActive,
		}

		if err := s.repo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create initial status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return Trip{}, err
	}

	return newTrip, nil
}

// Получение поездки по UUID.
func (s *Service) GetTrip(
	ctx context.Context,
	id string,
) (Trip, error) {
	return s.repo.GetByID(ctx, id)
}

// Завершение поездки и запись изменения статуса в одной транзакции.
func (s *Service) FinishTrip(
	ctx context.Context,
	id string,
) (Trip, error) {
	finishedAt := time.Now().UTC()

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Finish(ctx, id, finishedAt); err != nil {
			return fmt.Errorf("finish trip: %w", err)
		}

		fromStatus := StatusActive

		history := StatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   StatusCompleted,
		}

		if err := s.repo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create finish status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return Trip{}, err
	}

	return s.repo.GetByID(ctx, id)
}

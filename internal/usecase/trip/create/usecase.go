package create

import (
	"context"
	"fmt"
	"time"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Input содержит данные для создания поездки.
type Input struct {
	UserID   string
	DriverID string

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

// Usecase реализует создание поездки.
type Usecase struct {
	repo      tripRepository
	txManager txManager
}

// New создаёт usecase и передаёт ему зависимости.
func New(repo tripRepository, txManager txManager) *Usecase {
	return &Usecase{
		repo:      repo,
		txManager: txManager,
	}
}

// Execute создаёт поездку и запись истории атомарно.
func (u *Usecase) Execute(
	ctx context.Context,
	input Input,
) (model.Trip, error) {
	ctx, span := otel.Tracer("trip-service").Start(
		ctx,
		"CreateTrip",
	)
	defer span.End()

	now := time.Now().UTC()

	newTrip := model.Trip{
		ID:             uuid.NewString(),
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         domaintrip.StatusActive,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		// Сохраняем поездку.
		if err := u.repo.Create(ctx, newTrip); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		// Записываем первый статус поездки.
		history := model.StatusHistory{
			TripID:   newTrip.ID,
			ToStatus: domaintrip.StatusActive,
		}

		if err := u.repo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create initial status history: %w", err)
		}

		return nil
	})

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "CreateTrip failed")
		return model.Trip{}, err
	}

	return newTrip, nil
}

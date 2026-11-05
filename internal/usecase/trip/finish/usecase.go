package finish

import (
	"context"
	"fmt"
	"time"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type Usecase struct {
	repo      tripRepository
	txManager txManager
}

func New(repo tripRepository, txManager txManager) *Usecase {
	return &Usecase{
		repo:      repo,
		txManager: txManager,
	}
}

func (u *Usecase) Execute(ctx context.Context, id string) (model.Trip, error) {
	ctx, span := otel.Tracer("trip-service").Start(
		ctx,
		"FinishTrip",
	)
	defer span.End()

	var result model.Trip

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		existing, err := u.repo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("get trip for finish: %w", err)
		}

		// Проверяем переход статуса через доменную модель.
		domain := existing.ToDomain()

		finishedAt := time.Now().UTC()
		if err := domain.Finish(finishedAt); err != nil {
			return fmt.Errorf("finish trip domain rule: %w", err)
		}

		// Сохраняем новый статус поездки.
		if err := u.repo.Finish(ctx, id, finishedAt); err != nil {
			return fmt.Errorf("update trip: %w", err)
		}

		fromStatus := domaintrip.StatusActive

		history := model.StatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   domaintrip.StatusCompleted,
		}

		if err := u.repo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create finish history: %w", err)
		}

		existing.Status = domain.Status
		existing.FinishedAt = domain.FinishedAt
		existing.UpdatedAt = finishedAt
		result = existing

		return nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "FinishTrip failed")

		return model.Trip{}, err
	}

	return result, nil
}

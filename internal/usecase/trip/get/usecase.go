package get

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/EkLeRchEk/template/internal/model"
)

type Usecase struct {
	repo tripRepository
}

func New(repo tripRepository) *Usecase {
	return &Usecase{
		repo: repo,
	}
}

// Execute получает поездку по её идентификатору.
func (u *Usecase) Execute(
	ctx context.Context,
	id string,
) (model.Trip, error) {
	ctx, span := otel.Tracer("trip-service").Start(
		ctx,
		"GetTrip",
	)
	defer span.End()

	result, err := u.repo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "GetTrip failed")

		return model.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return result, nil
}

package list

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

// Execute получает историю координат поездки.
func (u *Usecase) Execute(
	ctx context.Context,
	tripID string,
) ([]model.TripPosition, error) {
	ctx, span := otel.Tracer("trip-service").Start(
		ctx,
		"ListTripPositions",
	)
	defer span.End()

	positions, err := u.repo.ListPositions(ctx, tripID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ListTripPositions failed")

		return nil, fmt.Errorf("list trip positions: %w", err)
	}

	return positions, nil
}

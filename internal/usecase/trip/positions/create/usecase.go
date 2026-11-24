package create

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/EkLeRchEk/template/internal/model"
)

type Usecase struct {
	repo      tripRepository
	tx        txManager
	canceller positionCanceller
	counter   metric.Int64Counter
}

func New(
	repo tripRepository,
	tx txManager,
	canceller positionCanceller,
) *Usecase {
	counter, _ := otel.Meter("trip-service").Int64Counter(
		"tripgo.positions.saved",
		metric.WithDescription("Number of saved trip positions"),
	)

	return &Usecase{
		repo:      repo,
		tx:        tx,
		canceller: canceller,
		counter:   counter,
	}
}

// Execute атомарно сохраняет координату и обновляет время последней позиции.
func (u *Usecase) Execute(
	ctx context.Context,
	input Input,
) (model.TripPosition, error) {
	ctx, span := otel.Tracer("trip-service").Start(
		ctx,
		"CreateTripPosition",
	)
	defer span.End()

	position := model.TripPosition{
		TripID:     input.TripID,
		Latitude:   input.Latitude,
		Longitude:  input.Longitude,
		RecordedAt: input.RecordedAt,
	}

	var saved model.TripPosition

	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		var err error
		saved, err = u.repo.SavePosition(txCtx, position)
		if err != nil {
			return fmt.Errorf("save trip position: %w", err)
		}

		return nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "save trip position failed")
		return model.TripPosition{}, err
	}

	// Новая координата делает предыдущий запрос ненужным.
	u.canceller.Cancel(input.TripID)

	u.counter.Add(ctx, 1)

	return saved, nil
}

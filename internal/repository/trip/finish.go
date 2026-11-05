package trip

import (
	"context"
	"fmt"
	"time"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	sq "github.com/Masterminds/squirrel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Finish завершает только активную поездку.
func (r *Repository) Finish(
	ctx context.Context,
	id string,
	finishedAt time.Time,
) error {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.Finish",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.
		Update("trips").
		Set("status", domaintrip.StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{
			"id":     id,
			"status": domaintrip.StatusActive,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("build finish trip query: %w", err)
	}

	result, err := r.executor(ctx).Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database update failed")

		return fmt.Errorf("finish trip: %w", err)
	}

	// Если ничего не обновилось, выясняем причину.
	if result.RowsAffected() == 0 {
		existing, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}

		if existing.Status == domaintrip.StatusCompleted {
			return domaintrip.ErrTripCompleted
		}

		return domaintrip.ErrTripNotFound
	}

	return nil
}

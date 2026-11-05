package trip

import (
	"context"
	"fmt"

	"github.com/EkLeRchEk/template/internal/model"
	sq "github.com/Masterminds/squirrel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// CreateStatusHistory сохраняет изменение статуса поездки.
func (r *Repository) CreateStatusHistory(
	ctx context.Context,
	history model.StatusHistory,
) error {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.CreateStatusHistory",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.
		Insert("trip_status_history").
		Columns(
			"trip_id",
			"from_status",
			"to_status",
			"reason",
		).
		Values(
			history.TripID,
			history.FromStatus,
			history.ToStatus,
			history.Reason,
		).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("build create status history query: %w", err)
	}

	_, err = r.executor(ctx).Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database insert failed")

		return fmt.Errorf("create status history: %w", err)
	}

	return nil
}

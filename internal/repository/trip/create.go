package trip

import (
	"context"
	"errors"
	"fmt"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Create сохраняет новую поездку в PostgreSQL.
func (r *Repository) Create(ctx context.Context, trip model.Trip) error {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.Create",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.
		Insert("trips").
		Columns(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
		).
		Values(
			trip.ID,
			trip.UserID,
			trip.DriverID,
			trip.StartLatitude,
			trip.StartLongitude,
			trip.EndLatitude,
			trip.EndLongitude,
			trip.Price,
			trip.Status,
			trip.StartedAt,
			trip.FinishedAt,
		).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	_, err = r.executor(ctx).Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database insert failed")

		var pgErr *pgconn.PgError

		// PostgreSQL: unique_violation.
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "trips_driver_active_unique_idx" {
			return domaintrip.ErrDriverBusy
		}

		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

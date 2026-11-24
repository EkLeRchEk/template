package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

// SavePosition сохраняет координату активной поездки.
func (r *Repository) SavePosition(
	ctx context.Context,
	position model.TripPosition,
) (model.TripPosition, error) {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.SavePosition",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	// Обновляем время последней координаты.
	// GREATEST не позволяет старой координате
	// уменьшить last_position_at.
	updateQuery, updateArgs, err := sq.
		Update("trips").
		Set(
			"last_position_at",
			sq.Expr(
				"GREATEST(COALESCE(last_position_at, ?), ?)",
				position.RecordedAt,
				position.RecordedAt,
			),
		).
		Where(sq.Eq{
			"id":     position.TripID,
			"status": domaintrip.StatusActive,
		}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return model.TripPosition{},
			fmt.Errorf("build update position query: %w", err)
	}

	result, err := r.executor(ctx).Exec(
		ctx,
		updateQuery,
		updateArgs...,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "update trip position failed")

		return model.TripPosition{},
			fmt.Errorf("update last position time: %w", err)
	}

	if result.RowsAffected() == 0 {
		existing, err := r.GetByID(ctx, position.TripID)
		if err != nil {
			return model.TripPosition{}, err
		}

		if existing.Status == domaintrip.StatusCompleted {
			return model.TripPosition{}, domaintrip.ErrTripCompleted
		}

		return model.TripPosition{}, domaintrip.ErrTripNotFound
	}

	// Сохраняем координату.
	query, args, err := sq.
		Insert("trip_positions").
		Columns(
			"trip_id",
			"latitude",
			"longitude",
			"recorded_at",
		).
		Values(
			position.TripID,
			position.Latitude,
			position.Longitude,
			position.RecordedAt,
		).
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return model.TripPosition{},
			fmt.Errorf("build insert position query: %w", err)
	}

	err = r.executor(ctx).
		QueryRow(ctx, query, args...).
		Scan(&position.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "insert position failed")

		return model.TripPosition{},
			fmt.Errorf("save trip position: %w", err)
	}

	return position, nil
}

// ListPositions получает историю координат поездки.
func (r *Repository) ListPositions(
	ctx context.Context,
	tripID string,
) ([]model.TripPosition, error) {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.ListPositions",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	// Проверяем существование поездки.
	if _, err := r.GetByID(ctx, tripID); err != nil {
		return nil, err
	}

	query, args, err := sq.
		Select(
			"id",
			"trip_id",
			"latitude",
			"longitude",
			"recorded_at",
		).
		From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions query: %w", err)
	}

	rows, err := r.executor(ctx).Query(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "select positions failed")

		return nil, fmt.Errorf("list trip positions: %w", err)
	}
	defer rows.Close()

	positions := make([]model.TripPosition, 0)

	for rows.Next() {
		var position model.TripPosition

		if err := rows.Scan(
			&position.ID,
			&position.TripID,
			&position.Latitude,
			&position.Longitude,
			&position.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trip position: %w", err)
		}

		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return positions, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "iterate positions failed")

		return nil, fmt.Errorf("iterate trip positions: %w", err)
	}

	return positions, nil
}

package trip

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

// ListStaleTrips возвращает активные поездки,
// по которым давно не поступали координаты.
func (r *Repository) ListStaleTrips(
	ctx context.Context,
	staleAfter time.Duration,
) ([]model.StaleTrip, error) {
	ctx, span := otel.Tracer("trip-service/repository").Start(
		ctx,
		"Repository.ListStaleTrips",
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	const query = `
		SELECT
			id,
			driver_id,
			COALESCE(last_position_at, started_at) AS stale_since
		FROM trips
		WHERE status = $1
		  AND COALESCE(last_position_at, started_at)
		      < NOW() - ($2::bigint * INTERVAL '1 microsecond')
		ORDER BY stale_since ASC, id ASC
	`

	rows, err := r.executor(ctx).Query(
		ctx,
		query,
		domaintrip.StatusActive,
		staleAfter.Microseconds(),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "select stale trips failed")
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()

	result := make([]model.StaleTrip, 0)

	for rows.Next() {
		var trip model.StaleTrip

		if err := rows.Scan(
			&trip.ID,
			&trip.DriverID,
			&trip.StaleSince,
		); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}

		result = append(result, trip)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "iterate stale trips failed")
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}

	return result, nil
}

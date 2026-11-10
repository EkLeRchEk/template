//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
	"github.com/EkLeRchEk/template/internal/model"
	"github.com/EkLeRchEk/template/internal/repository/trip"
)

func TestCreateRollbackIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := trip.New(pool, 5*time.Second)
	txManager := transaction.New(pool)

	tripID := uuid.NewString()
	driverID := uuid.NewString()
	userID := uuid.NewString()

	now := time.Now().UTC()

	newTrip := model.Trip{
		ID:             tripID,
		UserID:         userID,
		DriverID:       driverID,
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
		Status:         "active",
		StartedAt:      now,
	}

	expectedErr := errors.New("simulated history failure")

	err = txManager.Do(ctx, func(ctx context.Context) error {
		// Первая операция должна выполниться успешно.
		if err := repo.Create(ctx, newTrip); err != nil {
			return err
		}

		// Имитируем ошибку, которая произошла бы
		// при сохранении истории.
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected simulated error, got %v", err)
	}

	// После ROLLBACK поездки не должно быть в БД.
	var count int

	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM trips WHERE id = $1",
		tripID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("check rollback: %v", err)
	}

	if count != 0 {
		t.Fatalf("rollback failed: trip still exists")
	}
}

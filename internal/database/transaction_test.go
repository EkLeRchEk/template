package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/EkLeRchEk/template/internal/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestTransactionRollback(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := NewPostgres(ctx, config.DatabaseConfig{
		URL:             databaseURL,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	manager := NewTxManager(pool)

	// Генерируем уникальные UUID для тестовой поездки.
	tripID := uuid.NewString()
	userID := uuid.NewString()
	driverID := uuid.NewString()

	testErr := errors.New("simulated transaction failure")

	err = manager.WithinTransaction(
		ctx,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO trips (
					id, user_id, driver_id,
					start_latitude, start_longitude,
					end_latitude, end_longitude,
					price, status, started_at
				)
				VALUES (
					$1, $2, $3,
					59.9, 30.3,
					59.8, 30.4,
					1000, 'active', now()
				)
			`, tripID, userID, driverID)
			if err != nil {
				return err
			}

			// Имитируем ошибку после успешной записи.
			return testErr
		},
	)

	if !errors.Is(err, testErr) {
		t.Fatalf("expected simulated error, got %v", err)
	}

	// Проверяем, что поездка не сохранилась.
	var count int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM trips WHERE id = $1",
		tripID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count trips: %v", err)
	}

	if count != 0 {
		t.Fatalf("rollback failed: found %d trips", count)
	}
}

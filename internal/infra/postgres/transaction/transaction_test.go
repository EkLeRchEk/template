//go:build integration

package transaction

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/EkLeRchEk/template/internal/config"
	"github.com/EkLeRchEk/template/internal/infra/postgres"
)

// testPool создаёт подключение к тестовой БД
// и менеджер транзакций.
func testPool(t *testing.T) (*Manager, *pgxpool.Pool) {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := postgres.NewPostgres(ctx, config.DatabaseConfig{
		URL:             url,
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 30 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	t.Cleanup(pool.Close)

	return New(pool), pool
}

// TestTransactionRollback проверяет откат транзакции при ошибке.
func TestTransactionRollback(t *testing.T) {
	manager, pool := testPool(t)
	ctx := context.Background()

	tripID := uuid.NewString()
	userID := uuid.NewString()
	driverID := uuid.NewString()

	testErr := errors.New("simulated failure")

	err := manager.Do(ctx, func(ctx context.Context) error {
		tx := FromContext(ctx)
		if tx == nil {
			t.Fatal("transaction not found in context")
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO trips (
				id, user_id, driver_id,
				start_latitude, start_longitude,
				end_latitude, end_longitude,
				price, status, started_at
			)
			VALUES ($1, $2, $3, 59.9, 30.3, 59.8, 30.4,
				1000, 'active', now())
		`, tripID, userID, driverID)
		if err != nil {
			return err
		}

		// Имитируем ошибку после успешного INSERT.
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected simulated error, got %v", err)
	}

	// Проверяем, что запись не сохранилась в БД.
	var count int
	err = pool.QueryRow(
		context.Background(),
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

// TestNestedTransaction проверяет повторное
// использование существующей транзакции.
func TestNestedTransaction(t *testing.T) {
	manager, _ := testPool(t)

	err := manager.Do(context.Background(), func(ctx context.Context) error {
		outerTx := FromContext(ctx)
		if outerTx == nil {
			t.Fatal("outer transaction not found")
		}

		return manager.Do(ctx, func(ctx context.Context) error {
			innerTx := FromContext(ctx)

			if innerTx != outerTx {
				t.Fatal("nested transaction created a new transaction")
			}

			return nil
		})
	})
	if err != nil {
		t.Fatalf("nested transaction failed: %v", err)
	}
}

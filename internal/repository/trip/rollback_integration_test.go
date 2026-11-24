//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
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

	newTrip := model.Trip{
		ID:             tripID,
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
		Status:         "active",
		StartedAt:      time.Now().UTC(),
	}

	// Обе вставки выполняются в одной транзакции.
	err = txManager.Do(ctx, func(ctx context.Context) error {
		// Первая вставка: успешно создаём поездку.
		if err := repo.Create(ctx, newTrip); err != nil {
			return err
		}

		// Проверяем, что поездка видна внутри транзакции.
		if _, err := repo.GetByID(ctx, tripID); err != nil {
			return err
		}

		// Вторая вставка: реальный SQL-запрос.
		// PostgreSQL отклонит invalid_status,
		// поскольку CHECK разрешает только active/completed.
		return repo.CreateStatusHistory(ctx, model.StatusHistory{
			TripID:   tripID,
			ToStatus: "invalid_status",
		})
	})

	// Ошибка обязательно должна прийти от PostgreSQL.
	if err == nil {
		t.Fatal("expected database error, got nil")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	// 23514 = check_violation.
	if pgErr.Code != "23514" {
		t.Fatalf("expected SQLSTATE 23514, got %s", pgErr.Code)
	}

	// Проверяем первую таблицу.
	// После ROLLBACK поездка должна отсутствовать.
	var tripCount int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM trips WHERE id = $1",
		tripID,
	).Scan(&tripCount)
	if err != nil {
		t.Fatalf("check trips: %v", err)
	}

	if tripCount != 0 {
		t.Fatalf("rollback failed: found %d trips", tripCount)
	}

	// Проверяем вторую таблицу.
	// Записей истории тоже быть не должно.
	var historyCount int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1",
		tripID,
	).Scan(&historyCount)
	if err != nil {
		t.Fatalf("check trip status history: %v", err)
	}

	if historyCount != 0 {
		t.Fatalf("rollback failed: found %d history records", historyCount)
	}
}

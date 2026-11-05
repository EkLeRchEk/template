//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
	"github.com/EkLeRchEk/template/internal/repository/trip"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/create"
	finishuc "github.com/EkLeRchEk/template/internal/usecase/trip/finish"
)

func TestTripLifecycleIntegration(t *testing.T) {
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

	createUsecase := createuc.New(repo, txManager)
	finishUsecase := finishuc.New(repo, txManager)

	// Случайные идентификаторы исключают конфликт с другими поездками.
	input := createuc.Input{
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
	}

	created, err := createUsecase.Execute(ctx, input)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// Удаляем тестовые данные после выполнения.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE id = $1",
			created.ID,
		)
		if err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})

	if created.Status != domaintrip.StatusActive {
		t.Fatalf("expected active, got %s", created.Status)
	}

	// Проверяем запись истории после создания.
	var count int

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM trip_status_history
		WHERE trip_id = $1 AND to_status = 'active'
	`, created.ID).Scan(&count)
	if err != nil {
		t.Fatalf("check initial history: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 initial history, got %d", count)
	}

	// Завершаем поездку.
	finished, err := finishUsecase.Execute(ctx, created.ID)
	if err != nil {
		t.Fatalf("finish trip: %v", err)
	}

	if finished.Status != domaintrip.StatusCompleted {
		t.Fatalf("expected completed, got %s", finished.Status)
	}

	if finished.FinishedAt == nil {
		t.Fatal("finished_at is nil")
	}

	// Проверяем историю завершения.
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM trip_status_history
		WHERE trip_id = $1 AND to_status = 'completed'
	`, created.ID).Scan(&count)
	if err != nil {
		t.Fatalf("check finish history: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 finish history, got %d", count)
	}

	// Повторное завершение должно возвращать ошибку.
	_, err = finishUsecase.Execute(ctx, created.ID)

	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}
}

func TestConcurrentFinishIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	createUsecase := createuc.New(repo, txManager)
	finishUsecase := finishuc.New(repo, txManager)

	created, err := createUsecase.Execute(ctx, createuc.Input{
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cleanupCancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE id = $1",
			created.ID,
		); err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})

	var wg sync.WaitGroup

	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, err := finishUsecase.Execute(ctx, created.ID)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	completedCount := 0

	for err := range results {
		switch {
		case err == nil:
			successCount++

		case errors.Is(err, domaintrip.ErrTripCompleted):
			completedCount++

		default:
			t.Errorf("unexpected finish error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 successful finish, got %d", successCount)
	}

	if completedCount != 1 {
		t.Errorf("expected 1 completed error, got %d", completedCount)
	}

	var historyCount int

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM trip_status_history
		WHERE trip_id = $1
		  AND from_status = 'active'
		  AND to_status = 'completed'
	`, created.ID).Scan(&historyCount)
	if err != nil {
		t.Fatalf("count finish history: %v", err)
	}

	if historyCount != 1 {
		t.Fatalf("expected 1 finish history, got %d", historyCount)
	}
}

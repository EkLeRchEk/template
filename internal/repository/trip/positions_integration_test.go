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

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
	"github.com/EkLeRchEk/template/internal/model"
	triprepo "github.com/EkLeRchEk/template/internal/repository/trip"
)

// Создаёт подключение к PostgreSQL и необходимые зависимости.
func newPositionIntegrationFixture(
	t *testing.T,
) (context.Context, *pgxpool.Pool, *triprepo.Repository, *transaction.Manager) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := triprepo.New(pool, 5*time.Second)
	txManager := transaction.New(pool)

	return ctx, pool, repo, txManager
}

// Создаёт уникальную поездку и удаляет её после теста.
func createPositionTestTrip(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *triprepo.Repository,
	status domaintrip.Status,
	startedAt time.Time,
	finishedAt *time.Time,
) string {
	t.Helper()

	id := uuid.NewString()

	newTrip := model.Trip{
		ID:             id,
		UserID:         uuid.NewString(),
		DriverID:       uuid.NewString(),
		StartLatitude:  55.7558,
		StartLongitude: 37.6173,
		EndLatitude:    59.9398,
		EndLongitude:   30.3146,
		Price:          1500,
		Status:         status,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}

	if err := repo.Create(ctx, newTrip); err != nil {
		t.Fatalf("create test trip: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE id = $1",
			id,
		); err != nil {
			t.Errorf("cleanup trip %s: %v", id, err)
		}
	})

	return id
}

// Сохраняет координату внутри настоящей PostgreSQL-транзакции.
func savePositionInTransaction(
	t *testing.T,
	ctx context.Context,
	repo *triprepo.Repository,
	txManager *transaction.Manager,
	position model.TripPosition,
) model.TripPosition {
	t.Helper()

	var saved model.TripPosition

	err := txManager.Do(ctx, func(txCtx context.Context) error {
		var err error

		saved, err = repo.SavePosition(txCtx, position)

		return err
	})
	if err != nil {
		t.Fatalf("save position in transaction: %v", err)
	}

	return saved
}

// Проверяем сохранение координаты и обновление last_position_at.
func TestSavePositionIntegration(t *testing.T) {
	ctx, pool, repo, txManager := newPositionIntegrationFixture(t)

	tripID := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		time.Now().UTC().Add(-time.Hour),
		nil,
	)

	recordedAt := time.Now().UTC().Truncate(time.Microsecond)

	saved := savePositionInTransaction(
		t,
		ctx,
		repo,
		txManager,
		model.TripPosition{
			TripID:     tripID,
			Latitude:   55.7558,
			Longitude:  37.6173,
			RecordedAt: recordedAt,
		},
	)

	if saved.ID <= 0 {
		t.Fatalf("expected generated position ID, got %d", saved.ID)
	}

	if saved.TripID != tripID {
		t.Fatalf("unexpected trip ID: %s", saved.TripID)
	}

	// Проверяем непосредственно значение в таблице trips.
	var lastPositionAt time.Time

	err := pool.QueryRow(
		ctx,
		"SELECT last_position_at FROM trips WHERE id = $1",
		tripID,
	).Scan(&lastPositionAt)
	if err != nil {
		t.Fatalf("select last_position_at: %v", err)
	}

	if !lastPositionAt.Equal(recordedAt) {
		t.Fatalf(
			"unexpected last_position_at: got %v, want %v",
			lastPositionAt,
			recordedAt,
		)
	}

	// Проверяем, что координата доступна через Repository.
	positions, err := repo.ListPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}

	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	if positions[0].ID != saved.ID {
		t.Fatalf(
			"unexpected position ID: got %d, want %d",
			positions[0].ID,
			saved.ID,
		)
	}
}

// Проверяем, что завершённая поездка не принимает новые координаты.
func TestSavePositionCompletedTripIntegration(t *testing.T) {
	ctx, pool, repo, _ := newPositionIntegrationFixture(t)

	tripID := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		time.Now().UTC().Add(-time.Hour),
		nil,
	)

	finishedAt := time.Now().UTC()

	_, err := pool.Exec(
		ctx,
		`UPDATE trips
		 SET status = 'completed', finished_at = $1
		 WHERE id = $2`,
		finishedAt,
		tripID,
	)
	if err != nil {
		t.Fatalf("complete test trip: %v", err)
	}

	_, err = repo.SavePosition(ctx, model.TripPosition{
		TripID:     tripID,
		Latitude:   55.7558,
		Longitude:  37.6173,
		RecordedAt: time.Now().UTC(),
	})

	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf(
			"expected ErrTripCompleted, got %v",
			err,
		)
	}

	positions, err := repo.ListPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}

	if len(positions) != 0 {
		t.Fatalf("expected no positions, got %d", len(positions))
	}
}

// Проверяем сортировку координат по recorded_at и ID.
func TestListPositionsOrderingIntegration(t *testing.T) {
	ctx, pool, repo, txManager := newPositionIntegrationFixture(t)

	tripID := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		time.Now().UTC().Add(-time.Hour),
		nil,
	)

	baseTime := time.Now().UTC().
		Truncate(time.Microsecond).
		Add(-5 * time.Minute)

	times := []time.Time{
		baseTime.Add(2 * time.Minute),
		baseTime,
		baseTime,
	}

	savedIDs := make([]int64, 0, len(times))

	for i, recordedAt := range times {
		saved := savePositionInTransaction(
			t,
			ctx,
			repo,
			txManager,
			model.TripPosition{
				TripID:     tripID,
				Latitude:   55.0 + float64(i),
				Longitude:  37.0 + float64(i),
				RecordedAt: recordedAt,
			},
		)

		savedIDs = append(savedIDs, saved.ID)
	}

	positions, err := repo.ListPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}

	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(positions))
	}

	// Две координаты имеют одинаковое время.
	// При совпадении recorded_at первой должна идти
	// запись с меньшим ID.
	expectedIDs := []int64{
		savedIDs[1],
		savedIDs[2],
		savedIDs[0],
	}

	for i, position := range positions {
		if position.ID != expectedIDs[i] {
			t.Errorf(
				"position %d: got ID %d, want %d",
				i,
				position.ID,
				expectedIDs[i],
			)
		}
	}
}

// Проверяем поиск устаревших активных поездок.
func TestListStaleTripsIntegration(t *testing.T) {
	ctx, pool, repo, txManager := newPositionIntegrationFixture(t)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Активная поездка без координат более двух часов.
	staleWithoutPosition := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		now.Add(-2*time.Hour),
		nil,
	)

	// Активная поездка, созданная недавно.
	freshWithoutPosition := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		now.Add(-10*time.Minute),
		nil,
	)

	// Старая поездка, но координата поступила недавно.
	recentPositionTrip := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		now.Add(-3*time.Hour),
		nil,
	)

	recentPositionAt := now.Add(-10 * time.Minute)

	savePositionInTransaction(
		t,
		ctx,
		repo,
		txManager,
		model.TripPosition{
			TripID:     recentPositionTrip,
			Latitude:   55.0,
			Longitude:  37.0,
			RecordedAt: recentPositionAt,
		},
	)

	// Старая поездка со старой координатой.
	stalePositionTrip := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		now.Add(-3*time.Hour),
		nil,
	)

	stalePositionAt := now.Add(-2 * time.Hour)

	savePositionInTransaction(
		t,
		ctx,
		repo,
		txManager,
		model.TripPosition{
			TripID:     stalePositionTrip,
			Latitude:   56.0,
			Longitude:  38.0,
			RecordedAt: stalePositionAt,
		},
	)

	// Завершённая поездка не должна попасть в выборку.
	finishedAt := now.Add(-2 * time.Hour)

	completedTrip := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusCompleted,
		now.Add(-3*time.Hour),
		&finishedAt,
	)

	staleTrips, err := repo.ListStaleTrips(ctx, time.Hour)
	if err != nil {
		t.Fatalf("list stale trips: %v", err)
	}

	found := make(map[string]time.Time)

	for _, staleTrip := range staleTrips {
		found[staleTrip.ID] = staleTrip.StaleSince
	}

	// Поездка без координат должна использовать started_at.
	if got, ok := found[staleWithoutPosition]; !ok {
		t.Error("old trip without positions was not found")
	} else if !got.Equal(now.Add(-2 * time.Hour)) {
		t.Errorf("unexpected stale_since: %v", got)
	}

	// Поездка со старой координатой должна использовать
	// время последней координаты.
	if got, ok := found[stalePositionTrip]; !ok {
		t.Error("trip with stale position was not found")
	} else if !got.Equal(stalePositionAt) {
		t.Errorf(
			"unexpected stale_since: got %v, want %v",
			got,
			stalePositionAt,
		)
	}

	// Свежие координаты исключают поездку из выборки.
	if _, ok := found[freshWithoutPosition]; ok {
		t.Error("fresh trip without positions was incorrectly found")
	}

	if _, ok := found[recentPositionTrip]; ok {
		t.Error("trip with recent position was incorrectly found")
	}

	// Завершённые поездки не должны сканироваться.
	if _, ok := found[completedTrip]; ok {
		t.Error("completed trip was incorrectly found")
	}
}

// Проверяем атомарность обновления last_position_at
// и вставки координаты.
func TestSavePositionRollbackIntegration(t *testing.T) {
	ctx, pool, repo, txManager := newPositionIntegrationFixture(t)

	tripID := createPositionTestTrip(
		t,
		ctx,
		pool,
		repo,
		domaintrip.StatusActive,
		time.Now().UTC().Add(-time.Hour),
		nil,
	)

	recordedAt := time.Now().UTC()

	// Координата с широтой 100 нарушает CHECK:
	// latitude BETWEEN -90 AND 90.
	err := txManager.Do(ctx, func(txCtx context.Context) error {
		_, err := repo.SavePosition(txCtx, model.TripPosition{
			TripID:     tripID,
			Latitude:   100,
			Longitude:  37.6173,
			RecordedAt: recordedAt,
		})

		return err
	})

	if err == nil {
		t.Fatal("expected database constraint error")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	if pgErr.Code != "23514" {
		t.Fatalf(
			"expected SQLSTATE 23514, got %s",
			pgErr.Code,
		)
	}

	// Проверяем, что обновление времени тоже откатилось.
	var isNull bool

	err = pool.QueryRow(
		ctx,
		`SELECT last_position_at IS NULL
		 FROM trips
		 WHERE id = $1`,
		tripID,
	).Scan(&isNull)
	if err != nil {
		t.Fatalf("check last_position_at: %v", err)
	}

	if !isNull {
		t.Fatal("last_position_at was updated despite transaction rollback")
	}

	// Проверяем отсутствие частично сохранённых координат.
	var positionCount int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM trip_positions WHERE trip_id = $1",
		tripID,
	).Scan(&positionCount)
	if err != nil {
		t.Fatalf("count trip positions: %v", err)
	}

	if positionCount != 0 {
		t.Fatalf(
			"expected 0 positions after rollback, got %d",
			positionCount,
		)
	}
}

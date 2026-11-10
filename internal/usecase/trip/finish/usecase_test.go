package finish

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

// Создаём активную поездку для тестов.
func activeTrip() model.Trip {
	return model.Trip{
		ID:        "trip-123",
		Status:    domaintrip.StatusActive,
		StartedAt: time.Now().Add(-time.Hour),
	}
}

// Мок транзакции вызывает переданную функцию.
// Настоящую БД здесь не используем.
func expectTransaction(tx *MocktxManager) {
	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context,
			fn func(context.Context) error,
		) error {
			return fn(ctx)
		})
}

// Проверяем успешное завершение поездки.
func TestExecuteSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	trip := activeTrip()

	expectTransaction(tx)

	gomock.InOrder(
		repo.EXPECT().
			GetByID(gomock.Any(), trip.ID).
			Return(trip, nil),

		repo.EXPECT().
			Finish(gomock.Any(), trip.ID, gomock.Any()).
			Return(nil),

		repo.EXPECT().
			CreateStatusHistory(gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				history model.StatusHistory,
			) error {
				if history.TripID != trip.ID {
					t.Errorf("unexpected trip ID: %s", history.TripID)
				}

				if history.FromStatus == nil ||
					*history.FromStatus != domaintrip.StatusActive {
					t.Error("incorrect previous status")
				}

				if history.ToStatus != domaintrip.StatusCompleted {
					t.Error("incorrect new status")
				}

				return nil
			}),
	)

	usecase := New(repo, tx)

	result, err := usecase.Execute(context.Background(), trip.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != domaintrip.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	if result.FinishedAt == nil {
		t.Fatal("finished_at is nil")
	}
}

// Проверяем завершение уже завершённой поездки.
func TestExecuteAlreadyCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	trip := activeTrip()
	trip.Status = domaintrip.StatusCompleted

	expectTransaction(tx)

	repo.EXPECT().
		GetByID(gomock.Any(), trip.ID).
		Return(trip, nil)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), trip.ID)

	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}
}

// Проверяем отсутствие поездки.
func TestExecuteNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	expectTransaction(tx)

	repo.EXPECT().
		GetByID(gomock.Any(), "missing-trip").
		Return(model.Trip{}, domaintrip.ErrTripNotFound)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), "missing-trip")

	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound, got %v", err)
	}
}

// Проверяем ошибку обновления поездки.
func TestExecuteUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	expectedErr := errors.New("update failed")
	trip := activeTrip()

	expectTransaction(tx)

	gomock.InOrder(
		repo.EXPECT().
			GetByID(gomock.Any(), trip.ID).
			Return(trip, nil),

		repo.EXPECT().
			Finish(gomock.Any(), trip.ID, gomock.Any()).
			Return(expectedErr),
	)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), trip.ID)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected update error, got %v", err)
	}
}

// Проверяем ошибку записи истории.
func TestExecuteHistoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	expectedErr := errors.New("history failed")
	trip := activeTrip()

	expectTransaction(tx)

	gomock.InOrder(
		repo.EXPECT().
			GetByID(gomock.Any(), trip.ID).
			Return(trip, nil),

		repo.EXPECT().
			Finish(gomock.Any(), trip.ID, gomock.Any()).
			Return(nil),

		repo.EXPECT().
			CreateStatusHistory(gomock.Any(), gomock.Any()).
			Return(expectedErr),
	)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), trip.ID)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected history error, got %v", err)
	}
}

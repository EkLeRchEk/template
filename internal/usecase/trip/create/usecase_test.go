package create

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

func testInput() Input {
	return Input{
		UserID:         "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		DriverID:       "8860b315-ec86-42eb-a17c-7c163d721ff5",
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
	}
}

// Успешное создание поездки.
func TestExecuteSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context,
			fn func(context.Context) error,
		) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, trip model.Trip) error {
			if trip.ID == "" {
				t.Error("trip ID is empty")
			}
			if trip.Status != domaintrip.StatusActive {
				t.Errorf("unexpected status: %s", trip.Status)
			}
			return nil
		})

	repo.EXPECT().
		CreateStatusHistory(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, history model.StatusHistory) error {
			if history.ToStatus != domaintrip.StatusActive {
				t.Errorf("unexpected history status: %s", history.ToStatus)
			}
			return nil
		})

	usecase := New(repo, tx)

	result, err := usecase.Execute(context.Background(), testInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID == "" {
		t.Fatal("created trip has empty ID")
	}
}

// Ошибка сохранения поездки.
func TestExecuteRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	expectedErr := errors.New("database failure")

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context,
			fn func(context.Context) error,
		) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(expectedErr)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), testInput())

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected database error, got %v", err)
	}
}

// Ошибка записи истории статуса.
func TestExecuteHistoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	expectedErr := errors.New("history failure")

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context,
			fn func(context.Context) error,
		) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	repo.EXPECT().
		CreateStatusHistory(gomock.Any(), gomock.Any()).
		Return(expectedErr)

	usecase := New(repo, tx)

	_, err := usecase.Execute(context.Background(), testInput())

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected history error, got %v", err)
	}
}

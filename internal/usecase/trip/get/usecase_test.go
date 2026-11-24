package get

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

func TestExecuteSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)

	expected := model.Trip{
		ID:     "trip-123",
		Status: domaintrip.StatusActive,
	}

	repo.EXPECT().
		GetByID(gomock.Any(), "trip-123").
		Return(expected, nil)

	usecase := New(repo)

	result, err := usecase.Execute(context.Background(), "trip-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != expected.ID {
		t.Fatalf("expected ID %s, got %s", expected.ID, result.ID)
	}

	if result.Status != expected.Status {
		t.Fatalf("expected status %s, got %s", expected.Status, result.Status)
	}
}

func TestExecuteNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)

	repo.EXPECT().
		GetByID(gomock.Any(), "missing-trip").
		Return(model.Trip{}, domaintrip.ErrTripNotFound)

	usecase := New(repo)

	_, err := usecase.Execute(context.Background(), "missing-trip")

	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound, got %v", err)
	}
}

func TestExecuteRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)

	expectedErr := errors.New("database unavailable")

	repo.EXPECT().
		GetByID(gomock.Any(), "trip-123").
		Return(model.Trip{}, expectedErr)

	usecase := New(repo)

	_, err := usecase.Execute(context.Background(), "trip-123")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

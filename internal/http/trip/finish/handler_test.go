package finish

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/model"
)

const testTripID = "8a8055ef-2229-4922-a576-1565fd54ce64"

type fakeUsecase struct {
	execute func(context.Context, string) (model.Trip, error)
}

func (f fakeUsecase) Execute(
	ctx context.Context,
	id string,
) (model.Trip, error) {
	return f.execute(ctx, id)
}

func performRequest(uc fakeUsecase) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+testTripID+"/finish",
		nil,
	)

	rec := httptest.NewRecorder()
	id := api.TripId(uuid.MustParse(testTripID))

	New(uc).Handle(rec, req, id)

	return rec
}

func TestHandleSuccess(t *testing.T) {
	finishedAt := time.Now().UTC()

	uc := fakeUsecase{
		execute: func(_ context.Context, id string) (model.Trip, error) {
			if id != testTripID {
				t.Errorf("unexpected trip ID: %s", id)
			}

			return model.Trip{
				ID:       testTripID,
				UserID:   "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
				DriverID: "8860b315-ec86-42eb-a17c-7c163d721ff5",

				Status:     domaintrip.StatusCompleted,
				Price:      1450,
				StartedAt:  finishedAt.Add(-time.Hour),
				FinishedAt: &finishedAt,
			}, nil
		},
	}

	rec := performRequest(uc)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response api.Trip

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != api.TripStatus("completed") {
		t.Fatalf("unexpected status: %s", response.Status)
	}

	if response.FinishedAt == nil {
		t.Fatal("finished_at must not be nil")
	}
}

func TestHandleAlreadyCompleted(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, string) (model.Trip, error) {
			return model.Trip{}, domaintrip.ErrTripCompleted
		},
	}

	rec := performRequest(uc)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	var problem api.Problem

	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}

	if problem.Code != "trip_completed" {
		t.Fatalf("unexpected error code: %s", problem.Code)
	}
}

func TestHandleNotFound(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, string) (model.Trip, error) {
			return model.Trip{}, domaintrip.ErrTripNotFound
		},
	}

	rec := performRequest(uc)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleInternalError(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, string) (model.Trip, error) {
			return model.Trip{}, errors.New("database unavailable")
		},
	}

	rec := performRequest(uc)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

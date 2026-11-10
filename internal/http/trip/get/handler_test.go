package get

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
		http.MethodGet,
		"/api/v1/trips/8a8055ef-2229-4922-a576-1565fd54ce64",
		nil,
	)

	rec := httptest.NewRecorder()
	id := api.TripId(uuid.MustParse(
		"8a8055ef-2229-4922-a576-1565fd54ce64",
	))

	New(uc).Handle(rec, req, id)

	return rec
}

func TestHandleSuccess(t *testing.T) {
	const tripID = "8a8055ef-2229-4922-a576-1565fd54ce64"

	uc := fakeUsecase{
		execute: func(_ context.Context, id string) (model.Trip, error) {
			if id != tripID {
				t.Errorf("unexpected trip ID: %s", id)
			}

			return model.Trip{
				ID:        tripID,
				UserID:    "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
				DriverID:  "8860b315-ec86-42eb-a17c-7c163d721ff5",
				Status:    domaintrip.StatusActive,
				Price:     1450,
				StartedAt: time.Now().UTC(),
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

	if response.Id.String() != tripID {
		t.Fatalf("unexpected trip ID: %s", response.Id)
	}

	if response.Status != api.TripStatus("active") {
		t.Fatalf("unexpected status: %s", response.Status)
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

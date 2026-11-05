package create

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	"github.com/EkLeRchEk/template/internal/model"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/create"
)

// fakeUsecase заменяет настоящий CreateTrip Usecase.
type fakeUsecase struct {
	execute func(context.Context, createuc.Input) (model.Trip, error)
}

func (f fakeUsecase) Execute(
	ctx context.Context,
	input createuc.Input,
) (model.Trip, error) {
	return f.execute(ctx, input)
}

// validBody возвращает корректный JSON для создания поездки.
func validBody() string {
	return `{
		"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
		"start_point": {
			"latitude": 59.9398,
			"longitude": 30.3146
		},
		"end_point": {
			"latitude": 59.929,
			"longitude": 30.3626
		},
		"price": 1450
	}`
}

func performRequest(body string, uc fakeUsecase) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	New(uc).Handle(rec, req)

	return rec
}

func TestHandleSuccess(t *testing.T) {
	called := false

	uc := fakeUsecase{
		execute: func(_ context.Context, input createuc.Input) (model.Trip, error) {
			called = true

			if input.Price != 1450 {
				t.Errorf("unexpected price: %d", input.Price)
			}

			return model.Trip{
				ID:       "8a8055ef-2229-4922-a576-1565fd54ce64",
				UserID:   input.UserID,
				DriverID: input.DriverID,

				StartLatitude:  input.StartLatitude,
				StartLongitude: input.StartLongitude,
				EndLatitude:    input.EndLatitude,
				EndLongitude:   input.EndLongitude,

				Price:     input.Price,
				Status:    domaintrip.StatusActive,
				StartedAt: time.Now().UTC(),
			}, nil
		},
	}

	rec := performRequest(validBody(), uc)

	if !called {
		t.Fatal("usecase was not called")
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "/api/v1/trips/8a8055ef-2229-4922-a576-1565fd54ce64" {
		t.Fatalf("unexpected Location: %s", got)
	}

	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "active" {
		t.Fatalf("expected active, got %s", response.Status)
	}
}

func TestHandleMissingFields(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, createuc.Input) (model.Trip, error) {
			t.Fatal("usecase must not be called")
			return model.Trip{}, nil
		},
	}

	rec := performRequest(`{"price":1450}`, uc)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleInvalidCoordinates(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, createuc.Input) (model.Trip, error) {
			t.Fatal("usecase must not be called")
			return model.Trip{}, nil
		},
	}

	body := strings.Replace(
		validBody(),
		"59.9398",
		"120.0",
		1,
	)

	rec := performRequest(body, uc)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDriverBusy(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, createuc.Input) (model.Trip, error) {
			return model.Trip{}, domaintrip.ErrDriverBusy
		},
	}

	rec := performRequest(validBody(), uc)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	var problem struct {
		Code string `json:"code"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}

	if problem.Code != "driver_busy" {
		t.Fatalf("unexpected code: %s", problem.Code)
	}
}

func TestHandleInternalError(t *testing.T) {
	uc := fakeUsecase{
		execute: func(context.Context, createuc.Input) (model.Trip, error) {
			return model.Trip{}, errors.New("database unavailable")
		},
	}

	rec := performRequest(validBody(), uc)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

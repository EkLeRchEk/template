package httpadapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeDB заменяет PostgreSQL в тестах.
type fakeDB struct {
	err error
}

func (f *fakeDB) Ping(_ context.Context) error {
	return f.err
}

// Проверяем успешную работу /ready.
func TestReadySuccess(t *testing.T) {
	db := &fakeDB{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		db:     db,
		logger: logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	server.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// Проверяем недоступность PostgreSQL.
func TestReadyDatabaseUnavailable(t *testing.T) {
	db := &fakeDB{
		err: errors.New("database connection refused"),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := &Server{
		db:     db,
		logger: logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	server.Ready(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"unavailable"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}

	// Внутренняя ошибка PostgreSQL не должна попасть клиенту.
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatal("database error leaked into HTTP response")
	}
}

// Проверяем некорректный UUID в GET /api/v1/trips/{tripId}.
func TestGetTripInvalidUUID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := New(&fakeDB{}, logger, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/not-a-uuid",
		nil,
	)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// Проверяем некорректный UUID при завершении поездки.
func TestFinishTripInvalidUUID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := New(&fakeDB{}, logger, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/not-a-uuid/finish",
		nil,
	)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

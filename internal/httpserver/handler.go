package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/trip"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	service *trip.Service
	db      *pgxpool.Pool
}

var _ api.ServerInterface = (*Handler)(nil)

func NewHandler(service *trip.Service, db *pgxpool.Pool) *Handler {
	return &Handler{
		service: service,
		db:      db,
	}
}

// Отправка JSON с указанным HTTP-статусом.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Ошибки в формате Problem Details.
func writeProblem(
	w http.ResponseWriter,
	status int,
	code string,
	title string,
) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(api.Problem{
		Type:   "about:blank",
		Title:  title,
		Status: int32(status),
		Code:   code,
	})
}

// Преобразуем внутреннюю модель в OpenAPI-модель.
func toAPITrip(t trip.Trip) api.Trip {
	return api.Trip{
		Id:       uuid.MustParse(t.ID),
		UserId:   uuid.MustParse(t.UserID),
		DriverId: uuid.MustParse(t.DriverID),

		StartPoint: api.Coordinates{
			Latitude:  t.StartLatitude,
			Longitude: t.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndLatitude,
			Longitude: t.EndLongitude,
		},

		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

// POST /api/v1/trips
func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	_ = params // Идемпотентность относится к дополнительному заданию.

	var body api.TripData

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	if body.Price < 0 ||
		body.StartPoint.Latitude < -90 ||
		body.StartPoint.Latitude > 90 ||
		body.EndPoint.Latitude < -90 ||
		body.EndPoint.Latitude > 90 ||
		body.StartPoint.Longitude < -180 ||
		body.StartPoint.Longitude > 180 ||
		body.EndPoint.Longitude < -180 ||
		body.EndPoint.Longitude > 180 {
		writeProblem(w, http.StatusBadRequest, "bad_request", "Invalid trip data")
		return
	}

	input := trip.CreateTripInput{
		UserID:         body.UserId.String(),
		DriverID:       body.DriverId.String(),
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	}

	created, err := h.service.CreateTrip(r.Context(), input)
	if err != nil {
		log.Printf("CreateTrip failed: %+v", err)

		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeProblem(w, http.StatusConflict, "active_trip_conflict", "Driver already has an active trip")
			return
		}

		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", created.ID))
	writeJSON(w, http.StatusCreated, toAPITrip(created))
}

// GET /api/v1/trips/{tripId}
func (h *Handler) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripId api.TripId,
) {
	found, err := h.service.GetTrip(r.Context(), tripId.String())
	if err != nil {
		if errors.Is(err, trip.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Trip not found")
			return
		}

		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(found))
}

// POST /api/v1/trips/{tripId}/finish
func (h *Handler) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripId api.TripId,
) {
	finished, err := h.service.FinishTrip(r.Context(), tripId.String())

	if errors.Is(err, trip.ErrTripCompleted) {
		writeProblem(
			w,
			http.StatusConflict,
			"trip_completed",
			"Trip already completed",
		)
		return
	}

	if err != nil {
		if errors.Is(err, trip.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Trip not found or not active")
			return
		}

		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(finished))
}

// GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{
		Status: api.HealthResponseStatus("ok"),
	})
}

// GET /ready
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "not_ready", "Database unavailable")
		return
	}

	writeJSON(w, http.StatusOK, api.HealthResponse{
		Status: api.HealthResponseStatus("ok"),
	})
}

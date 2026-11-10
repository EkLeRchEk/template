package create

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"

	"github.com/google/uuid"

	"github.com/EkLeRchEk/template/internal/http/domainerror"
	triphttp "github.com/EkLeRchEk/template/internal/http/trip"
	"github.com/EkLeRchEk/template/internal/model"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/create"
)

// Интерфейс описывает только нужную обработчику операцию.
type usecase interface {
	Execute(ctx context.Context, input createuc.Input) (model.Trip, error)
}

type Handler struct {
	usecase usecase
}

func New(uc usecase) *Handler {
	return &Handler{usecase: uc}
}

type requestCoordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type requestBody struct {
	UserID     *uuid.UUID          `json:"user_id"`
	DriverID   *uuid.UUID          `json:"driver_id"`
	StartPoint *requestCoordinates `json:"start_point"`
	EndPoint   *requestCoordinates `json:"end_point"`
	Price      *int64              `json:"price"`
}

// Handle обрабатывает создание поездки.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	// Проверяем тип содержимого.
	contentType, _, err := mime.ParseMediaType(
		r.Header.Get("Content-Type"),
	)
	if err != nil || contentType != "application/json" {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Expected application/json",
		)
		return
	}

	// Ограничиваем тело запроса одним MiB.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var body requestBody

	if err := decoder.Decode(&body); err != nil {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Invalid JSON",
		)
		return
	}

	// Не допускаем второй JSON-объект.
	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Unexpected extra JSON",
		)
		return
	}

	// Проверяем наличие обязательных полей.
	if body.UserID == nil ||
		body.DriverID == nil ||
		body.StartPoint == nil ||
		body.EndPoint == nil ||
		body.Price == nil {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Missing required fields",
		)
		return
	}

	start := body.StartPoint
	end := body.EndPoint

	if start.Latitude == nil ||
		start.Longitude == nil ||
		end.Latitude == nil ||
		end.Longitude == nil {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Missing required fields",
		)
		return
	}

	// Проверяем диапазоны координат.
	validCoordinate := func(value float64, min, max float64) bool {
		return !math.IsNaN(value) &&
			!math.IsInf(value, 0) &&
			value >= min &&
			value <= max
	}

	if *body.Price < 0 ||
		!validCoordinate(*start.Latitude, -90, 90) ||
		!validCoordinate(*start.Longitude, -180, 180) ||
		!validCoordinate(*end.Latitude, -90, 90) ||
		!validCoordinate(*end.Longitude, -180, 180) {
		domainerror.WriteProblem(
			w, http.StatusBadRequest,
			"invalid_request", "Invalid trip data",
		)
		return
	}

	// Преобразуем HTTP-данные во входную модель usecase.
	input := createuc.Input{
		UserID:         body.UserID.String(),
		DriverID:       body.DriverID.String(),
		StartLatitude:  *start.Latitude,
		StartLongitude: *start.Longitude,
		EndLatitude:    *end.Latitude,
		EndLongitude:   *end.Longitude,
		Price:          *body.Price,
	}

	// Вызываем бизнес-логику.
	created, err := h.usecase.Execute(r.Context(), input)
	if err != nil {
		domainerror.Write(w, err)
		return
	}

	// Возвращаем созданную поездку.
	w.Header().Set(
		"Location",
		fmt.Sprintf("/api/v1/trips/%s", created.ID),
	)

	triphttp.WriteJSON(
		w,
		http.StatusCreated,
		triphttp.ToAPITrip(created),
	)
}

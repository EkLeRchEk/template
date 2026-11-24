package create

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"mime"
	"net/http"
	"time"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/http/domainerror"
	triphttp "github.com/EkLeRchEk/template/internal/http/trip"
	"github.com/EkLeRchEk/template/internal/model"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/positions/create"
)

type usecase interface {
	Execute(
		ctx context.Context,
		input createuc.Input,
	) (model.TripPosition, error)
}

type Handler struct {
	usecase usecase
}

func New(uc usecase) *Handler {
	return &Handler{usecase: uc}
}

type coordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type requestBody struct {
	Coordinates *coordinates `json:"coordinates"`
	RecordedAt  *time.Time   `json:"recorded_at"`
}

// Handle сохраняет координату поездки.
func (h *Handler) Handle(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	contentType, _, err := mime.ParseMediaType(
		r.Header.Get("Content-Type"),
	)

	if err != nil || contentType != "application/json" {
		domainerror.WriteProblem(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Expected application/json",
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var body requestBody

	if err := decoder.Decode(&body); err != nil {
		domainerror.WriteProblem(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid JSON",
		)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		domainerror.WriteProblem(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Unexpected extra JSON",
		)
		return
	}

	if body.Coordinates == nil ||
		body.Coordinates.Latitude == nil ||
		body.Coordinates.Longitude == nil ||
		body.RecordedAt == nil {
		domainerror.WriteProblem(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Missing required fields",
		)
		return
	}

	latitude := *body.Coordinates.Latitude
	longitude := *body.Coordinates.Longitude

	if math.IsNaN(latitude) ||
		math.IsInf(latitude, 0) ||
		latitude < -90 ||
		latitude > 90 ||
		math.IsNaN(longitude) ||
		math.IsInf(longitude, 0) ||
		longitude < -180 ||
		longitude > 180 ||
		body.RecordedAt.IsZero() {
		domainerror.WriteProblem(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid position data",
		)
		return
	}

	input := createuc.Input{
		TripID:     tripID.String(),
		Latitude:   latitude,
		Longitude:  longitude,
		RecordedAt: *body.RecordedAt,
	}

	created, err := h.usecase.Execute(r.Context(), input)
	if err != nil {
		domainerror.Write(w, err)
		return
	}

	triphttp.WriteJSON(
		w,
		http.StatusCreated,
		triphttp.ToAPITripPosition(created),
	)
}

package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	domaintrip "github.com/EkLeRchEk/template/internal/domain/trip"
	api "github.com/EkLeRchEk/template/internal/generated"
)

// WriteProblem отправляет ошибку в формате Problem Details.
func WriteProblem(
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

// Write преобразует доменную ошибку в HTTP-ответ.
func Write(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domaintrip.ErrTripNotFound):
		WriteProblem(
			w,
			http.StatusNotFound,
			"trip_not_found",
			"Trip not found",
		)

	case errors.Is(err, domaintrip.ErrTripCompleted):
		WriteProblem(
			w,
			http.StatusConflict,
			"trip_completed",
			"Trip already completed",
		)

	case errors.Is(err, domaintrip.ErrDriverBusy):
		WriteProblem(
			w,
			http.StatusConflict,
			"driver_busy",
			"Driver already has an active trip",
		)

	default:
		WriteProblem(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"Internal server error",
		)
	}
}

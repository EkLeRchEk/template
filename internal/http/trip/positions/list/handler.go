package list

import (
	"context"
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/http/domainerror"
	triphttp "github.com/EkLeRchEk/template/internal/http/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

type usecase interface {
	Execute(
		ctx context.Context,
		tripID string,
	) ([]model.TripPosition, error)
}

type Handler struct {
	usecase usecase
}

func New(uc usecase) *Handler {
	return &Handler{usecase: uc}
}

// Handle возвращает историю координат поездки.
func (h *Handler) Handle(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	positions, err := h.usecase.Execute(
		r.Context(),
		tripID.String(),
	)
	if err != nil {
		domainerror.Write(w, err)
		return
	}

	result := make([]api.TripPosition, 0, len(positions))

	for _, position := range positions {
		result = append(
			result,
			triphttp.ToAPITripPosition(position),
		)
	}

	triphttp.WriteJSON(
		w,
		http.StatusOK,
		api.TripPositionsResponse{
			Positions: result,
		},
	)
}

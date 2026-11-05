package finish

import (
	"context"
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/http/domainerror"
	triphttp "github.com/EkLeRchEk/template/internal/http/trip"
	"github.com/EkLeRchEk/template/internal/model"
)

// usecase описывает бизнес-операцию завершения поездки.
type usecase interface {
	Execute(ctx context.Context, id string) (model.Trip, error)
}

type Handler struct {
	usecase usecase
}

func New(uc usecase) *Handler {
	return &Handler{
		usecase: uc,
	}
}

// Handle завершает поездку по идентификатору.
func (h *Handler) Handle(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	finished, err := h.usecase.Execute(
		r.Context(),
		tripID.String(),
	)
	if err != nil {
		domainerror.Write(w, err)
		return
	}

	triphttp.WriteJSON(
		w,
		http.StatusOK,
		triphttp.ToAPITrip(finished),
	)
}

package triphttp

import (
	"encoding/json"
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/model"
	"github.com/google/uuid"
)

// ToAPITrip преобразует внутреннюю модель в OpenAPI-модель.
func ToAPITrip(t model.Trip) api.Trip {
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

// WriteJSON отправляет JSON-ответ.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

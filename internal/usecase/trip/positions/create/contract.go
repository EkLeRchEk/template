package create

import (
	"context"
	"time"

	"github.com/EkLeRchEk/template/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

type tripRepository interface {
	SavePosition(
		ctx context.Context,
		position model.TripPosition,
	) (model.TripPosition, error)
}

type positionCanceller interface {
	Cancel(tripID string)
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type Input struct {
	TripID     string
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

package list

import (
	"context"

	"github.com/EkLeRchEk/template/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=list

type tripRepository interface {
	ListPositions(
		ctx context.Context,
		tripID string,
	) ([]model.TripPosition, error)
}

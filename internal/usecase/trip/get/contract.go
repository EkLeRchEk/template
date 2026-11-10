package get

import (
	"context"

	"github.com/EkLeRchEk/template/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=get

type tripRepository interface {
	GetByID(ctx context.Context, id string) (model.Trip, error)
}

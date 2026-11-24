package finish

import (
	"context"
	"time"

	"github.com/EkLeRchEk/template/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish

type tripRepository interface {
	GetByID(ctx context.Context, id string) (model.Trip, error)
	Finish(ctx context.Context, id string, finishedAt time.Time) error
	CreateStatusHistory(ctx context.Context, history model.StatusHistory) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

package create

import (
	"context"

	"github.com/EkLeRchEk/template/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

// Интерфейсы зависимостей usecase.
type tripRepository interface {
	Create(ctx context.Context, trip model.Trip) error
	CreateStatusHistory(ctx context.Context, history model.StatusHistory) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

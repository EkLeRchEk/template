package container

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/EkLeRchEk/template/internal/gateway/push"
	httpadapter "github.com/EkLeRchEk/template/internal/http"
	createhandler "github.com/EkLeRchEk/template/internal/http/trip/create"
	finishhandler "github.com/EkLeRchEk/template/internal/http/trip/finish"
	gethandler "github.com/EkLeRchEk/template/internal/http/trip/get"
	createpositionhandler "github.com/EkLeRchEk/template/internal/http/trip/positions/create"
	listpositionhandler "github.com/EkLeRchEk/template/internal/http/trip/positions/list"
	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
	triprepo "github.com/EkLeRchEk/template/internal/repository/trip"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/create"
	finishuc "github.com/EkLeRchEk/template/internal/usecase/trip/finish"
	getuc "github.com/EkLeRchEk/template/internal/usecase/trip/get"
	createpositionuc "github.com/EkLeRchEk/template/internal/usecase/trip/positions/create"
	listpositionuc "github.com/EkLeRchEk/template/internal/usecase/trip/positions/list"
	positionworker "github.com/EkLeRchEk/template/internal/worker/positions"
)

// Container собирает зависимости приложения.
type Container struct {
	HTTP      *httpadapter.Server
	positions *positionworker.Worker
}

// New создаёт и связывает все компоненты приложения.
func New(
	pool *pgxpool.Pool,
	queryTimeout time.Duration,
	workerConfig positionworker.Config,
	pushURL string,
) (*Container, error) {
	// Инфраструктура.
	repository := triprepo.New(pool, queryTimeout)
	txManager := transaction.New(pool)

	// Push Gateway.
	pushClient, err := push.New(
		pushURL,
		workerConfig.PushRequestTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize push gateway: %w", err)
	}

	// Worker Pool.
	positionsWorker := positionworker.New(
		repository,
		pushClient,
		workerConfig,
		slog.Default(),
	)

	// Бизнес-логика.
	createUsecase := createuc.New(repository, txManager)
	getUsecase := getuc.New(repository)
	finishUsecase := finishuc.New(repository, txManager)

	createPositionUsecase := createpositionuc.New(
		repository,
		txManager,
		positionsWorker,
	)
	listPositionsUsecase := listpositionuc.New(repository)

	// HTTP-обработчики.
	createHandler := createhandler.New(createUsecase)
	getHandler := gethandler.New(getUsecase)
	finishHandler := finishhandler.New(finishUsecase)

	createPositionHandler := createpositionhandler.New(createPositionUsecase)
	listPositionHandler := listpositionhandler.New(listPositionsUsecase)

	// HTTP-сервер.
	httpServer := httpadapter.New(
		pool,
		slog.Default(),
		createHandler,
		getHandler,
		finishHandler,
		createPositionHandler,
		listPositionHandler,
	)

	return &Container{
		HTTP:      httpServer,
		positions: positionsWorker,
	}, nil
}

// Start запускает фоновые процессы приложения.
func (c *Container) Start(ctx context.Context) {
	c.positions.Start(ctx)
}

// Stop останавливает фоновые процессы приложения.
func (c *Container) Stop(ctx context.Context) error {
	return c.positions.Stop(ctx)
}

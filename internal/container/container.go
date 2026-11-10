package container

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/EkLeRchEk/template/internal/http"
	createhandler "github.com/EkLeRchEk/template/internal/http/trip/create"
	finishhandler "github.com/EkLeRchEk/template/internal/http/trip/finish"
	gethandler "github.com/EkLeRchEk/template/internal/http/trip/get"
	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
	triprepo "github.com/EkLeRchEk/template/internal/repository/trip"
	createuc "github.com/EkLeRchEk/template/internal/usecase/trip/create"
	finishuc "github.com/EkLeRchEk/template/internal/usecase/trip/finish"
	getuc "github.com/EkLeRchEk/template/internal/usecase/trip/get"
)

// Container собирает зависимости приложения.
type Container struct {
	HTTP *httpadapter.Server
}

// New создаёт и связывает все компоненты приложения.
func New(
	pool *pgxpool.Pool,
	queryTimeout time.Duration,
) *Container {
	// Инфраструктура.
	repository := triprepo.New(pool, queryTimeout)
	txManager := transaction.New(pool)

	// Бизнес-логика.
	createUsecase := createuc.New(repository, txManager)
	getUsecase := getuc.New(repository)
	finishUsecase := finishuc.New(repository, txManager)

	// HTTP-обработчики.
	createHandler := createhandler.New(createUsecase)
	getHandler := gethandler.New(getUsecase)
	finishHandler := finishhandler.New(finishUsecase)

	// HTTP-сервер.
	server := httpadapter.New(
		pool,
		createHandler,
		getHandler,
		finishHandler,
	)

	return &Container{
		HTTP: server,
	}
}

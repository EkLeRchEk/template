package trip

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/EkLeRchEk/template/internal/infra/postgres/transaction"
)

// DBTX описывает исполнителя SQL-запросов.
type DBTX interface {
	Exec(
		ctx context.Context,
		sql string,
		arguments ...any,
	) (pgconn.CommandTag, error)

	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

type Repository struct {
	db           DBTX
	queryTimeout time.Duration
}

func New(db DBTX, queryTimeout time.Duration) *Repository {
	return &Repository{
		db:           db,
		queryTimeout: queryTimeout,
	}
}

// executor выбирает транзакцию или обычное подключение.
func (r *Repository) executor(ctx context.Context) DBTX {
	if tx := transaction.FromContext(ctx); tx != nil {
		return tx
	}

	return r.db
}

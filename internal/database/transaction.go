package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{
		pool: pool,
	}
}

// txKey — специальный ключ для хранения транзакции в context.
type txKey struct{}

// TxFromContext извлекает транзакцию из context.
// Если транзакции нет, возвращает nil.
func TxFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txKey{}).(pgx.Tx)
	return tx
}

// Do выполняет функцию внутри транзакции.
func (m *TxManager) Do(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	// Если транзакция уже существует, переиспользуем её.
	if TxFromContext(ctx) != nil {
		return fn(ctx)
	}

	// Начинаем новую транзакцию.
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Если COMMIT не состоялся, откатываем транзакцию.
	// defer выполнится также при panic.
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	// Сохраняем транзакцию внутри context.
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Выполняем бизнес-операцию.
	if err := fn(txCtx); err != nil {
		return err
	}

	// Если ошибок не было, сохраняем изменения.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

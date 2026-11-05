package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type contextKey struct{}

type Manager struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// FromContext возвращает транзакцию, если она есть.
func FromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(contextKey{}).(pgx.Tx)
	return tx
}

// Do выполняет функцию внутри транзакции.
func (m *Manager) Do(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	ctx, span := otel.Tracer("trip-service/transaction").Start(
		ctx,
		"Transaction",
	)
	defer span.End()

	// Записываем ошибку транзакции в трассировку.
	recordError := func(err error) error {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "transaction failed")
		}
		return err
	}

	// Переиспользуем существующую транзакцию.
	if FromContext(ctx) != nil {
		return recordError(fn(ctx))
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return recordError(fmt.Errorf("begin transaction: %w", err))
	}

	// При ошибке или panic выполняется откат.
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	txCtx := context.WithValue(ctx, contextKey{}, tx)

	if err := fn(txCtx); err != nil {
		return recordError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return recordError(fmt.Errorf("commit transaction: %w", err))
	}

	return nil
}

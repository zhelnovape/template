package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type pgxTxManager struct {
	pool     *pgxpool.Pool
	isoLevel pgx.TxIsoLevel
	txTimeout time.Duration
}

func NewTxManager(pool *pgxpool.Pool, isoLevel pgx.TxIsoLevel, txTimeout time.Duration) TxManager {
	return &pgxTxManager{pool: pool, isoLevel: isoLevel, txTimeout: txTimeout}
}

func (m *pgxTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	beginCtx, beginCancel := context.WithTimeout(ctx, m.txTimeout)
	defer beginCancel()

	tx, err := m.pool.BeginTx(beginCtx, pgx.TxOptions{IsoLevel: m.isoLevel})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), m.txTimeout)
	defer rollbackCancel()

	commitCtx, commitCancel := context.WithTimeout(ctx, m.txTimeout)
	defer commitCancel()

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(rollbackCtx)
		}
	}()

	if err := fn(withTx(ctx, tx)); err != nil {
		return err
	}

	if err := tx.Commit(commitCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

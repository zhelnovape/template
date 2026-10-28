package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Manager struct {
	pool      *pgxpool.Pool
	txTimeout time.Duration
}

func New(pool *pgxpool.Pool, txTimeout time.Duration) *Manager {
	return &Manager{pool: pool, txTimeout: txTimeout}
}

func (m *Manager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	beginCtx, beginCancel := context.WithTimeout(ctx, m.txTimeout)
	defer beginCancel()

	tx, err := m.pool.BeginTx(beginCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.txTimeout)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	if err := fn(withTx(ctx, tx)); err != nil {
		return err
	}

	commitCtx, commitCancel := context.WithTimeout(ctx, m.txTimeout)
	defer commitCancel()

	if err := tx.Commit(commitCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

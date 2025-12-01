package db

import (
	"context"
	"fmt"

	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func WithTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	logger logger.Logger,
	fn func(ctx context.Context, q Querier) error,
) (retErr error) {
	if pool == nil {
		return fmt.Errorf("db.WithTx: pool is nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		logger.Error("tx_begin_failed", "err", err)
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				logger.Error("tx_rollback_failed", "err", rbErr)
			}
			panic(p)
		}

		if retErr != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
				logger.Error("tx_rollback_failed", "err", rbErr)
			}
			return
		}

		if cmErr := tx.Commit(ctx); cmErr != nil {
			logger.Error("tx_commit_failed", "err", cmErr)
			retErr = fmt.Errorf("commit tx: %w", cmErr)
		}
	}()

	retErr = fn(ctx, tx)
	return retErr
}

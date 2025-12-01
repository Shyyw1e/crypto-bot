package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Open(ctx context.Context, cfg *config.Config, log logger.Logger) (*pgxpool.Pool, error) {
	dsn := cfg.Postgres.DSN
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("empty dsn")
	}

	clearedDSN := clearSecret(dsn)

	pgcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Error("db_parse_config_failed", "dsn", clearedDSN, "err", err)
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	if cfg.Postgres.MaxOpenConns > 0 {
		pgcfg.MaxConns = int32(cfg.Postgres.MaxOpenConns)
	}
	if cfg.Postgres.MaxIdleConns > 0 {
		pgcfg.MinConns = int32(cfg.Postgres.MaxIdleConns)
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgcfg)
	if err != nil {
		log.Error("db_open_failed", "dsn", clearedDSN, "err", err)
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		log.Error("db_ping_failed", "dsn", clearedDSN, "err", err)
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("db_connected", "dsn", clearedDSN)
	return pool, nil
}

func Close(pool *pgxpool.Pool, log logger.Logger) {
	if pool == nil {
		return
	}
	pool.Close()
	log.Info("db_closed")
}

func clearSecret(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return s
	}
	passPart := strings.Split(parts[2], "@")
	if len(passPart) < 2 {
		return s
	}
	password := passPart[0]
	if password == "" {
		return s
	}
	return strings.Replace(s, password, "****", 1)
}

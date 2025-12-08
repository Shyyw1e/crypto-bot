package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	rds "github.com/redis/go-redis/v9"
)

type Client = rds.Client

func New(ctx context.Context, cfg *config.Config, log logger.Logger) (*rds.Client, error) {
	addr := cfg.Redis.Addr
	password := cfg.Redis.Password
	dbNumber := cfg.Redis.DB

	if addr == "" {
		log.Error("redis_empty_addr")
		return nil, fmt.Errorf("redis: empty addr")
	}

	rdb := rds.NewClient(&rds.Options{
		Addr:     addr,
		Password: password,
		DB:       dbNumber,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Error("redis_ping_failed", "addr", addr, "err", err)
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info("redis_connected", "addr", addr, "db", dbNumber)
	return rdb, nil
}

func Close(rdb *rds.Client, log logger.Logger) {
	if rdb == nil {
		return
	}
	_ = rdb.Close()
	log.Info("redis_closed")
}

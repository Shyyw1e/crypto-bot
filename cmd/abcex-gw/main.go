package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/adapters/abcexws"
	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/adapters/publisher"
	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := config.MustLoad("abcex-gw")
	log := logger.New(cfg.App.LogLevel, "abcex-gw")
	log.Info("abcex_gw_starting", "env", cfg.App.Env)

	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("abcex_gw_redis_init_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	if cfg.Nats.URL == "" {
		log.Error("abcex_gw_nats_url_empty")
		os.Exit(1)
	}

	nc, err := nats.Connect(cfg.Nats.URL, nats.Name("abcex-gw"))
	if err != nil {
		log.Error("abcex_gw_nats_connect_failed", "url", cfg.Nats.URL, "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	client := abcexws.NewClient(cfg.ABCEX, log)
	pub := publisher.NewRedisOrderbookPublisher(rdb, log, 2*time.Second, nc)

	symbol := cfg.ABCEX.Symbols[0]
	svc := usecase.NewService(log, client, pub, cfg.ABCEX.ReconnectDelay, symbol)

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("abcex_gw_service_stopped_with_error", "err", err)
		os.Exit(1)
	}

	log.Info("abcex_gw_stopped")
}

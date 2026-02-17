package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/adapters/grinexapi"
	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/adapters/publisher"
	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := config.MustLoad("grinex-gw")
	log := logger.New(cfg.App.LogLevel, "grinex-gw")
	log.Info("grinex_gw_starting", "env", cfg.App.Env)

	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("grinex_gw_redis_init_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	if cfg.Nats.URL == "" {
		log.Error("grinex_gw_nats_url_empty")
		os.Exit(1)
	}

	nc, err := nats.Connect(cfg.Nats.URL, nats.Name("grinex-gw"))
	if err != nil {
		log.Error("grinex_gw_nats_connect_failed", "url", cfg.Nats.URL, "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := grinexapi.NewClient(cfg.Grinex, httpClient, log)
	pub := publisher.NewRedisOrderbookPublisher(rdb, log, 2*time.Second, nc)

	// На текущем этапе поддерживаем один символ.
	symbol := cfg.Grinex.Symbols[0]
	svc := usecase.NewService(log, client, pub, cfg.Grinex.PollInterval, symbol)

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("grinex_gw_service_stopped_with_error", "err", err)
		os.Exit(1)
	}

	log.Info("grinex_gw_stopped")
}

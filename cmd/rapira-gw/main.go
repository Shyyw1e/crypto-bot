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
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/adapters/middleware"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/adapters/publisher"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/adapters/rapiraapi"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := config.MustLoad("rapira-gw")
	log := logger.New(cfg.App.LogLevel, "rapira-gw")

	log.Info("rapira_gw_starting", "env", cfg.App.Env)

	// Redis
	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("rapira_gw_redis_init_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	// NATS
	if cfg.Nats.URL == "" {
		log.Error("rapira_gw_nats_url_empty")
		os.Exit(1)
	}

	nc, err := nats.Connect(cfg.Nats.URL, nats.Name("rapira-gw"))
	if err != nil {
		log.Error("rapira_gw_nats_connect_failed", "url", cfg.Nats.URL, "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	log.Info("rapira_gw_nats_connected", "url", cfg.Nats.URL)


	// Базовый HTTP-клиент
	baseHTTP := &http.Client{
		Timeout: 5 * time.Second,
	}

	// TokenManager для авторизации в Rapira
	tm, err := middleware.NewTokenManager(
    log,
    cfg.Rapira.PrivateKeyBase64,
    "rapira-api",        // issuer
    "rapira-api",        // audience
    cfg.Rapira.APIKeyKID,
    10*time.Minute,      // ttl
    baseHTTP,
)

	if err != nil {
		log.Error("rapira_gw_token_manager_init_failed", "err", err)
		os.Exit(1)
	}

	// HTTP-клиент с обёрткой, которая всегда подставляет актуальный JWT
	httpClient := middleware.NewRapiraHTTPClient(tm, log)

	// Rapira API client
	rapiraClient := rapiraapi.NewClient(cfg.Rapira, httpClient, log)

	// Publisher → Redis (TTL пока захардкожен, можно вынести в конфиг)
	pub := publisher.NewRedisOrderbookPublisher(rdb, log, 2*time.Second, nc)

	// Usecase сервиса
	svc := usecase.NewService(
		log,
		rapiraClient,
		pub,
		cfg.Rapira.PollInterval, // интервал опроса Rapira из env
	)

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("rapira_gw_service_stopped_with_error", "err", err)
		os.Exit(1)
	}

	log.Info("rapira_gw_stopped")
}

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	cfg := config.MustLoad("")
	log := logger.New(cfg.App.LogLevel, "rapira-gw")

	log.Info("rapira_gw_starting", "env", cfg.App.Env)

	// Redis
	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("rapira_gw_redis_init_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	// Базовый HTTP-клиент (без авторизации)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// TokenManager (использует cfg.Rapira.* и httpClient для /open/generate_jwt)
	tm, err := middleware.NewTokenManager(cfg.Rapira, log, httpClient)
	if err != nil {
		log.Error("rapira_gw_token_manager_init_failed", "err", err)
		os.Exit(1)
	}

	// Оборачиваем транспорт httpClient в AuthTransport, который подставляет Bearer-токен
    var baseTransport *http.Transport

    switch tr := httpClient.Transport.(type) {
    case nil:
        baseTransport = http.DefaultTransport.(*http.Transport).Clone()
    case *http.Transport:
        baseTransport = tr
    default:
        baseTransport = http.DefaultTransport.(*http.Transport).Clone()
    }

    httpClient.Transport = middleware.NewAuthTransport(baseTransport, tm, log)


	// Rapira HTTP client (использует httpClient с AuthTransport)
	rapiraClient := rapiraapi.NewClient(cfg.Rapira, httpClient, log)

	// Publisher → Redis, TTL можно взять из конфигов, пока 2 секунды
	pub := publisher.NewRedisOrderbookPublisher(rdb, log, 2*time.Second)

	// Usecase service
	svc := usecase.NewService(
		log,
		rapiraClient,
		pub,
		cfg.Rapira.PollInterval, // интервал опроса берём из конфига
	)

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("rapira_gw_service_stopped_with_error", "err", err)
		os.Exit(1)
	}

	log.Info("rapira_gw_stopped_gracefully")
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tgbotgrpc "github.com/Shyyw1e/crypto-bot/internal/analyser/adapters/tgbotgrpc"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	pgrepo "github.com/Shyyw1e/crypto-bot/internal/analyser/repository/postgres"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/db"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

var ErrEmptyNATSURL = fmt.Errorf("nats url is empty")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.MustLoad("analyser")
	log := logger.New(cfg.App.LogLevel, "analyser")
	log.Info("analyser_starting", "env", cfg.App.Env)

	// Postgres
	pool, err := db.Open(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_db_open_failed", "err", err)
		os.Exit(1)
	}
	defer db.Close(pool, log)

	// Redis
	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_redis_open_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	// Репозитории
	userRepo := pgrepo.NewUserSettingsPostgres(pool, log)
	notifRepo := pgrepo.NewNotificationPostgres(pool, log)

	// Кэши
	obCache := cache.NewOrderbookCache(rdb, log)
	dedup := cache.NewNotificationDedup(rdb, log)

	// gRPC-клиент к tg-боту (уведомления)
	if cfg.Telegram.Addr == "" {
		log.Error("analyser_tgbot_grpc_addr_empty")
		os.Exit(1)
	}

	conn, err := grpc.DialContext(
		ctx,
		cfg.Telegram.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Error("analyser_tgbot_grpc_connect_failed", "addr", cfg.Telegram.Addr, "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	notifier := tgbotgrpc.NewGRPCNotifier(conn, log, 2*time.Second)

	// Сервис анализатора
	svc := usecase.NewService(log, obCache, userRepo, notifRepo, dedup, notifier)

	// NATS
	nc, err := initNATS(cfg, log)
	if err != nil {
		log.Error("analyser_nats_init_failed", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	subject := "orderbook.updated.rapira.usdt_rub"

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		// payload не нужен — analyser сам забирает актуальный стакан из Redis
		_ = msg

		ctxTick, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		go func() {
			defer cancel()
			if err := svc.HandleTick(ctxTick); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("analyser_handletick_failed", "err", err)
			}
		}()
	})
	if err != nil {
		log.Error("analyser_nats_subscribe_failed", "subject", subject, "err", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	log.Info("analyser_ready", "subject", subject, "nats_url", cfg.Nats.URL)

	<-ctx.Done()
	log.Info("analyser_stopping")
}

func initNATS(cfg *config.Config, log logger.Logger) (*nats.Conn, error) {
	url := cfg.Nats.URL
	if url == "" {
		log.Error("analyser_nats_url_empty")
		return nil, ErrEmptyNATSURL
	}

	nc, err := nats.Connect(url, nats.Name("analyser"))
	if err != nil {
		log.Error("analyser_nats_connect_failed", "url", url, "err", err)
		return nil, err
	}

	log.Info("analyser_nats_connected", "url", url)
	return nc, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tgbotgrpc "github.com/Shyyw1e/crypto-bot/internal/analyser/adapters/tgbotgrpc"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	analysergrpc "github.com/Shyyw1e/crypto-bot/internal/analyser/grpc"
	pgrepo "github.com/Shyyw1e/crypto-bot/internal/analyser/repository/postgres"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/usecase"
	analyserpb "github.com/Shyyw1e/crypto-bot/internal/proto/analyserpb"
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
	// Postgres (retry до 30 секунд, чтобы дождаться старта контейнера postgres)
	var pool *pgxpool.Pool
	deadline := time.Now().Add(30 * time.Second)

	for {
		p, err := db.Open(ctx, cfg, log)
		if err == nil {
			pool = p
			break
		}
		if time.Now().After(deadline) {
			log.Error("analyser_db_open_failed", "err", err)
			os.Exit(1)
		}
		time.Sleep(1 * time.Second)
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
	if cfg.Telegram.AnalyserAddr == "" {
        log.Error("analyser_grpc_addr_empty", "hint", "set ANALYSER_GRPC_ADDR env (например, :50052)")
        os.Exit(1)
    }

    lis, err := net.Listen("tcp", cfg.Telegram.AnalyserAddr)
    if err != nil {
        log.Error("analyser_grpc_listen_failed", "addr", cfg.Telegram.AnalyserAddr, "err", err)
        os.Exit(1)
    }

    grpcSrv := grpc.NewServer()

    settingsServer := analysergrpc.NewSettingsServer(log, svc)
    analyserpb.RegisterAnalyserSettingsServiceServer(grpcSrv, settingsServer)

    go func() {
        log.Info("analyser_grpc_server_start", "addr", cfg.Telegram.AnalyserAddr)
        if err := grpcSrv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
            log.Error("analyser_grpc_server_failed", "err", err)
            stop() // гасим приложение
        }
    }()

	// NATS
	nc, err := initNATS(cfg, log)
	if err != nil {
		log.Error("analyser_nats_init_failed", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	subject := "orderbook.updated.*.*"

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		start := time.Now()

		log.Debug("analyser_nats_tick_received", "subject", msg.Subject)

		ctxTick, cancel := context.WithTimeout(ctx, 2*time.Second) // на время дебага увеличь
		defer cancel()

		err := svc.HandleTick(ctxTick)

		log.Debug("analyser_tick_processed",
			"dur_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	})


	if err != nil {
		log.Error("analyser_nats_subscribe_failed", "subject", subject, "err", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	log.Info("analyser_ready", "subject", subject, "nats_url", cfg.Nats.URL)

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ctxTick, cancel := context.WithTimeout(ctx, 1*time.Second)
				_ = svc.HandleTick(ctxTick)
				cancel()
			}
		}
	}()


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

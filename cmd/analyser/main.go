package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	pgrepo "github.com/Shyyw1e/crypto-bot/internal/analyser/repository/postgres"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/db"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

// MVPNotifier — временный notifier, который просто логирует уведомления.
type MVPNotifier struct {
	log logger.Logger
}

func NewMVPNotifier(log logger.Logger) *MVPNotifier {
	return &MVPNotifier{log: log}
}

func (m *MVPNotifier) Send(ctx context.Context, n *domain.Notification) error {
	_ = ctx // пока не используем, но оставляем для будущего
	m.log.Info(
		"send_notification_mvp",
		"chat_id", n.ChatID,
		"type", n.Type,
		"pair", n.Pair,
		"direction", n.Direction,
		"profit_diff", n.ProfitDiff,
		"notional", n.Notional,
		"op_hash", n.OpHash,
	)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.MustLoad("")
	log := logger.New(cfg.App.LogLevel, "analyser")
	log.Info("analyser_starting", "env", cfg.App.Env)

	pool, err := db.Open(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_db_open_failed", "err", err)
		os.Exit(1)
	}
	defer db.Close(pool, log)

	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_redis_open_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	userRepo := pgrepo.NewUserSettingsPostgres(pool, log)
	notifRepo := pgrepo.NewNotificationPostgres(pool, log)

	obCache := cache.NewOrderbookCache(rdb, log)
	dedup := cache.NewNotificationDedup(rdb, log)

	notifier := NewMVPNotifier(log)

	svc := usecase.NewService(log, obCache, userRepo, notifRepo, dedup, notifier)

	nc, err := initNATS(cfg, log)
	if err != nil {
		log.Error("analyser_nats_init_failed", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	subject := "orderbook.updated.rapira.usdt_rub"

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		// Здесь payload не нужен — мы просто триггерим HandleTick.
		_ = msg

		ctxTick, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
			if err := svc.HandleTick(ctxTick); err != nil {
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

var ErrEmptyNATSURL = fmt.Errorf("nats url is empty")

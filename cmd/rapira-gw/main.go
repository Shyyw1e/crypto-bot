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

	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	pgrepo "github.com/Shyyw1e/crypto-bot/internal/analyser/repository/postgres"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/usecase"
	grpcnotifier "github.com/Shyyw1e/crypto-bot/internal/analyser/adapters/tgbotgrpc"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/db"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

// MVPNotifier — запасной notifier, который просто логирует уведомления
// (используем, если нет gRPC-адреса бота).
type MVPNotifier struct {
	log logger.Logger
}

func NewMVPNotifier(log logger.Logger) *MVPNotifier {
	return &MVPNotifier{log: log}
}

func (m *MVPNotifier) Send(_ context.Context, n *domain.Notification) error {
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

	cfg := config.MustLoad("analyser")
	log := logger.New(cfg.App.LogLevel, "analyser")

	log.Info("analyser_starting", "env", cfg.App.Env)

	// --- Postgres ---
	pool, err := db.Open(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_db_open_failed", "err", err)
		os.Exit(1)
	}
	defer db.Close(pool, log)

	// --- Redis ---
	rdb, err := redis.New(ctx, cfg, log)
	if err != nil {
		log.Error("analyser_redis_open_failed", "err", err)
		os.Exit(1)
	}
	defer redis.Close(rdb, log)

	// --- Repositories & cache ---
	userRepo := pgrepo.NewUserSettingsPostgres(pool, log)
	notifRepo := pgrepo.NewNotificationPostgres(pool, log)
	obCache := cache.NewOrderbookCache(rdb, log)
	dedup := cache.NewNotificationDedup(rdb, log)

	// --- Notifier (gRPC к tg-bot, либо MVP-логгер) ---
	var notifier usecase.Notifier

	if cfg.Telegram.Addr == "" {
		log.Warn("analyser_tgbot_addr_empty_fallback_mvp_notifier")
		notifier = NewMVPNotifier(log)
	} else {
		tgConn, err := grpc.NewClient(
			cfg.Telegram.Addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Error("analyser_tgbot_grpc_connect_failed", "addr", cfg.Telegram.Addr, "err", err)
			os.Exit(1)
		}
		defer tgConn.Close()

		// timeout можно вынести в конфиг, пока захардкодим 1–2 секунды
		notifier = grpcnotifier.NewGRPCNotifier(tgConn, log, 2*time.Second)

		log.Info("analyser_tgbot_grpc_connected", "addr", cfg.Telegram.Addr)
	}

	// --- Usecase service ---
	svc := usecase.NewService(
		log,
		obCache,
		userRepo,
		notifRepo,
		dedup,
		notifier,
	)

	// --- NATS (подписка на обновления стакана) ---
	nc, err := initNATS(cfg, log)
	if err != nil {
		log.Error("analyser_nats_init_failed", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	subject := "orderbook.updated.rapira.usdt_rub"

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		_ = msg // payload нам не нужен — просто триггерим HandleTick

		ctxTick, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		go func() {
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

var ErrEmptyNATSURL = fmt.Errorf("nats url is empty")

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

package main

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Shyyw1e/crypto-bot/internal/proto/analyserpb"
	"github.com/Shyyw1e/crypto-bot/internal/proto/tgbotpb"
	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"

	grpcserver "github.com/Shyyw1e/crypto-bot/internal/tg-bot/grpc"
	"github.com/Shyyw1e/crypto-bot/internal/tg-bot/usecase"
)

func main() {
	// Грейсфул-шатдаун по Ctrl+C / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Конфиг + логгер
	cfg := config.MustLoad("tg-bot")
	log := logger.New(cfg.App.LogLevel, "tg-bot")

	log.Info("tgbot_starting", "env", cfg.App.Env)

	// --- Telegram Bot API ---
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		log.Error("tgbot_init_failed", "err", err)
		os.Exit(1)
	}

	if cfg.App.Env == "local" || cfg.App.Env == "dev" {
		bot.Debug = true
	}

	log.Info("tgbot_authorized", "bot_username", bot.Self.UserName)

	// --- gRPC клиент к analyser (настройки пользователя) ---
	analyserAddr := os.Getenv("ANALYSER_GRPC_ADDR")
	if analyserAddr == "" {
		log.Error("tgbot_missing_env", "env", "ANALYSER_GRPC_ADDR")
		os.Exit(1)
	}

	analyserConn, err := grpcLib.DialContext(
		ctx,
		analyserAddr,
		grpcLib.WithTransportCredentials(insecure.NewCredentials()),
		// без WithBlock, без WithTimeout — коннект асинхронный
	)
	if err != nil {
		log.Error("tgbot_analyser_grpc_connect_failed", "addr", analyserAddr, "err", err)
		os.Exit(1)
	}

	log.Info("tgbot_analyser_grpc_connected", "addr", analyserAddr)
	analyserClient := analyserpb.NewAnalyserSettingsServiceClient(analyserConn)
	defer analyserConn.Close()

	// --- Usecase-слой (диалоги и команды) ---
	svc := usecase.NewService(log, bot, analyserClient)

	// --- Telegram updates (long polling) ---
	updateCfg := tgbotapi.NewUpdate(0)
	updateCfg.Timeout = 30

	updates, err := bot.GetUpdatesChan(updateCfg)
	if err != nil {
		log.Error("tgbot_get_updates_failed", "err", err)
		os.Exit(1)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info("tgbot_updates_stopped", "reason", ctx.Err())
				return
			case upd, ok := <-updates:
				if !ok {
					log.Info("tgbot_updates_channel_closed")
					return
				}

				log.Debug("tgbot_update_received",
					"has_message", upd.Message != nil,
					"chat_id", func() int64 {
						if upd.Message != nil {
							return upd.Message.Chat.ID
						}
						return 0
					}(),
				)

				if err := svc.HandleUpdate(ctx, upd); err != nil {
					log.Error("tgbot_handle_update_failed", "err", err)
				}
			}
		}
	}()

	if cfg.Telegram.Addr == "" {
		log.Error("tgbot_grpc_addr_empty", "hint", "set TGBOT_GRPC_ADDR env")
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", cfg.Telegram.Addr)
	if err != nil {
		log.Error("tgbot_grpc_listen_failed", "addr", cfg.Telegram.Addr, "err", err)
		os.Exit(1)
	}

	grpcSrv := grpcLib.NewServer()
	notifServer := grpcserver.New(log, bot)
	tgbotpb.RegisterTgBotNotificationServiceServer(grpcSrv, notifServer)

	go func() {
		log.Info("tgbot_grpc_server_start", "addr", cfg.Telegram.Addr)
		if err := grpcSrv.Serve(lis); err != nil && !errors.Is(err, grpcLib.ErrServerStopped) {
			log.Error("tgbot_grpc_server_failed", "err", err)
			// если упал gRPC-сервер — гасим всё приложение
			stop()
		}
	}()

	log.Info("tgbot_ready", "grpc_addr", cfg.Telegram.Addr, "analyser_addr", analyserAddr)

	// Ожидаем завершения по сигналу
	<-ctx.Done()
	log.Info("tgbot_stopping", "reason", ctx.Err())

	grpcSrv.GracefulStop()
}

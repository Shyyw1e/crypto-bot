package grpcserver

import (
	"context"
	"fmt"

	"github.com/Shyyw1e/crypto-bot/internal/proto/tgbotpb"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Server реализует gRPC-сервис TgBotNotificationService.
type Server struct {
	tgbotpb.UnimplementedTgBotNotificationServiceServer

	log logger.Logger
	bot *tgbotapi.BotAPI
}

// New создаёт новый gRPC-сервер-обработчик для нотификаций.
func New(log logger.Logger, bot *tgbotapi.BotAPI) *Server {
	return &Server{
		log: log,
		bot: bot,
	}
}

// SendNotification вызывается analyser'ом по gRPC.
func (s *Server) SendNotification(ctx context.Context, req *tgbotpb.SendNotificationRequest) (*tgbotpb.SendNotificationResponse, error) {
	n := req.GetNotification()
	if n == nil {
		s.log.Warn("tgbot_grpc_empty_notification")
		return &tgbotpb.SendNotificationResponse{}, nil
	}

	chatID := n.GetChatId()
	if chatID == 0 {
		s.log.Warn("tgbot_grpc_no_chat_id", "notification", n)
		return &tgbotpb.SendNotificationResponse{}, nil
	}

	text := s.formatNotificationText(n)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	if _, err := s.bot.Send(msg); err != nil {
		s.log.Error(
			"tgbot_grpc_send_notification_failed",
			"chat_id", chatID,
			"err", err,
		)
		// для analyser это не критический фейл — вернём OK, чтобы он не ретраил бесконечно
		return &tgbotpb.SendNotificationResponse{}, nil
	}

	s.log.Info(
		"tgbot_grpc_notification_sent",
		"chat_id", chatID,
		"type", n.GetType(),
		"pair", n.GetPair(),
		"direction", n.GetDirection(),
		"profit_diff", n.GetProfitDiff(),
		"notional", n.GetNotional(),
		"op_hash", n.GetOpHash(),
	)

	return &tgbotpb.SendNotificationResponse{}, nil
}

// formatNotificationText — формирует текст сообщения для Telegram.
func (s *Server) formatNotificationText(n *tgbotpb.Notification) string {
	var kind string
	switch n.GetType() {
	case "fact":
		kind = "ФАКТ"
	case "potential":
		kind = "ПОТЕНЦИАЛ"
	default:
		kind = n.GetType()
	}

	// simple markdown, без лишних спецсимволов
	return fmt.Sprintf(
		"*%s* по паре *%s*\n"+
			"Направление: `%s`\n"+
			"Разница: *%.2f*\n"+
			"Нотионал: *%.2f* USDT\n"+
			"`op_hash: %s`",
		kind,
		n.GetPair(),
		n.GetDirection(),
		n.GetProfitDiff(),
		n.GetNotional(),
		n.GetOpHash(),
	)
}

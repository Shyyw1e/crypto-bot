package grpcserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"


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

type opHashDetails struct {
	pair         string
	buyExchange  string
	sellExchange string
	buyPrice     float64
	sellPrice    float64
}

func parseOpHash(hash string) (*opHashDetails, bool) {
	parts := strings.Split(hash, "-")
	// Минимум:
	// chatID, pair, buyEx, sellEx, buyPrice, sellPrice, buyAmount
	if len(parts) < 7 {
		return nil, false
	}

	last := len(parts) - 1

	// парсим числовой хвост с конца
	// buyAmount пока не используем, но валидируем формат
	if _, err := strconv.ParseFloat(parts[last], 64); err != nil {
		return nil, false
	}

	sellPrice, err := strconv.ParseFloat(parts[last-1], 64)
	if err != nil {
		return nil, false
	}

	buyPrice, err := strconv.ParseFloat(parts[last-2], 64)
	if err != nil {
		return nil, false
	}

	sellExchange := parts[last-3]
	buyExchange := parts[last-4]

	pairParts := parts[1 : last-4]
	if len(pairParts) == 0 {
		return nil, false
	}
	pair := strings.Join(pairParts, "-")

	return &opHashDetails{
		pair:         pair,
		buyExchange:  buyExchange,
		sellExchange: sellExchange,
		buyPrice:     buyPrice,
		sellPrice:    sellPrice,
	}, true
}


func prettyExchange(raw string) string {
	switch raw {
	case "rapira":
		return "Rapira"
	case "grinex_usdt_a7a5":
		return "Grinex"
	default:
		if raw == "" {
			return "Unknown"
		}
		return strings.ToUpper(raw[:1]) + raw[1:]
	}
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
		kind = strings.ToUpper(n.GetType())
	}

	details, ok := parseOpHash(n.GetOpHash())
	if !ok {
		return fmt.Sprintf(
			"%s по паре %s\n"+
				"Направление: %s\n"+
				"Разница с учетом комиссии: %.2f\n"+
				"Тотал: %.2f USDT",
			kind,
			n.GetPair(),
			n.GetDirection(),
			n.GetProfitDiff(),
			n.GetNotional(),
		)
	}

	pair := n.GetPair()
	if pair == "" {
		pair = details.pair
	}

	buyPair := pair
	sellPair := pair
	if left, right, found := strings.Cut(pair, "->"); found {
		buyPair = left
		sellPair = right
	}

	return fmt.Sprintf(
		"%s по паре %s\n"+
			"Ордер покупки: %s %s %.2f\n"+
			"Ордер продажи: %s %s %.2f\n"+
			"Разница с учетом комиссии: %.2f\n"+
			"Тотал: %.2f USDT",
		kind,
		pair,
		prettyExchange(details.buyExchange),
		buyPair,
		details.buyPrice,
		prettyExchange(details.sellExchange),
		sellPair,
		details.sellPrice,
		n.GetProfitDiff(),
		n.GetNotional(),
	)
}

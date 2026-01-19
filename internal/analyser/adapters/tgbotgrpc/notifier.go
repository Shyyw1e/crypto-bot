package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/proto/tgbotpb"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"google.golang.org/grpc"
)

// GRPCNotifier реализует usecase.Notifier и шлёт нотификации в tg-bot по gRPC.
type GRPCNotifier struct {
	log     logger.Logger
	client  tgbotpb.TgBotNotificationServiceClient
	timeout time.Duration
}

func NewGRPCNotifier(cc grpc.ClientConnInterface, log logger.Logger, timeout time.Duration) *GRPCNotifier {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &GRPCNotifier{
		log:     log,
		client:  tgbotpb.NewTgBotNotificationServiceClient(cc),
		timeout: timeout,
	}
}

// Send — реализация интерфейса usecase.Notifier.
func (n *GRPCNotifier) Send(ctx context.Context, notif *domain.Notification) error {
	if notif == nil {
		return fmt.Errorf("grpc_notifier: nil notification")
	}

	createdAt := notif.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	pbNotif := &tgbotpb.Notification{
		ChatId:        notif.ChatID,
		Type:          string(notif.Type),
		Pair:          string(notif.Pair),
		Direction:     notif.Direction,
		ProfitDiff:    notif.ProfitDiff,
		Notional:      notif.Notional,
		OpHash:        notif.OpHash,
		CreatedAtUnix: createdAt.Unix(),
	}

	req := &tgbotpb.SendNotificationRequest{
		Notification: pbNotif,
	}

	sendCtx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	_, err := n.client.SendNotification(sendCtx, req)
	if err != nil {
		n.log.Error("grpc_notifier_send_failed",
			"chat_id", notif.ChatID,
			"type", notif.Type,
			"pair", notif.Pair,
			"err", err,
		)
		return fmt.Errorf("send notification via grpc: %w", err)
	}

	n.log.Debug("grpc_notifier_send_ok",
		"chat_id", notif.ChatID,
		"type", notif.Type,
		"pair", notif.Pair,
	)

	return nil
}

package repository

import (
	"context"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
)

type UserSettingsRepository interface {
	GetByChatID(ctx context.Context, chatID int64) (*domain.UserSettings, error)
	Save(ctx context.Context, s *domain.UserSettings) error
	SetActive(ctx context.Context, chatID int64, active bool) error
	ListActive(ctx context.Context) ([]*domain.UserSettings, error)
}

type NotificationRepository interface {
	Create(ctx, n *domain.Notification) error
	ListByChatID(ctx, chatID int64, limit int)
	//LastForChat(ctx, chatID int64)
}
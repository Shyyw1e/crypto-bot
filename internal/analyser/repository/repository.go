package repository

import (
	"context"
	"errors"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
)

// ErrNotFound — общая ошибка для кейса "запись не найдена".
var ErrNotFound = errors.New("not found")

type UserSettingsRepository interface {
	GetByChatID(ctx context.Context, chatID int64) (*domain.UserSettings, error)
	Save(ctx context.Context, s *domain.UserSettings) error
	SetActive(ctx context.Context, chatID int64, active bool) error
	ListActive(ctx context.Context) ([]*domain.UserSettings, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	ListByChatID(ctx context.Context, chatID int64, limit int) ([]*domain.Notification, error)

	// LastForChat(ctx context.Context, chatID int64) (*domain.Notification, error)
}

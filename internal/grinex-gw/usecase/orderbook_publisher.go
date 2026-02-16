package usecase

import (
	"context"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
)

type OrderbookPublisher interface {
	Publish(ctx context.Context, src domain.Source, pair domain.Pair, ob *Orderbook) error
}
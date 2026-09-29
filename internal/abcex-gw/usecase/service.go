package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/adapters/abcexws"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type ABCEXClient interface {
	StreamOrderbook(ctx context.Context, symbol string, handle func(context.Context, *abcexws.OrderbookMessage) error) error
}

type Service struct {
	log       logger.Logger
	client    ABCEXClient
	publisher OrderbookPublisher

	reconnectDelay time.Duration
	symbol         string
	source         domain.Source
	pair           domain.Pair
}

func NewService(
	log logger.Logger,
	client ABCEXClient,
	publisher OrderbookPublisher,
	reconnectDelay time.Duration,
	symbol string,
) *Service {
	if reconnectDelay <= 0 {
		reconnectDelay = 3 * time.Second
	}
	if symbol == "" {
		symbol = "USDTRUB"
	}

	return &Service{
		log:            log,
		client:         client,
		publisher:      publisher,
		reconnectDelay: reconnectDelay,
		symbol:         symbol,
		source:         domain.SourceABCEX,
		pair:           domain.USDTRUB,
	}
}

func (s *Service) Run(ctx context.Context) error {
	s.log.Info("abcex_gw_service_started",
		"symbol", s.symbol,
		"reconnect_delay", s.reconnectDelay.String(),
	)

	for {
		if err := s.client.StreamOrderbook(ctx, s.symbol, s.handleOrderbook); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				s.log.Info("abcex_gw_service_stopped")
				return ctx.Err()
			}
			s.log.Warn("abcex_gw_stream_stopped", "err", err)
		}

		select {
		case <-ctx.Done():
			s.log.Info("abcex_gw_service_stopped")
			return ctx.Err()
		case <-time.After(s.reconnectDelay):
		}
	}
}

func (s *Service) handleOrderbook(ctx context.Context, msg *abcexws.OrderbookMessage) error {
	ob := Mapper(msg, s.log)
	if ob == nil {
		return nil
	}
	return s.publisher.Publish(ctx, s.source, s.pair, ob)
}

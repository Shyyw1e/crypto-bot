package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/adapters/rapiraapi"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type RapiraClient interface {
	GetMarketDepth(ctx context.Context, symbol string) (*rapiraapi.PlateResponse, error)
}

type Service struct {
	log       logger.Logger
	client    RapiraClient
	publisher OrderbookPublisher

	pollInterval time.Duration
	symbol string
	source domain.Source
	pair   domain.Pair
}

func NewService(
	log logger.Logger,
	client RapiraClient,
	publisher OrderbookPublisher,
	pollInterval time.Duration,
) *Service {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}

	return &Service{
		log:          log,
		client:       client,
		publisher:    publisher,
		pollInterval: pollInterval,
		symbol:       "USDT/RUB",
		source:       domain.SourceRapira,
		pair:         domain.USDTRUB,
	}
}

// Run — главный тикер-луп. Блокирует до ctx.Done().
func (s *Service) Run(ctx context.Context) error {
	s.log.Info("rapira_gw_service_started",
		"symbol", s.symbol,
		"interval", s.pollInterval.String(),
	)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	if err := s.tick(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		s.log.Warn("rapira_gw_first_tick_failed", "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.log.Info("rapira_gw_service_stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				s.log.Warn("rapira_gw_tick_failed", "err", err)
			}
		}
	}
}

func (s *Service) tick(ctx context.Context) error {
	ctxReq, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.client.GetMarketDepth(ctxReq, s.symbol)
	if err != nil {
		if errors.Is(err, rapiraapi.ErrEmptyBook) {
			s.log.Warn("rapira_gw_empty_book", "symbol", s.symbol)
			return nil
		}
		return err
	}

	ob := Mapper(resp, s.log)
	if ob == nil {
		return nil
	}

	if err := s.publisher.Publish(ctx, s.source, s.pair, ob); err != nil {
		return err
	}

	return nil
}
